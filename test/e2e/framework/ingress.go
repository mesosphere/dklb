package framework

import (
	"context"
	"fmt"
	"strings"

	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	watchapi "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/watch"

	"github.com/mesosphere/dklb/pkg/constants"
	translatorapi "github.com/mesosphere/dklb/pkg/translator/api"
	"github.com/mesosphere/dklb/pkg/util/kubernetes"
)

// IngressCustomizer represents a function that can be used to customize an Ingress resource.
type IngressCustomizer func(ingress *extsv1beta1.Ingress)

type IngressEdgeLBPoolSpecCustomizer func(spec *translatorapi.IngressEdgeLBPoolSpec)

// CreateIngressFromYamlSpec creates the Ingress resource from the given spec.
func (f *Framework) CreateIngressFromYamlSpec(spec string) (*extsv1beta1.Ingress, error) {
	ingress := &extsv1beta1.Ingress{}
	decoder := yaml.NewYAMLToJSONDecoder(strings.NewReader(spec))
	err := decoder.Decode(ingress)
	if err != nil {
		return nil, err
	}

	return f.KubeClient.ExtensionsV1beta1().Ingresses(ingress.Namespace).Create(ingress)
}

// CreateIngress creates the Ingress resource with the specified namespace and name in the Kubernetes API after running it through the specified customization function.
func (f *Framework) CreateIngress(namespace, name string, fn IngressCustomizer) (*extsv1beta1.Ingress, error) {
	ingress := &extsv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: make(map[string]string),
			Namespace:   namespace,
			Name:        name,
		},
	}
	if fn != nil {
		fn(ingress)
	}
	return f.KubeClient.ExtensionsV1beta1().Ingresses(namespace).Create(ingress)
}

// CreateEdgeLBIngress creates the Ingress resource with the specified namespace and name in the Kubernetes API after running it through the specified customization function.
// The Ingress is explicitly annotated to be provisioned by EdgeLB.
func (f *Framework) CreateEdgeLBIngress(namespace, name string, fn IngressCustomizer) (*extsv1beta1.Ingress, error) {
	return f.CreateIngress(namespace, name, func(ingress *extsv1beta1.Ingress) {
		fn(ingress)
		ingress.Annotations[constants.EdgeLBIngressClassAnnotationKey] = constants.EdgeLBIngressClassAnnotationValue
	})
}

// UpdateIngressEdgeLBPoolSpec updates the EdgeLB pool specification contained in specified Ingress resource according to the supplied customization function.
func (f *Framework) UpdateIngressEdgeLBPoolSpec(ingress *extsv1beta1.Ingress, fn IngressEdgeLBPoolSpecCustomizer) (*extsv1beta1.Ingress, error) {
	s, err := translatorapi.GetIngressEdgeLBPoolSpec(ingress)
	if err != nil {
		return nil, err
	}
	fn(s)
	_ = translatorapi.SetIngressEdgeLBPoolSpec(ingress, s)
	return f.KubeClient.ExtensionsV1beta1().Ingresses(ingress.Namespace).Update(ingress)
}

// WaitUntilIngressCondition blocks until the specified condition function is verified, or until the provided context times out.
func (f *Framework) WaitUntilIngressCondition(ctx context.Context, ingress *extsv1beta1.Ingress, fn watch.ConditionFunc) error {
	// Create a selector that targets the specified Ingress resource.
	fs := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.namespace==%s,metadata.name==%s", ingress.Namespace, ingress.Name))
	// Grab a ListerWatcher with which we can watch the Ingress resource.
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fs.String()
			return f.KubeClient.ExtensionsV1beta1().Ingresses(ingress.Namespace).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watchapi.Interface, error) {
			options.FieldSelector = fs.String()
			return f.KubeClient.ExtensionsV1beta1().Ingresses(ingress.Namespace).Watch(options)
		},
	}
	// Watch for updates to the specified Ingress resource until fn is satisfied.
	last, err := watch.UntilWithSync(ctx, lw, &extsv1beta1.Ingress{}, nil, fn)
	if err != nil {
		return err
	}
	if last == nil {
		return fmt.Errorf("no events received for ingress %q", kubernetes.Key(ingress))
	}
	return nil
}

// WaitForPublicIPForIngress blocks until a public IP is reported for the specified Service resource, or until the provided context times out.
func (f *Framework) WaitForPublicIPForIngress(ctx context.Context, ingress *extsv1beta1.Ingress) (string, error) {
	var (
		result string
		err    error
	)
	// Wait until the Ingress resource reports a non-empty status.
	err = f.WaitUntilIngressCondition(ctx, ingress, func(event watchapi.Event) (b bool, e error) {
		switch event.Type {
		case watchapi.Added:
			fallthrough
		case watchapi.Modified:
			// Iterate over the entries in ".status.loadBalancer.ingress", storing each IP in "result".
			// This will cause "result" to hold the last reported IP when this function exits.
			// Due to the way the ".status" object is computed, this is reasonably guaranteed to be a public IP where the ingress can be reached.
			for _, e := range event.Object.(*extsv1beta1.Ingress).Status.LoadBalancer.Ingress {
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
