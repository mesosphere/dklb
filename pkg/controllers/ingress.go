package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	extsv1beta1informers "k8s.io/client-go/informers/extensions/v1beta1"
	extsv1beta1listers "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/translator"
	"github.com/mesosphere/dklb/pkg/util/prettyprint"
)

const (
	// ingressControllerName is the name of the ingress controller.
	ingressControllerName = "ingress-controller"
	// ingressControllerThreadiness is the number of workers the ingress controller will use to process items from its work queue.
	ingressControllerThreadiness = 2
)

// IngressController is the controller for Ingress resources.
type IngressController struct {
	// IngressController is based-off of a generic controller.
	*genericController
	// ingressLister knows how to list Ingress resources from a shared informer's store.
	ingressLister extsv1beta1listers.IngressLister
}

// NewIngressController creates a new instance of the EdgeLB ingress controller.
func NewIngressController(ingressInformer extsv1beta1informers.IngressInformer) *IngressController {
	// Create a new instance of the ingress controller with the specified name and threadiness.
	c := &IngressController{
		genericController: newGenericController(ingressControllerName, ingressControllerThreadiness),
		ingressLister:     ingressInformer.Lister(),
	}
	// Make the controller wait for caches to sync.
	c.hasSyncedFuncs = []cache.InformerSynced{
		ingressInformer.Informer().HasSynced,
	}
	// Make processQueueItem the handler for items popped out of the work queue.
	c.syncHandler = c.processQueueItem

	// Setup an event handler to inform us when Ingress resources change.
	ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueIfEdgeLBIngress(obj.(*extsv1beta1.Ingress))
		},
		UpdateFunc: func(_, obj interface{}) {
			c.enqueueIfEdgeLBIngress(obj.(*extsv1beta1.Ingress))
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueIfEdgeLBIngress(obj.(*extsv1beta1.Ingress))
		},
	})

	// Return the instance created above.
	return c
}

// enqueueIfEdgeLBIngress checks if the specified Ingress resource is annotated to be provisioned by EdgeLB, and enqueues it if this condition is verified.
func (c *IngressController) enqueueIfEdgeLBIngress(obj *extsv1beta1.Ingress) {
	// If the object has no annotations, return.
	if obj.Annotations == nil {
		return
	}
	// If the required annotation is not present, return.
	v, exists := obj.Annotations[constants.EdgeLBIngressClassAnnotationKey]
	if !exists {
		return
	}
	// If the annotation is present but doesn't have the required value, return.
	if v != constants.EdgeLBIngressClassAnnotationValue {
		return
	}
	// Enqueue the Ingress resource for later processing.
	c.enqueue(obj)
}

// processQueueItem attempts to reconcile the state of the Ingress resource pointed at by the specified key.
func (c *IngressController) processQueueItem(key string) error {
	// Convert the specified key into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key %q", key))
		return nil
	}

	// Get the Ingress resource with the specified namespace and name.
	resource, err := c.ingressLister.Ingresses(namespace).Get(name)
	if err != nil {
		// The Ingress resource may no longer exist, in which case we must stop processing.
		// TODO (@bcustodio) This might (or might not) be a good place to perform cleanup of any associated EdgeLB pools.
		if apierrors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("ingress %q in work queue no longer exists", key))
			return nil
		}
		return err
	}

	// Compute the set of options that will be used to translate the Ingress resource into an EdgeLB pool.
	options, err := translator.ComputeIngressTranslationOptions(resource)
	if err != nil {
		// Log an error, but do not re-enqueue as the resource is likely invalid.
		// TODO (@bcustodio) Understand if this is indeed the case, and whether we should re-enqueue the current key.
		c.logger.Errorf("failed to compute translation options for ingress %q: %v", key, err)
		return nil
	}

	// Output some debugging information about the computed set of options.
	c.logger.Debugf("options for ingress %q: %s", key, prettyprint.Sprint(options))

	// TODO (@bcustodio) Implement translation.
	return errors.New("translation not implemented")
}

// Run starts the controller, blocking until the specified context is canceled.
func (c *IngressController) Run(ctx context.Context) error {
	// Handle any possible crashes and shutdown the work queue when we're done.
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	c.logger.Debugf("starting %q", ingressControllerName)

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
