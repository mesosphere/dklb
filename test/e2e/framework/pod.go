package framework

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// echoContainerPort is the container port used by "echo" pods.
	echoContainerPort = 8080
	// echoImage is the image used to create "echo" pods.
	echoImage = "gcr.io/k8s-ingress-image-push/ingress-gce-echo-amd64:master"
	// echoPodEnvVarName is the name of the "pod" environment variable set on "echo" pods.
	echoPodEnvVarName = "pod"
	// echoNamespaceEnvVarName is the name of the "namespace" environment variable set on "echo" pods.
	echoNamespaceEnvVarName = "namespace"
	// echoPodLabelKey is the key of the "pod" label set on "echo" pods.
	echoPodLabelKey = "pod"
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

// CreateEchoPod creates an "echo" pod in the specified namespace with the provided name.
func (f *Framework) CreateEchoPod(namespace, name string) (*corev1.Pod, error) {
	return f.CreatePod(namespace, name, func(pod *corev1.Pod) {
		pod.Labels = map[string]string{
			echoPodLabelKey: name,
		}
		pod.Spec.Containers = []corev1.Container{
			{
				Name:  name,
				Image: echoImage,
				Ports: []corev1.ContainerPort{
					{
						ContainerPort: echoContainerPort,
					},
				},
				Env: []corev1.EnvVar{
					{
						Name:  echoNamespaceEnvVarName,
						Value: namespace,
					},
					{
						Name:  echoPodEnvVarName,
						Value: name,
					},
				},
			},
		}
	})
}
