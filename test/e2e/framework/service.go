package framework

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// echoServicePort is the service port used by "echo" services.
	echoServicePort = 80
)

// ServiceCustomizer represents a function that can be used to customize a Service resource.
type ServiceCustomizer func(service *corev1.Service)

// CreateService creates the Service resource with the specified namespace and name in the Kubernetes API after running it through the specified customization function.
func (f *Framework) CreateService(namespace, name string, fn ServiceCustomizer) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	if fn != nil {
		fn(svc)
	}
	return f.KubeClient.CoreV1().Services(namespace).Create(svc)
}

// CreateServiceOfTypeLoadBalancer creates the Service resource of type LoadBalancer with the specified namespace and name in the Kubernetes API after running it through the specified customization function.
func (f *Framework) CreateServiceOfTypeLoadBalancer(namespace, name string, fn ServiceCustomizer) (*corev1.Service, error) {
	return f.CreateService(namespace, name, func(service *corev1.Service) {
		fn(service)
		service.Spec.Type = corev1.ServiceTypeLoadBalancer
	})
}

// CreateServiceForEchoPod creates a Service resource of type NodePort targeting the specified "echo" pod.
func (f *Framework) CreateServiceForEchoPod(pod *corev1.Pod) (*corev1.Service, error) {
	return f.CreateService(pod.Namespace, pod.Name, func(service *corev1.Service) {
		service.Labels = pod.Labels
		service.Spec.Selector = pod.Labels
		service.Spec.Type = corev1.ServiceTypeNodePort
		service.Spec.Ports = []corev1.ServicePort{
			{
				Name:       pod.Name,
				Port:       echoServicePort,
				TargetPort: intstr.FromInt(int(pod.Spec.Containers[0].Ports[0].ContainerPort)),
			},
		}
	})
}
