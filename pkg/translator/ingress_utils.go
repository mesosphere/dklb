package translator

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mesosphere/dcos-edge-lb/models"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/util/pointers"
	dklbstrings "github.com/mesosphere/dklb/pkg/util/strings"
)

const (
	// edgeLBIngressBackendNameFormatString is the format string used to compute the name for an EdgeLB backend corresponding to a given Ingress backend.
	// The resulting name is of the form "<cluster-name>:<ingress-namespace>:<ingress-name>:<service-name>:<service-port>".
	edgeLBIngressBackendNameFormatString = "%s" + separator + "%s" + separator + "%s" + separator + "%s" + separator + "%s"
	// edgeLBIngressFrontendNameFormatString is the format string used to compute the name for an EdgeLB frontend corresponding to a given Ingress resource.
	// The resulting name is of the form "<cluster-name>:<ingress-namespace>:<ingress-name>".
	edgeLBIngressFrontendNameFormatString = "%s" + separator + "%s" + separator + "%s"
	// edgeLBPathCatchAllRegex is the regular expression used by EdgeLB to match all paths.
	edgeLBPathCatchAllRegex = "^.*$"
	// edgeLBPathRegexFormatString is the format string used to compute the regular expression used by EdgeLB to match a given path.
	edgeLBPathRegexFormatString = "^%s$"
)

// IngressBackendNodePortMap represents a mapping between Ingress backends and their target node ports.
type IngressBackendNodePortMap map[extsv1beta1.IngressBackend]int32

// ingressOwnedEdgeLBObjectMetadata groups together information about the Ingress resource that owns a given EdgeLB backend/frontend.
type ingressOwnedEdgeLBObjectMetadata struct {
	// ClusterName is the name of the Kubernetes cluster to which the Ingress resource belongs.
	ClusterName string
	// Name is the name of the Ingress resource.
	Name string
	// Namespace is the namespace to which the Ingress resource belongs.
	Namespace string
	// IngressBackend is the reconstructed IngressBackend object represented by the current EdgeLB object (in case said object is an EdgeLB backend).
	IngressBackend *extsv1beta1.IngressBackend
}

// IsOwnedBy indicates whether the current EdgeLB object is owned by the specified Ingress resource.
func (m *ingressOwnedEdgeLBObjectMetadata) IsOwnedBy(clusterName string, ingress *extsv1beta1.Ingress) bool {
	return m.ClusterName == clusterName && m.Namespace == ingress.Namespace && m.Name == ingress.Name
}

// computeEdgeLBBackendForIngressBackend computes the EdgeLB backend that corresponds to the specified Ingress backend.
func computeEdgeLBBackendForIngressBackend(clusterName string, ingress *extsv1beta1.Ingress, backend extsv1beta1.IngressBackend, nodePort int32) *models.V2Backend {
	return &models.V2Backend{
		Balance: constants.EdgeLBBackendBalanceLeastConnections,
		Name:    computeEdgeLBBackendNameForIngressBackend(clusterName, ingress, backend),
		// TODO (@bcustodio) Understand if/when we need to use HTTPS here.
		Protocol: models.V2ProtocolHTTP,
		Services: []*models.V2Service{
			{
				Endpoint: &models.V2Endpoint{
					Check: &models.V2EndpointCheck{
						Enabled: pointers.NewBool(true),
					},
					Port: nodePort,
					Type: models.V2EndpointTypeCONTAINERIP,
				},
				Marathon: &models.V2ServiceMarathon{
					// We don't want to use any Marathon service as the backend.
				},
				Mesos: &models.V2ServiceMesos{
					FrameworkName:   clusterName,
					TaskNamePattern: constants.KubeNodeTaskPattern,
				},
			},
		},
		RewriteHTTP: &models.V2RewriteHTTP{
			Request: &models.V2RewriteHTTPRequest{
				// Add the "X-Forwarded-For" header to requests.
				Forwardfor: pointers.NewBool(true),
				// Add the "X-Forwarded-Port" header to requests.
				XForwardedPort: pointers.NewBool(true),
				// Add the "X-Forwarded-Proto" header to requests.
				XForwardedProtoHTTPSIfTLS: pointers.NewBool(true),
				// Explicitly disable rewriting of paths.
				RewritePath: pointers.NewBool(false),
				// Explicitly disable setting the "Host" header on requests.
				// This header should be set by clients alone.
				SetHostHeader: pointers.NewBool(false),
			},
			Response: &models.V2RewriteHTTPResponse{
				// Explicitly disable rewriting locations.
				RewriteLocation: pointers.NewBool(false),
			},
		},
	}
}

// computeEdgeLBBackendNameForIngressBackend computes the name of the EdgeLB backend that corresponds to the specified Ingress backend.
func computeEdgeLBBackendNameForIngressBackend(clusterName string, ingress *extsv1beta1.Ingress, backend extsv1beta1.IngressBackend) string {
	return fmt.Sprintf(edgeLBIngressBackendNameFormatString, dklbstrings.ReplaceForwardSlashesWithDots(clusterName), ingress.Namespace, ingress.Name, backend.ServiceName, backend.ServicePort.String())
}

