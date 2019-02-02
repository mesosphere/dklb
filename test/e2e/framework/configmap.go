package framework

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigMapCustomizer represents a function that can be used to customize a ConfigMap resource.
type ConfigMapCustomizer func(configMap *corev1.ConfigMap)

// CreateConfigMap creates the ConfigMap resource with the specified namespace and name in the Kubernetes API after running it through the specified customization function.
func (f *Framework) CreateConfigMap(namespace, name string, fn ConfigMapCustomizer) (*corev1.ConfigMap, error) {
	svc := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	if fn != nil {
		fn(svc)
	}
	return f.KubeClient.CoreV1().ConfigMaps(namespace).Create(svc)
}
