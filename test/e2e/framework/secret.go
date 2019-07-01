package framework

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretCustomizer represents a function that can be used to customize a Secret resource.
type SecretCustomizer func(secret *corev1.Secret)

// CreateSecret creates the Secret resource with the specified namespace and name in the Kubernetes API after running it through the specified customization function.
func (f *Framework) CreateSecret(namespace, name string, fn SecretCustomizer) (*corev1.Secret, error) {
	svc := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: make(map[string]string),
			Namespace:   namespace,
			Name:        name,
		},
	}
	if fn != nil {
		fn(svc)
	}
	return f.KubeClient.CoreV1().Secrets(namespace).Create(svc)
}
