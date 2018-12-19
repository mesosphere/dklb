package translator

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/mesosphere/dklb/pkg/cluster"
	stringsutil "github.com/mesosphere/dklb/pkg/util/strings"
)

const (
	// serviceBackendNameFormatString is the format string used to compute the name for a backend for a given Service resource.
	serviceBackendNameFormatString = "%s" + separator + "%s" + separator + "%s" + separator + "%d"
	// serviceFrontendNameFormatString is the format string used to compute the name for a frontend for a given Service resource.
	serviceFrontendNameFormatString = serviceBackendNameFormatString
	// separator is the separator used between the different parts that comprise the name of a backend/frontend.
	separator = "__"
)

// backendNameForServicePort computes the name of the backend used for the specified service port.
func backendNameForServicePort(service *corev1.Service, port corev1.ServicePort) string {
	return fmt.Sprintf(serviceBackendNameFormatString, stringsutil.RemoveSlashes(cluster.KubernetesClusterFrameworkName), service.Namespace, service.Name, port.Port)
}

// frontendNameForServicePort computes the name of the frontend used for  the specified service port.
func frontendNameForServicePort(service *corev1.Service, port corev1.ServicePort) string {
	return fmt.Sprintf(serviceFrontendNameFormatString, stringsutil.RemoveSlashes(cluster.KubernetesClusterFrameworkName), service.Namespace, service.Name, port.Port)
}
