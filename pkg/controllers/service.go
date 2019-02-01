package controllers

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	dklbcache "github.com/mesosphere/dklb/pkg/cache"
	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/edgelb/manager"
	"github.com/mesosphere/dklb/pkg/metrics"
	"github.com/mesosphere/dklb/pkg/translator"
	kubernetesutil "github.com/mesosphere/dklb/pkg/util/kubernetes"
	"github.com/mesosphere/dklb/pkg/util/prettyprint"
)

const (
	// serviceControllerName is the name of the service controller.
	serviceControllerName = "service-controller"
	// serviceControllerThreadiness is the number of workers the service controller will use to process items from its work queue.
	serviceControllerThreadiness = 1
)

// ServiceController is the controller for Service resources.
type ServiceController struct {
	// ServiceController is based-off of a generic controller.
	*genericController
	// kubeClient is a client to the Kubernetes core APIs.
	kubeClient kubernetes.Interface
	// dklbCache is the instance of the Kubernetes resource cache to use.
	kubeCache dklbcache.KubernetesResourceCache
	// edgelbManager is the instance of the EdgeLB manager to use for materializing EdgeLB pools for Service resources.
	edgelbManager manager.EdgeLBManager
}

// NewServiceController creates a new instance of the EdgeLB service controller.
func NewServiceController(clusterName string, kubeClient kubernetes.Interface, serviceInformer corev1informers.ServiceInformer, configMapInformer corev1informers.ConfigMapInformer, kubeCache dklbcache.KubernetesResourceCache, edgelbManager manager.EdgeLBManager) *ServiceController {
	// Create a new instance of the service controller with the specified name and threadiness.
	c := &ServiceController{
		genericController: newGenericController(clusterName, serviceControllerName, serviceControllerThreadiness),
		kubeClient:        kubeClient,
		kubeCache:         kubeCache,
		edgelbManager:     edgelbManager,
	}
	// Make the controller wait for the caches to sync.
	c.hasSyncedFuncs = []cache.InformerSynced{
		configMapInformer.Informer().HasSynced,
		serviceInformer.Informer().HasSynced,
		kubeCache.HasSynced,
	}
	// Make processQueueItem the handler for items popped out of the work queue.
	c.syncHandler = c.processQueueItem

	// Setup an event handler to inform us when Service resources change.
	// A Service resource is enqueued in the following scenarios:
	// * It was listed ("ADDED") and is of type "LoadBalancer".
	// * It was updated ("MODIFIED") and either the old or the new types (or both) are of type "LoadBalancer".
	//   * This allows for handling the cases in which the type of a service changes.
	// * It was deleted ("DELETED") and was of type "LoadBalancer".
	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc := obj.(*corev1.Service)
			if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
				return
			}
			c.enqueue(svc)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldSvc := oldObj.(*corev1.Service)
			newSvc := newObj.(*corev1.Service)
			if oldSvc.Spec.Type != corev1.ServiceTypeLoadBalancer && newSvc.Spec.Type != corev1.ServiceTypeLoadBalancer {
				return
			}
			c.enqueue(newSvc)
		},
		DeleteFunc: func(obj interface{}) {
			svc := obj.(*corev1.Service)
			if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
				return
			}
			c.enqueueTombstone(svc)
		},
	})
	// Setup an event handler to inform us when ConfigMap resources change.
	// This allows us to enqueue all Service resources that reference a given ConfigMap resource whenever it changes.
	configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueReferencingServices(obj.(*corev1.ConfigMap))
		},
		UpdateFunc: func(_, obj interface{}) {
			c.enqueueReferencingServices(obj.(*corev1.ConfigMap))
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueReferencingServices(obj.(*corev1.ConfigMap))
		},
	})

	// Return the instance created above.
	return c
}

