package kubernetes

import (
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/mesosphere/dklb/pkg/constants"
)

// NewEventRecorderForNamespace returns an event recorder that can be used to emit Kubernetes events for object in the specified namespace.
// TODO (@bcustodio) Cache event recorders in a per-namespace basis?
func NewEventRecorderForNamespace(kubeClient kubernetes.Interface, namespace string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Debugf)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: corev1client.New(kubeClient.CoreV1().RESTClient()).Events(namespace)})
	return eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: constants.ComponentName})
}
