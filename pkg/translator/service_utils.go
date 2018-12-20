package translator

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mesosphere/dcos-edge-lb/models"
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

// servicePortBackendFrontend groups together the backend and frontend that correspond to a given service port.
type servicePortBackendFrontend struct {
	Backend  *models.V2Backend
	Frontend *models.V2Frontend
}

// backendNameForServicePort computes the name of the backend used for the specified service port.
func backendNameForServicePort(service *corev1.Service, port corev1.ServicePort) string {
	return fmt.Sprintf(serviceBackendNameFormatString, stringsutil.RemoveSlashes(cluster.KubernetesClusterFrameworkName), service.Namespace, service.Name, port.Port)
}

// frontendNameForServicePort computes the name of the frontend used for the specified service port.
func frontendNameForServicePort(service *corev1.Service, port corev1.ServicePort) string {
	return fmt.Sprintf(serviceFrontendNameFormatString, stringsutil.RemoveSlashes(cluster.KubernetesClusterFrameworkName), service.Namespace, service.Name, port.Port)
}

// serviceOwnedEdgeLBObjectMetadata groups together information about about the Service resource that owns a given EdgeLB backend/frontend.
type serviceOwnedEdgeLBObjectMetadata struct {
	// ClusterName is the name of the Kubernetes cluster to which the Service resource belongs.
	ClusterName string
	// Name is the name of the Service resource.
	Name string
	// Namespace is the namespace to which the Service resource belongs.
	Namespace string
	// ServicePort is the service port that corresponds to the current backend/frontend object.
	ServicePort int32
}

// IsOwnedBy indicates whether the current object is owned by the specified Service resource.
func (sp *serviceOwnedEdgeLBObjectMetadata) IsOwnedBy(service *corev1.Service) bool {
	return sp.ClusterName == stringsutil.RemoveSlashes(cluster.KubernetesClusterFrameworkName) && sp.Namespace == service.Namespace && sp.Name == service.Name
}

// computeServiceOwnedEdgeLBObjectMetadata parses the provided backend/frontend name and returns metadata about the Service resource that owns it.
func computeServiceOwnedEdgeLBObjectMetadata(name string) (*serviceOwnedEdgeLBObjectMetadata, error) {
	parts := strings.Split(name, separator)
	if len(parts) != 4 {
		return nil, errors.New("invalid backend/frontend Name for service")
	}
	p, err := strconv.Atoi(parts[3])
	if err != nil {
		return nil, errors.New("invalid backend/frontend Name for service")
	}
	return &serviceOwnedEdgeLBObjectMetadata{
		ClusterName: parts[0],
		Namespace:   parts[1],
		Name:        parts[2],
		ServicePort: int32(p),
	}, nil
}

// poolInspectionReport is a utility struct used to convey information about the status of a pool (and the required changes) upon inspection.
type poolInspectionReport struct {
	Lines []string
}

// Report adds a formatted message to the pool inspection report.
func (pir *poolInspectionReport) Report(message string, args ...interface{}) {
	pir.Lines = append(pir.Lines, fmt.Sprintf(message, args...))
}

// String returns a string representation of the pool inspection report.
func (pir *poolInspectionReport) String() string {
	var sb strings.Builder
	for _, item := range pir.Lines {
		sb.WriteString("=> ")
		sb.WriteString(item)
		sb.WriteRune('\n')
	}
	return sb.String()
}