// processQueueItem attempts to reconcile the state of the Service resource pointed at by the specified key.
func (c *ServiceController) processQueueItem(workItem WorkItem) error {
	// Record the current iteration.
	startTime := time.Now()
	metrics.RecordSync(serviceControllerName, workItem.Key)
	defer metrics.RecordSyncDuration(serviceControllerName, startTime)

	// Convert the specified key into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(workItem.Key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key %q", workItem.Key))
		return nil
	}

	// Get the Service resource with the specified namespace and name.
	service, err := c.kubeCache.GetService(namespace, name)
	if err == nil {
		// Create a deep copy of the Service resource in order to avoid possibly mutating the cache.
		service = service.DeepCopy()
	} else {
		// Return immediately if the current error's type is something other than "NotFound".
		if !apierrors.IsNotFound(err) {
			return err
		}
		// At this point we know the service resource does not exist anymore.
		// Hence, we take its tombstone and hand it over to the translator so it can perform cleanup of the associated EdgeLB pool as it sees fit.
		if workItem.Tombstone == nil {
			return fmt.Errorf("service %q in work queue no longer exists, and no tombstone was recovered", workItem.Key)
		}
		// Create a deep copy of the tombstone in order to avoid mutating the cache.
		service = workItem.Tombstone.(*corev1.Service).DeepCopy()
		// Set the current timestamp as the value of ".metadata.deletionTimestamp" so the translator can understand that the resource has been deleted.
		deletionTimestamp := metav1.NewTime(startTime)
		service.ObjectMeta.DeletionTimestamp = &deletionTimestamp
	}

	// Create an event recorder that we can use to report events related with the Ingress resource.
	er := kubernetesutil.NewEventRecorderForNamespace(c.kubeClient, service.Namespace)

	// Compute the set of options that will be used to translate the Service resource into an EdgeLB pool.
	options, err := translator.ComputeServiceTranslationOptions(c.clusterName, service)
	if err != nil {
		// Emit an event and log an error, but do not re-enqueue as the resource's spec was found to be invalid.
		er.Eventf(service, corev1.EventTypeWarning, constants.ReasonInvalidAnnotations, "the resource's annotations are not valid: %v", err)
		c.logger.Errorf("failed to compute translation options for service %q: %v", workItem.Key, err)
		return nil
	}

	// Output some tracing information about the computed set of options.
	prettyprint.LogfSpew(log.Tracef, options, "computed service translation options for %q", workItem.Key)

	// Perform translation of the Service resource into an EdgeLB pool.
	status, err := translator.NewServiceTranslator(c.clusterName, service, *options, c.kubeCache, c.edgelbManager).Translate()
	if err != nil {
		er.Eventf(service, corev1.EventTypeWarning, constants.ReasonTranslationError, "failed to translate service: %v", err)
		c.logger.Errorf("failed to translate service %q: %v", workItem.Key, err)
		return err
	}

	// Update the status of the Service resource if it hasn't been deleted.
	if service.ObjectMeta.DeletionTimestamp == nil && status != nil {
		service.Status = corev1.ServiceStatus{LoadBalancer: *status}
		if _, err := c.kubeClient.CoreV1().Services(service.Namespace).UpdateStatus(service); err != nil {
			c.logger.Errorf("failed to update status for service %q: %v", workItem.Key, err)
			return err
		}
	}
	return nil
}

// enqueueReferencingServices enqueues Service resources that reference the provided ConfigMap resource.
func (c *ServiceController) enqueueReferencingServices(configMap *corev1.ConfigMap) {
	// Grab a list of all Service resources in the same namespace as the ConfigMap resource.
	services, err := c.kubeCache.GetServices(configMap.Namespace)
	if err != nil {
		c.logger.Errorf("failed to list all services in namespace %q: %v", configMap.Name, err)
		return
	}
	// Iterate over all listed Service resources, checking whether each one references this ConfigMap resource and enqueueing it if it does.
	for _, service := range services {
		obj := service
		if obj.GetAnnotations() != nil {
			if obj.GetAnnotations()[constants.CloudLoadBalancerConfigMapNameAnnotationKey] == configMap.Name {
				c.enqueue(obj)
			}
		}
	}
}
