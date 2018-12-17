package controllers

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// Controller represents a controller that handles Kubernetes resources.
type Controller interface {
	// Run instructs the workers to start processing items from the queue.
	Run(ctx context.Context) error
}

// genericController contains basic functionality shared by all controllers.
type genericController struct {
	// logger is the logger that the controller will use.
	logger log.FieldLogger
	// workqueue is a rate limited work queue.
	// It is used to queue work to be processed instead of performing it as soon as a change happens.
	// This means we can ensure we only process a fixed amount of resources at a time, and makes it easy to ensure we are never processing the same resource simultaneously in two different worker goroutines.
	workqueue workqueue.RateLimitingInterface
	// hasSyncedFuncs are the functions used to determine if caches are synced.
	hasSyncedFuncs []cache.InformerSynced
	// syncHandler is a function that takes a key (namespace/name) and processes the corresponding resource.
	syncHandler func(key string) error
	// threadiness is the number of workers to use for processing items from the work queue.
	threadiness int
}

// newGenericController returns a new generic controller.
func newGenericController(name string, threadiness int) *genericController {
	// Return a new instance of a generic controller.
	return &genericController{
		logger:      log.WithField("controller", name),
		workqueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
		threadiness: threadiness,
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
			key string
			ok  bool
		)

		// We expect strings to come off the work queue.
		// These are of the form "namespace/name".
		// We do this as the delayed nature of the work queue means the items in the informer cache may actually be more up to date that when the item was initially put onto the work queue.
		if key, ok = obj.(string); !ok {
			// As the item in the work queue is actually invalid, we call Forget here else we'd go into a loop of attempting to process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected a string to come off of the work queue but got %#v", obj))
			return nil
		}
		// Call syncHandler, passing it the "namespace/name" string that corresponds to the resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing %q: %s", key, err.Error())
		}
		// Finally, and if no error occurs, we forget this item so it does not get queued again until another change happens.
		c.workqueue.Forget(obj)
		c.logger.Debugf("successfully synced %q", key)
		return nil
	}(obj)

	// If we've got an error, pass it to HandleError so a back-off behavior can be applied.
	if err != nil {
		runtime.HandleError(err)
	}
	return true
}

// enqueue takes a Kubernetes resource and converts it into a "namespace/name" string which is then put onto the work queue.
func (c *genericController) enqueue(obj interface{}) {
	if key, err := cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
	} else {
		c.workqueue.AddRateLimited(key)
	}
}
