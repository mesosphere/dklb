package secret

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceCustomizer represents a function that can be used to
// customize a Secret resource.
type ResourceCustomizer func(secret *corev1.Secret)

// DummySecretResource returns a dummy, minimal Secret resource with
// the specified namespace and name.  If any customization functions
// are specified, they are run before the resource is returned.
func DummySecretResource(namespace, name string, opts ...ResourceCustomizer) *corev1.Secret {
	res := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Annotations: map[string]string{},
		},
		Data: map[string][]byte{},
	}
	for _, fn := range opts {
		fn(res)
	}
	return res
}
