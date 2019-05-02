package translator

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mesosphere/dcos-edge-lb/models"
	corev1 "k8s.io/api/core/v1"

	"github.com/mesosphere/dklb/pkg/constants"
	translatorapi "github.com/mesosphere/dklb/pkg/translator/api"
	"github.com/mesosphere/dklb/pkg/util/pointers"
	stringsutil "github.com/mesosphere/dklb/pkg/util/strings"
)

const (
	// serviceBackendNameFormatString is the format string used to compute the name for a backend for a given Service resource.
	serviceBackendNameFormatString = "%s" + separator + "%s" + separator + "%s" + separator + "%d"
	// serviceFrontendNameFormatString is the format string used to compute the name for a frontend for a given Service resource.
	serviceFrontendNameFormatString = serviceBackendNameFormatString
	// separator is the separator used between the different parts that comprise the name of a backend/frontend.
	separator = ":"
)

// servicePortBackendFrontend groups together the backend and frontend that correspond to a given service port.
type servicePortBackendFrontend struct {
	Backend  *models.V2Backend
	Frontend *models.V2Frontend
}

// backendNameForServicePort computes the name of the backend used for the specified service port.
func backendNameForServicePort(service *corev1.Service, port corev1.ServicePort, kubernetesClusterName string) string {
	return fmt.Sprintf(serviceBackendNameFormatString, stringsutil.ReplaceForwardSlashesWithDots(kubernetesClusterName), service.Namespace, service.Name, port.Port)
}

// frontendNameForServicePort computes the name of the frontend used for the specified service port.
func frontendNameForServicePort(service *corev1.Service, port corev1.ServicePort, kubernetesClusterName string) string {
	return fmt.Sprintf(serviceFrontendNameFormatString, stringsutil.ReplaceForwardSlashesWithDots(kubernetesClusterName), service.Namespace, service.Name, port.Port)
}

// serviceOwnedEdgeLBObjectMetadata groups together information about about the Service resource that owns a given EdgeLB backend/frontend.
type serviceOwnedEdgeLBObjectMetadata struct {
	// Name is the name of the Kubernetes cluster to which the Service resource belongs.
	ClusterName string
	// Name is the name of the Service resource.
	Name string
	// Namespace is the namespace to which the Service resource belongs.
	Namespace string
	// ServicePort is the service port that corresponds to the current backend/frontend object.
	ServicePort int32
}

// IsOwnedBy indicates whether the current object is owned by the specified Service resource.
func (sp *serviceOwnedEdgeLBObjectMetadata) IsOwnedBy(service *corev1.Service, kubernetesClusterName string) bool {
	return sp.ClusterName == kubernetesClusterName && sp.Namespace == service.Namespace && sp.Name == service.Name
}

// computeBackendForServicePort computes the backend that correspond to the specified service port.
func computeBackendForServicePort(service *corev1.Service, servicePort corev1.ServicePort, kubernetesClusterName string) *models.V2Backend {
	// Compute the name to give to the backend.
	return &models.V2Backend{
		Balance:  constants.EdgeLBBackendBalanceLeastConnections,
		Name:     backendNameForServicePort(service, servicePort, kubernetesClusterName),
		Protocol: models.V2ProtocolTCP,
		Services: []*models.V2Service{
			{
				Endpoint: &models.V2Endpoint{
					Check: &models.V2EndpointCheck{
						Enabled: pointers.NewBool(true),
					},
					Port: servicePort.NodePort,
					Type: models.V2EndpointTypeCONTAINERIP,
				},
				Marathon: &models.V2ServiceMarathon{
					// We don't want to use any Marathon service as the backend.
				},
				Mesos: &models.V2ServiceMesos{
					FrameworkName:   kubernetesClusterName,
					TaskNamePattern: constants.KubeNodeTaskPattern,
				},
			},
		},
		// Explicitly set "RewriteHTTP" in order to make it easier to compare a computed backend with a V2Backend returned by the EdgeLB API server later on.
		RewriteHTTP: &models.V2RewriteHTTP{
			Request: &models.V2RewriteHTTPRequest{
				Forwardfor:                pointers.NewBool(true),
				RewritePath:               pointers.NewBool(true),
				SetHostHeader:             pointers.NewBool(true),
				XForwardedPort:            pointers.NewBool(true),
				XForwardedProtoHTTPSIfTLS: pointers.NewBool(true),
			},
			Response: &models.V2RewriteHTTPResponse{
				RewriteLocation: pointers.NewBool(true),
			},
		},
	}
}

// computeFrontendForServicePort computes the frontend that correspond to the specified service port.
func computeFrontendForServicePort(service *corev1.Service, spec translatorapi.ServiceEdgeLBPoolSpec, servicePort corev1.ServicePort, kubernetesClusterName string) *models.V2Frontend {
	var (
		bindPort     int32
		frontendName string
	)

	// If a cloud-provider configuration is being specified, force a dynamic frontend port.
	// Otherwise, compute the value to use as the frontend bind port, falling back to the service port in case one isn't provided.
	if *spec.CloudProviderConfiguration != "" {
		bindPort = 0
	} else {
		bindPort = servicePort.Port
		for _, frontend := range spec.Frontends {
			if frontend.ServicePort == servicePort.Port {
				bindPort = *frontend.Port
			}
		}
	}

	// Compute the name to give to the frontend.
	frontendName = frontendNameForServicePort(service, servicePort, kubernetesClusterName)
	// Compute the backend and frontend objects and return them.
	return &models.V2Frontend{
		BindAddress: constants.EdgeLBFrontendBindAddress,
		Name:        frontendName,
		Protocol:    models.V2ProtocolTCP,
		BindPort:    &bindPort,
		LinkBackend: &models.V2FrontendLinkBackend{
			DefaultBackend: backendNameForServicePort(service, servicePort, kubernetesClusterName),
		},
	}
}

// computeServiceOwnedEdgeLBObjectMetadata parses the provided backend/frontend name and returns metadata about the Service resource that owns it.
func computeServiceOwnedEdgeLBObjectMetadata(name string) (*serviceOwnedEdgeLBObjectMetadata, error) {
	parts := strings.Split(name, separator)
	if len(parts) != 4 {
		return nil, errors.New("invalid backend/frontend name for service")
	}
	p, err := strconv.Atoi(parts[3])
	if err != nil {
		return nil, errors.New("invalid backend/frontend name for service")
	}
	return &serviceOwnedEdgeLBObjectMetadata{
		ClusterName: stringsutil.ReplaceDotsWithForwardSlashes(parts[0]),
		Namespace:   parts[1],
		Name:        parts[2],
		ServicePort: int32(p),
	}, nil
}
