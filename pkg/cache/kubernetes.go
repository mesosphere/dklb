package cache

import (
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
)

// KubernetesResourceCache knows how to list Kubernetes resources.
type KubernetesResourceCache interface {
	// HasSynced returns a value indicating whether the cache is synced.
	HasSynced() bool
	// GetIngress returns the Ingress resource with the specified namespace and name.
	GetIngress(string, string) (*extsv1beta1.Ingress, error)
	// GetIngresses returns a list of all Ingress resources in the specified namespace.
	GetIngresses(string) ([]*extsv1beta1.Ingress, error)
	// GetService returns the Service resource with the specified namespace and name.
	GetService(string, string) (*corev1.Service, error)
	// GetServices returns a list of all Service resources in the specified namespace.
	GetServices(string) ([]*corev1.Service, error)
}
