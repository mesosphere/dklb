package framework

import (
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressCustomizer represents a function that can be used to customize an Ingress resource.
type IngressCustomizer func(ingress *extsv1beta1.Ingress)

// CreateIngress creates the Ingress resource with the specified namespace and name in the Kubernetes API after running it through the specified customization function.
func (f *Framework) CreateIngress(namespace, name string, fn IngressCustomizer) (*extsv1beta1.Ingress, error) {
	ingress := &extsv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	if fn != nil {
		fn(ingress)
	}
	return f.KubeClient.ExtensionsV1beta1().Ingresses(namespace).Create(ingress)
}
