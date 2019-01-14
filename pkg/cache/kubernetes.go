package cache

import (
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	extsv1beta1informers "k8s.io/client-go/informers/extensions/v1beta1"
)

// kubernetesResourceCache knows how to list Kubernetes resources.
type KubernetesResourceCache interface {
	// HasSynced returns a value indicating whether the cache is synced.
	HasSynced() bool
	// GetIngress returns the Ingress resource with the specified namespace and name.
	GetIngress(string, string) (*extsv1beta1.Ingress, error)
	// GetIngresses returns a list of all Ingress resources in the specified namespace.
	GetIngresses(string) ([]*extsv1beta1.Ingress, error)
	// GetService returns the Service resource with the specified namespace and name.
	GetService(namespace, name string) (*corev1.Service, error)
}

// kubernetesResourceCache knows how to list Kubernetes resources.
// The main implementation basically groups together and wraps listers for the Kubernetes resources we are interested in.
type kubernetesResourceCache struct {
	// ingressInformer is an informer for Ingress resources.
	ingressInformer extsv1beta1informers.IngressInformer
	// serviceInformer is an informer for Service resources.
	serviceInformer corev1informers.ServiceInformer
}

// NewKubernetesResourceCache returns a new instance of the Kubernetes resource cache.
func NewKubernetesResourceCache(factory kubeinformers.SharedInformerFactory) KubernetesResourceCache {
	return &kubernetesResourceCache{
		ingressInformer: factory.Extensions().V1beta1().Ingresses(),
		serviceInformer: factory.Core().V1().Services(),
	}
}

// HasSynced returns a value indicating whether the cache is synced.
func (c *kubernetesResourceCache) HasSynced() bool {
	return c.ingressInformer.Informer().HasSynced() && c.serviceInformer.Informer().HasSynced()
}

// GetIngress returns the Ingress resource with the specified namespace and name.
func (c *kubernetesResourceCache) GetIngress(namespace, name string) (*extsv1beta1.Ingress, error) {
	return c.ingressInformer.Lister().Ingresses(namespace).Get(name)
}

// GetIngresses returns a list of all Ingress resources in the specified namespace.
func (c *kubernetesResourceCache) GetIngresses(namespace string) ([]*extsv1beta1.Ingress, error) {
	return c.ingressInformer.Lister().Ingresses(namespace).List(labels.Everything())
}

// GetService returns the Service resource with the specified namespace and name.
func (c *kubernetesResourceCache) GetService(namespace, name string) (*corev1.Service, error) {
	return c.serviceInformer.Lister().Services(namespace).Get(name)
}
