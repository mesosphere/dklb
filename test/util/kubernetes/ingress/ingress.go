package ingress

import (
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mesosphere/dklb/pkg/constants"
)

// ResourceCustomizer represents a function that can be used to customize an Ingress resource.
type ResourceCustomizer func(ingress *extsv1beta1.Ingress)

// DummyEdgeLBIngressResource returns a dummy, minimal Ingress resource with the specified namespace and name.
// If any customization functions are specified, they are run before the resource is returned.
func DummyEdgeLBIngressResource(namespace, name string, opts ...ResourceCustomizer) *extsv1beta1.Ingress {
	res := &extsv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.EdgeLBIngressClassAnnotationKey: constants.EdgeLBIngressClassAnnotationValue,
			},
			Namespace: namespace,
			Name:      name,
		},
		Spec: extsv1beta1.IngressSpec{},
	}
	for _, fn := range opts {
		fn(res)
	}
	return res
}
