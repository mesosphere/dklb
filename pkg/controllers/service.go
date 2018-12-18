package controllers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/edgelb/manager"
	"github.com/mesosphere/dklb/pkg/translator"
	kubernetesutil "github.com/mesosphere/dklb/pkg/util/kubernetes"
	"github.com/mesosphere/dklb/pkg/util/prettyprint"
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
	// kubeClient is a client to the Kubernetes core APIs.
	kubeClient kubernetes.Interface
	// serviceLister knows how to list Service resources from a shared informer's store.
	serviceLister corev1listers.ServiceLister
	// edgelbManager is the instance of the EdgeLB manager to use for materializing EdgeLB pools for Service resources.
	edgelbManager manager.EdgeLBManager
}

// NewServiceController creates a new instance of the EdgeLB service controller.
func NewServiceController(kubeClient kubernetes.Interface, serviceInformer corev1informers.ServiceInformer, edgelbManager manager.EdgeLBManager) *ServiceController {
	// Create a new instance of the service controller with the specified name and threadiness.
	c := &ServiceController{
		genericController: newGenericController(serviceControllerName, serviceControllerThreadiness),
		kubeClient:        kubeClient,
		serviceLister:     serviceInformer.Lister(),
		edgelbManager:     edgelbManager,
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
	// Convert the specified key into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key %q", key))
		return nil
	}

	// Get the Service resource with the specified namespace and name.
	service, err := c.serviceLister.Services(namespace).Get(name)
	if err != nil {
		// The Service resource may no longer exist, in which case we must stop processing.
		// TODO (@bcustodio) This might (or might not) be a good place to perform cleanup of any associated EdgeLB pools.
		if apierrors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("service %q in work queue no longer exists", key))
			return nil
		}
		return err
	}

	// Create an event recorder that we can use to report events related with the Ingress resource.
	er := kubernetesutil.NewEventRecorderForNamespace(c.kubeClient, service.Namespace)

	// Compute the set of options that will be used to translate the Service resource into an EdgeLB pool.
	options, err := translator.ComputeServiceTranslationOptions(service)
	if err != nil {
		// Emit an event and log an error, but do not re-enqueue as the resource is likely invalid.
		// TODO (@bcustodio) Understand if this is indeed the case, and whether we should re-enqueue the current key.
		er.Eventf(service, corev1.EventTypeWarning, constants.ReasonInvalidAnnotations, "the resource's annotations are not valid: %v", err)
		c.logger.Errorf("failed to compute translation options for service %q: %v", key, err)
		return nil
	}

	// Output some debugging information about the computed set of options.
	c.logger.Debugf("options for service %q: %s", key, prettyprint.Sprint(options))

	// Perform translation of the Service resource into an EdgeLB pool.
	if err := translator.NewServiceTranslator(service, *options, c.edgelbManager).Translate(); err != nil {
		c.logger.Errorf("failed to translate service %q: %v", key, err)
		return err
	}

	// Update the status of the Service resource.
	if _, err := c.kubeClient.CoreV1().Services(service.Namespace).UpdateStatus(service); err != nil {
		c.logger.Errorf("failed to update status for service %q: %v", key, err)
		return err
	}
	return nil
}

// Run starts the controller, blocking until the specified context is canceled.
func (c *ServiceController) Run(ctx context.Context) error {
	// Handle any possible crashes and shutdown the work queue when we're done.
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	c.logger.Debugf("starting %q", serviceControllerName)

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
