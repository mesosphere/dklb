package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	// serviceControllerName is the name of the service controller.
	serviceControllerName = "service-controller"
	// serviceControllerThreadiness is the number of workers the service controller will use to process items from its work queue.
	serviceControllerThreadiness = 2
)

// ServiceController is the controller for Service resources.
type ServiceController struct {
	// ServiceController is based-off of a generic controller.
	*genericController
}

// NewServiceController creates a new instance of the EdgeLB service controller.
func NewServiceController(serviceInformer corev1informers.ServiceInformer) *ServiceController {
	// Create a new instance of the service controller with the specified name and threadiness.
	c := &ServiceController{
		genericController: newGenericController(serviceControllerName, serviceControllerThreadiness),
	}
	// Make the controller wait for caches to sync.
	c.hasSyncedFuncs = []cache.InformerSynced{
		serviceInformer.Informer().HasSynced,
	}
	// Make processQueueItem the handler for items popped out of the work queue.
	c.syncHandler = c.processQueueItem

	// Setup an event handler to inform us when Service resources change.
	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueIfLoadBalancer(obj.(*corev1.Service))
		},
		UpdateFunc: func(_, obj interface{}) {
			c.enqueueIfLoadBalancer(obj.(*corev1.Service))
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueIfLoadBalancer(obj.(*corev1.Service))
		},
	})

	// Return the instance created above.
	return c
}

// enqueueIfLoadBalancer checks if the specified Service resource is of type "LoadBalancer", and enqueues it if this condition is verified.
func (c *ServiceController) enqueueIfLoadBalancer(service *corev1.Service) {
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return
	}
	c.enqueue(service)
}

// processQueueItem attempts to reconcile the state of the Service resource pointed at by the specified key.
func (c *ServiceController) processQueueItem(key string) error {
	// TODO (@bcustodio) Implement
	return errors.New("not implemented")
}

// Run starts the controller, blocking until the specified context is canceled.
func (c *ServiceController) Run(ctx context.Context) error {
	// Handle any possible crashes and shutdown the work queue when we're done.
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	c.logger.Debug("starting %q", serviceControllerName)

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
