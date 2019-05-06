package cache

import (
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	extsv1beta1informers "k8s.io/client-go/informers/extensions/v1beta1"
)

// informerBackedResourceCache is an implementation of KubernetesResourceCache backed by informers and their associated listers.
type informerBackedResourceCache struct {
	// ingressInformer is an informer for Ingress resources.
	ingressInformer extsv1beta1informers.IngressInformer
	// secretInformer is an informer for Secret resources.
	secretInformer corev1informers.SecretInformer
	// serviceInformer is an informer for Service resources.
	serviceInformer corev1informers.ServiceInformer
}

// NewInformerBackedResourceCache returns a new cache that reads resources using listers obtained from the provided shared informer factory..
func NewInformerBackedResourceCache(factory kubeinformers.SharedInformerFactory) KubernetesResourceCache {
	return &informerBackedResourceCache{
		ingressInformer: factory.Extensions().V1beta1().Ingresses(),
		secretInformer:  factory.Core().V1().Secrets(),
		serviceInformer: factory.Core().V1().Services(),
	}
}

// HasSynced returns a value indicating whether the cache is synced.
func (c *informerBackedResourceCache) HasSynced() bool {
	return c.ingressInformer.Informer().HasSynced() && c.serviceInformer.Informer().HasSynced()
}

// GetIngress returns the Ingress resource with the specified namespace and name.
func (c *informerBackedResourceCache) GetIngress(namespace, name string) (*extsv1beta1.Ingress, error) {
	return c.ingressInformer.Lister().Ingresses(namespace).Get(name)
}

// GetIngresses returns a list of all Ingress resources in the specified namespace.
func (c *informerBackedResourceCache) GetIngresses(namespace string) ([]*extsv1beta1.Ingress, error) {
	return c.ingressInformer.Lister().Ingresses(namespace).List(labels.Everything())
}

// GetSecret returns the Secret resource with the specified namespace and name.
func (c *informerBackedResourceCache) GetSecret(namespace, name string) (*corev1.Secret, error) {
	return c.secretInformer.Lister().Secrets(namespace).Get(name)
}

// GetService returns the Service resource with the specified namespace and name.
func (c *informerBackedResourceCache) GetService(namespace, name string) (*corev1.Service, error) {
	return c.serviceInformer.Lister().Services(namespace).Get(name)
}

// GetServices returns a list of all Service resources in the specified namespace.
func (c *informerBackedResourceCache) GetServices(namespace string) ([]*corev1.Service, error) {
	return c.serviceInformer.Lister().Services(namespace).List(labels.Everything())
}
