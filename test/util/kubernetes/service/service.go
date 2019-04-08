package service

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceCustomizer represents a function that can be used to customize a Service resource.
type ResourceCustomizer func(service *corev1.Service)

// DummyServiceResource returns a dummy, minimal Service resource with the specified namespace and name.
// If any customization functions are specified, they are run before the resource is returned.
func DummyServiceResource(namespace, name string, opts ...ResourceCustomizer) *corev1.Service {
	res := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: corev1.ServiceSpec{},
	}
	for _, fn := range opts {
		fn(res)
	}
	return res
}
