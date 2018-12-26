package framework

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceCustomizer represents a function that can be used to customize a Service resource.
type ServiceCustomizer func(service *corev1.Service)

// CreateServiceOfTypeLoadBalancer creates the Service resource of type LoadBalancer with the specified namespace and name in the Kubernetes API after running it through the specified customization function.
func (f *Framework) CreateServiceOfTypeLoadBalancer(namespace, name string, fn ServiceCustomizer) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}
	if fn != nil {
		fn(svc)
	}
	return f.KubeClient.CoreV1().Services(namespace).Create(svc)
}
