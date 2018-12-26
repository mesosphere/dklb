package framework

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodCustomizer represents a function that can be used to customize a Pod resource.
type PodCustomizer func(pod *corev1.Pod)

// CreatePod creates the pod with the specified namespace and name in the Kubernetes API after running it through the specified customization function.
func (f *Framework) CreatePod(namespace, name string, fn PodCustomizer) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	if fn != nil {
		fn(pod)
	}
	return f.KubeClient.CoreV1().Pods(namespace).Create(pod)
}