// computeEdgeLBFrontendForIngress computes the EdgeLB frontend that corresponds to the specified Ingress resource.
func computeEdgeLBFrontendForIngress(clusterName string, ingress *extsv1beta1.Ingress, options IngressTranslationOptions) *models.V2Frontend {
	// Compute the base frontend object.
	frontend := &models.V2Frontend{
		BindAddress: constants.EdgeLBFrontendBindAddress,
		Name:        computeEdgeLBFrontendNameForIngress(clusterName, ingress),
		// TODO (@bcustodio) Split into HTTP/HTTPS port when TLS support is introduced.
		Protocol:    models.V2ProtocolHTTP,
		BindPort:    &options.EdgeLBPoolPort,
		LinkBackend: &models.V2FrontendLinkBackend{},
	}

	// hostItems will contain "V2FrontendLinkBackendMapItems0" items for rules that specify a ".host" field.
	// These should take precedence over (i.e. be matched before) any rules that don't specify said field.
	hostItems := make([]*models.V2FrontendLinkBackendMapItems0, 0)
	// pathItems will contain "V2FrontendLinkBackendMapItems0" items for rules that don't specify a ".host" field.
	pathItems := make([]*models.V2FrontendLinkBackendMapItems0, 0)

	// Iterate over Ingress backends, building the corresponding "V2FrontendLinkBackendMapItems0" EdgeLB object.
	forEachIngresBackend(ingress, func(host, path *string, backend extsv1beta1.IngressBackend) {
		switch {
		case host == nil && path == nil:
			frontend.LinkBackend.DefaultBackend = computeEdgeLBBackendNameForIngressBackend(clusterName, ingress, backend)
		default:
			item := &models.V2FrontendLinkBackendMapItems0{
				Backend: computeEdgeLBBackendNameForIngressBackend(clusterName, ingress, backend),
				HostEq:  *host,
			}
			if *path == "" {
				// A ".path" field has not been specified, so the current rule should catch all requests.
				item.PathReg = edgeLBPathCatchAllRegex
			} else {
				// A ".path" field has been specified, so we should use it to match requests.
				// TODO (@bcustodio) HAProxy uses PCRE regular expressions, while the Ingress spec dictates that regular expressions follow the egrep (IEEE Std 1003.1) syntax.
				// TODO (@bcustodio) https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#httpingresspath-v1beta1-extensions
				// TODO (@bcustodio) We need to understand whether "translation" is required/desirable (and possible), or accept PCRE and document that the syntax for paths does not follow the spec.
				item.PathReg = fmt.Sprintf(edgeLBPathRegexFormatString, *path)
			}
			if *host == "" {
				// A ".host" field has not been specified, so the current rule should be matched last.
				pathItems = append(pathItems, item)
			} else {
				// A ".host" field has been specified, so the current rule should be matched first.
				hostItems = append(hostItems, item)
			}
		}
	})

	// Build the final map by concatenating "hostItems" and  "pathItems".
	frontend.LinkBackend.Map = append(frontend.LinkBackend.Map, hostItems...)
	frontend.LinkBackend.Map = append(frontend.LinkBackend.Map, pathItems...)
	// Return the computed EdgeLB frontend object.
	return frontend
}

// computeEdgeLBFrontendNameForIngress computes the name of the EdgeLB frontend that corresponds to the specified Ingress resource.
func computeEdgeLBFrontendNameForIngress(clusterName string, ingress *extsv1beta1.Ingress) string {
	return fmt.Sprintf(edgeLBIngressFrontendNameFormatString, dklbstrings.ReplaceForwardSlashesWithDots(clusterName), ingress.Namespace, ingress.Name)
}

// computeServiceOwnedEdgeLBObjectMetadata parses the provided EdgeLB backend/frontend name and returns metadata about the Ingress resource that owns it.
func computeIngressOwnedEdgeLBObjectMetadata(name string) (*ingressOwnedEdgeLBObjectMetadata, error) {
	// Split the provided name by "separator".
	parts := strings.Split(name, separator)
	// Check how many parts we are dealing with, and act accordingly.
	switch len(parts) {
	case 5:
		// The provided name is composed of 5 parts separated by "separator".
		// Hence, it most likely corresponds to an EdgeLB backend owned by an Ingress resource.
		return &ingressOwnedEdgeLBObjectMetadata{
			ClusterName: dklbstrings.ReplaceDotsWithForwardSlashes(parts[0]),
			Namespace:   parts[1],
			Name:        parts[2],
			// Reconstruct the Ingress backend so we can compare it with the computed (desired) state later on.
			IngressBackend: &extsv1beta1.IngressBackend{
				ServiceName: parts[3],
				ServicePort: intstr.Parse(parts[4]),
			},
		}, nil
	case 3:
		// The provided name is composed of 3 parts separated by "separator".
		// Hence, it most likely corresponds to an EdgeLB frontend owned by an Ingress resource.
		return &ingressOwnedEdgeLBObjectMetadata{
			ClusterName:    dklbstrings.ReplaceDotsWithForwardSlashes(parts[0]),
			Namespace:      parts[1],
			Name:           parts[2],
			IngressBackend: nil,
		}, nil
	default:
		// The provided name is composed of a different number of parts.
		// Hence, it does not correspond to an Ingress-owned EdgeLB object.
		return nil, errors.New("invalid backend/frontend name for ingress")
	}
}

// forEachIngressBackend iterates over Ingress backends defined on the specified Ingress resource, calling "fn" with each Ingress backend object and the associated host and path whenever applicable.
func forEachIngresBackend(ingress *extsv1beta1.Ingress, fn func(host *string, path *string, backend extsv1beta1.IngressBackend)) {
	if ingress.Spec.Backend != nil {
		// Use nil values for "host" and "path" so that the caller can identify the current Ingress backend as the default one if it needs to.
		fn(nil, nil, *ingress.Spec.Backend)
	}
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				// Use the specified (possibly empty) values for "host" and "path".
				fn(&rule.Host, &path.Path, path.Backend)
			}
		}
	}
}
