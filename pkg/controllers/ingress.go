package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	extsv1beta1informers "k8s.io/client-go/informers/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"

	"github.com/mesosphere/dklb/pkg/constants"
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
}

// NewIngressController creates a new instance of the EdgeLB ingress controller.
func NewIngressController(ingressInformer extsv1beta1informers.IngressInformer) *IngressController {
	// Create a new instance of the ingress controller with the specified name and threadiness.
	c := &IngressController{
		genericController: newGenericController(ingressControllerName, ingressControllerThreadiness),
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
	// TODO (@bcustodio) Implement
	return errors.New("not implemented")
}

// Run starts the controller, blocking until the specified context is canceled.
func (c *IngressController) Run(ctx context.Context) error {
	// Handle any possible crashes and shutdown the work queue when we're done.
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	c.logger.Debug("starting %q", ingressControllerName)

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
