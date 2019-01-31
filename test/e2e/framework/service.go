package framework

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	watchapi "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/watch"

	"github.com/mesosphere/dklb/pkg/util/kubernetes"
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

// WaitUntilServiceCondition blocks until the specified condition function is verified, or until the provided context times out.
func (f *Framework) WaitUntilServiceCondition(ctx context.Context, service *corev1.Service, fn watch.ConditionFunc) error {
	// Create a selector that targets the specified Service resource.
	fs := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.namespace==%s,metadata.name==%s", service.Namespace, service.Name))
	// Grab a ListerWatcher with which we can watch the Service resource.
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fs.String()
			return f.KubeClient.CoreV1().Services(service.Namespace).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watchapi.Interface, error) {
			options.FieldSelector = fs.String()
			return f.KubeClient.CoreV1().Services(service.Namespace).Watch(options)
		},
	}
	// Watch for updates to the specified Service resource until fn is satisfied.
	last, err := watch.UntilWithSync(ctx, lw, &corev1.Service{}, nil, fn)
	if err != nil {
		return err
	}
	if last == nil {
		return fmt.Errorf("no events received for service %q", kubernetes.Key(service))
	}
	return nil
}

// WaitForPublicIPForService blocks until a public IP is reported for the specified Service resource, or until the provided context times out.
func (f *Framework) WaitForPublicIPForService(ctx context.Context, service *corev1.Service) (string, error) {
	var (
		result string
		err    error
	)
	// Wait until the Service resource reports a non-empty status.
	err = f.WaitUntilServiceCondition(ctx, service, func(event watchapi.Event) (b bool, e error) {
		switch event.Type {
		case watchapi.Added:
			fallthrough
		case watchapi.Modified:
			// Iterate over the entries in ".status.loadBalancer.ingress", storing each IP in "result".
			// This will cause "result" to hold the last reported IP when this function exits.
			// Due to the way the ".status" object is computed, this is reasonably guaranteed to be a public IP where the service can be reached.
			for _, e := range event.Object.(*corev1.Service).Status.LoadBalancer.Ingress {
				if e.IP != "" {
					result = e.IP
				}
			}
			return result != "", nil
		default:
			return false, fmt.Errorf("got event of unexpected type %q", event.Type)
		}
	})
	return result, err
}
