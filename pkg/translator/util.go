package translator

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/mesosphere/dklb/pkg/cluster"
)

const (
	// backendNameFormatString is the format string used to compute the name for a backend.
	backendNameFormatString = "%s__%s__%s__%d"
	// frontendNameFormatString is the format string used to compute the name for a frontend.
	frontendNameFormatString = "%s__%s__%s__%d"
)

// backendNameForServicePort computes the name of the backend used for the specified service port.
func backendNameForServicePort(service *corev1.Service, port corev1.ServicePort) string {
	return replaceSlashes(fmt.Sprintf(backendNameFormatString, cluster.KubernetesClusterFrameworkName, service.Namespace, service.Name, port.Port))
}

// frontendNameForServicePort computes the name of the frontend used for  the specified service port.
func frontendNameForServicePort(service *corev1.Service, port corev1.ServicePort) string {
	return replaceSlashes(fmt.Sprintf(frontendNameFormatString, cluster.KubernetesClusterFrameworkName, service.Namespace, service.Name, port.Port))
}

// replaceSlashes replaces slashes in the specified string with underscores.
func replaceSlashes(v string) string {
	return strings.Replace(v, "/", "_", -1)
}
