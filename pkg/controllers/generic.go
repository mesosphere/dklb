package controllers

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// Controller represents a controller that handles Kubernetes resources.
type Controller interface {
	// Run instructs the workers to start processing items from the queue.
	Run(ctx context.Context) error
}

// WorkItem represents an item that is placed onto the controller's work queue.
// It is a pairing between the "namespace/name" key corresponding to a given Kubernetes resource and the resource's tombstone in case the resource has been deleted.
type WorkItem struct {
	// Key is the key of the Kubernetes resource being synced.
	Key string
	// Tombstone is the tombstone (i.e. last known state) of the Kubernetes resource being synced.
	Tombstone interface{}
}

// genericController contains basic functionality shared by all controllers.
type genericController struct {
	// clusterName is the name of the Mesos framework that corresponds to the current Kubernetes cluster.
	clusterName string
	// logger is the logger that the controller will use.
	logger log.FieldLogger
	// workqueue is a rate limited work queue.
	// It is used to queue work to be processed instead of performing it as soon as a change happens.
	// This means we can ensure we only process a fixed amount of resources at a time, and makes it easy to ensure we are never processing the same resource simultaneously in two different worker goroutines.
	workqueue workqueue.RateLimitingInterface
	// hasSyncedFuncs are the functions used to determine if caches are synced.
	hasSyncedFuncs []cache.InformerSynced
	// syncHandler is a function that takes a work item and processes it.
	syncHandler func(wi WorkItem) error
	// threadiness is the number of workers to use for processing items from the work queue.
	threadiness int
	// name is the name of the controller.
	name string
}

// Run starts the controller, blocking until the specified context is canceled.
func (c *genericController) Run(ctx context.Context) error {
	// Handle any possible crashes and shutdown the work queue when we're done.
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	c.logger.Debugf("starting %q", c.name)

	// Wait for the caches to be synced before starting workers.
	c.logger.Debug("waiting for informer caches to be synced")
	if ok := cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...); !ok {
		return fmt.Errorf("failed to wait for informer caches to be synced")
	}

	c.logger.Debug("starting workers")

	// Launch "threadiness" workers to process items from the work queue.
	for i := 0; i < c.threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, ctx.Done())
	}

	c.logger.Info("started workers")

	// Block until the context is canceled.
	<-ctx.Done()
	return nil
}

// newGenericController returns a new generic controller.
func newGenericController(name string, threadiness int) *genericController {
	// Return a new instance of a generic controller.
	return &genericController{
		logger:      log.WithField("controller", name),
		workqueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
		threadiness: threadiness,
		name:        name,
	}
}

// runWorker is a long-running function that will continually call the processNextWorkItem function in order to read and process an item from the work queue.
func (c *genericController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the work queue and attempt to process it by calling syncHandler.
func (c *genericController) processNextWorkItem() bool {
	// Read an item from the work queue.
	obj, shutdown := c.workqueue.Get()
	// Return immediately if we've been told to shut down.
	if shutdown {
		return false
	}

	// Wrap this block in a func so we can defer the call to c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the work queue knows we have finished processing this item.
		// We also must remember to call Forget if we do not want this work item to be re-queued.
		// For example, we do not call Forget if a transient error occurs.
		// Instead the item is put back on the work queue and attempted again after a back-off period.
		defer c.workqueue.Done(obj)

		var (
			workItem WorkItem
			ok       bool
		)

		// We expect objects of type "WorkItem" to come off the work queue.
		// These group together keys of the form "namespace/name" corresponding to Kubernetes resources and their tombstone in case these resources have been deleted.
		if workItem, ok = obj.(WorkItem); !ok {
			// As the item in the work queue is actually invalid, we call Forget here else we'd go into a loop of attempting to process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected a string to come off of the work queue but got %#v", obj))
			return nil
		}
		// Call syncHandler, passing it the "namespace/name" string that corresponds to the resource to be synced.
		if err := c.syncHandler(workItem); err != nil {
			return fmt.Errorf("error syncing %q: %s", workItem.Key, err.Error())
		}
		// Finally, and if no error occurs, we forget this item so it does not get queued again until another change happens.
		c.workqueue.Forget(obj)
		c.logger.Debugf("successfully synced %q", workItem.Key)
		return nil
	}(obj)

	// If we've got an error, pass it to HandleError so a back-off behavior can be applied.
	if err != nil {
		runtime.HandleError(err)
	}
	return true
}

// enqueue takes a Kubernetes resource, computes its resource key and puts it as a work item onto the work queue.
func (c *genericController) enqueue(obj interface{}) {
	if key, err := cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
	} else {
		c.workqueue.AddRateLimited(WorkItem{
			Key: key,
		})
	}
}

// enqueueTombstone takes the tombstone of a Kubernetes resource that has been deleted, computes its resource key and puts it as a work item onto the work queue.
// Must only be used to handle cleanup in scenarios where the Kubernetes resource has been deleted.
// For all other usage scenarios, "enqueue" should be used instead.
func (c *genericController) enqueueTombstone(obj interface{}) {
	if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
	} else {
		c.workqueue.AddRateLimited(WorkItem{
			Key:       key,
			Tombstone: obj,
		})
	}
}
