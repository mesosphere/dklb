package translator

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mesosphere/dcos-edge-lb/models"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/mesosphere/dklb/pkg/constants"
	translatorapi "github.com/mesosphere/dklb/pkg/translator/api"
	kubernetesutil "github.com/mesosphere/dklb/pkg/util/kubernetes"
	"github.com/mesosphere/dklb/pkg/util/pointers"
	dklbstrings "github.com/mesosphere/dklb/pkg/util/strings"
)

const (
	// edgeLBHostCatchAllRegex is the regular expression used by EdgeLB to match all hosts.
	edgeLBHostCatchAllRegex = "^.*$"
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
	// Name is the name of the Kubernetes cluster to which the Ingress resource belongs.
	ClusterName string
	// Name is the name of the Ingress resource.
	Name string
	// Namespace is the namespace to which the Ingress resource belongs.
	Namespace string
	// IngressBackend is the reconstructed IngressBackend object represented by the current EdgeLB object (in case said object is an EdgeLB backend).
	IngressBackend *extsv1beta1.IngressBackend
}

// prioritizedMatchingRule is a helper struct used to associate a priority with an EdgeLB "V2FrontendLinkBackendMapItems0".
type prioritizedMatchingRule struct {
	item     *models.V2FrontendLinkBackendMapItems0
	priority int
}

// IsOwnedBy indicates whether the current EdgeLB object is owned by the specified Ingress resource.
func (m *ingressOwnedEdgeLBObjectMetadata) IsOwnedBy(ingress *extsv1beta1.Ingress, kubernetesClusterName string) bool {
	return m.ClusterName == kubernetesClusterName && m.Namespace == ingress.Namespace && m.Name == ingress.Name
}

// computeEdgeLBBackendForIngressBackend computes the EdgeLB backend that corresponds to the specified Ingress backend.
func computeEdgeLBBackendForIngressBackend(ingress *extsv1beta1.Ingress, backend extsv1beta1.IngressBackend, nodePort int32, kubernetesClusterName string) *models.V2Backend {
	return &models.V2Backend{
		Balance:  constants.EdgeLBBackendBalanceLeastConnections,
		Name:     computeEdgeLBBackendNameForIngressBackend(ingress, backend, kubernetesClusterName),
		Protocol: models.V2ProtocolHTTP,
		// At this point we would need to know if the target server is TLS-enabled or not so that we could configure HAProxy accordingly.
		// This is so because when the target server is TLS-enabled, HAProxy **MUST** be configured with the "ssl verify none" option (so that it actually communicates over TLS **AND** skips certificate verification).
		// On the other hand, specifying said option when the target server is **NOT** TLS-enabled will cause HAProxy to be unable to communicate with said server.
		// Hence, we add the same server twice:
		// * We add the TLS-enabled variant of the server as the preferred server, forcing health-checks to happen over TLS.
		//   This version will be the one used whenever the server responds adequately to said TLS-enabled health-checks (and only in this situation).
		// * We add the TLS-disabled variant of the server as the "backup" server.
		//   This version will be the one used whenever the server does not respond adequately to TLS-enabled health-checks (and only in this situation).
		// This will result in an HAProxy config similar to the following one:
		//
		// backend ingress-backend
		//    mode http
		//    server 1.2.3.4:5678 check check-ssl ssl verify none
		//    server 4.3.2.1:5678 check check-ssl ssl verify none
		//    server 1.2.3.4:5678 check backup
		//    server 4.3.2.1:5678 check backup
		Services: []*models.V2Service{
			{
				Endpoint: &models.V2Endpoint{
					Check: &models.V2EndpointCheck{
						Enabled: pointers.NewBool(true),
					},
					MiscStr: computeEdgeLBBackendMiscStr(constants.EdgeLBBackendTLSCheck, constants.EdgeLBBackendInsecureSkipTLSVerify),
					Port:    nodePort,
					Type:    models.V2EndpointTypeCONTAINERIP,
				},
				Marathon: &models.V2ServiceMarathon{
					// We don't want to use any Marathon service as the backend.
				},
				Mesos: &models.V2ServiceMesos{
					FrameworkName:   kubernetesClusterName,
					TaskNamePattern: constants.KubeNodeTaskPattern,
				},
			},
			{
				Endpoint: &models.V2Endpoint{
					Check: &models.V2EndpointCheck{
						Enabled: pointers.NewBool(true),
					},
					MiscStr: computeEdgeLBBackendMiscStr(constants.EdgeLBBackendBackup),
					Port:    nodePort,
					Type:    models.V2EndpointTypeCONTAINERIP,
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
func computeEdgeLBBackendNameForIngressBackend(ingress *extsv1beta1.Ingress, backend extsv1beta1.IngressBackend, kubernetesClusterName string) string {
	return fmt.Sprintf(edgeLBIngressBackendNameFormatString, dklbstrings.ReplaceForwardSlashesWithDots(kubernetesClusterName), ingress.Namespace, ingress.Name, backend.ServiceName, backend.ServicePort.String())
}

// computeEdgeLBFrontendForIngress computes the EdgeLB frontend that corresponds to the specified Ingress resource.
func computeEdgeLBFrontendForIngress(ingress *extsv1beta1.Ingress, spec translatorapi.IngressEdgeLBPoolSpec, kubernetesClusterName string) *models.V2Frontend {
	// Compute the base frontend object.
	frontend := &models.V2Frontend{
		BindAddress: constants.EdgeLBFrontendBindAddress,
		Name:        computeEdgeLBFrontendNameForIngress(ingress, kubernetesClusterName),
		// TODO (@bcustodio) Split into HTTP/HTTPS port when TLS support is introduced.
		Protocol:    models.V2ProtocolHTTP,
		BindPort:    spec.Frontends.HTTP.Port,
		LinkBackend: &models.V2FrontendLinkBackend{},
	}

	// Create the slice that will hold the set of matching rules.
	var rules []prioritizedMatchingRule

	// Iterate over Ingress backends, building the corresponding "V2FrontendLinkBackendMapItems0" EdgeLB object.
	kubernetesutil.ForEachIngresBackend(ingress, func(host, path *string, backend extsv1beta1.IngressBackend) {
		switch {
		case host == nil && path == nil:
			frontend.LinkBackend.DefaultBackend = computeEdgeLBBackendNameForIngressBackend(ingress, backend, kubernetesClusterName)
		default:
			rule := prioritizedMatchingRule{
				item: &models.V2FrontendLinkBackendMapItems0{
					Backend: computeEdgeLBBackendNameForIngressBackend(ingress, backend, kubernetesClusterName),
				},
				priority: 0,
			}

			switch {
			case host == nil || *host == "":
				// No value (or an empty value) was specified for ".host".
				// Hence we set this rule's priority as the lowest possible one, causing HAProxy to match it only **AFTER** any other rules specifying a non-empty ".host".
				rule.item.HostReg = edgeLBHostCatchAllRegex
				rule.priority = math.MinInt32
			default:
				// A non-empty value was specified for ".host".
				// Hence we set this rule's priority to a normal level, causing HAProxy to match it **BEFORE** any other rules specifying an empty ".host".
				rule.item.HostEq = *host
				rule.priority = 0
			}

			switch {
			case path == nil || *path == "":
				// No value (or an empty value) was specified for ".path".
				// Hence we keep this rule's priority as-is, causing HAProxy to match it only **AFTER** any other rules specifying a non-empty ".path".
				rule.item.PathReg = edgeLBPathCatchAllRegex
				rule.priority += 0
			default:
				// A non-empty value was specified for ".path".
				// Hence we add the length of the path to this rule's priority, causing HAProxy to match it **BEFORE** any other rules specifying a shorter ".path".
				// TODO (@bcustodio) HAProxy uses PCRE regular expressions, while the Ingress spec dictates that regular expressions follow the egrep (IEEE Std 1003.1) syntax.
				// TODO (@bcustodio) https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#httpingresspath-v1beta1-extensions
				// TODO (@bcustodio) We need to understand whether "translation" is required/desirable (and possible), or accept PCRE and document that the syntax for paths does not follow the spec.
				rule.item.PathReg = fmt.Sprintf(edgeLBPathRegexFormatString, *path)
				rule.priority += len(*path)
			}

			rules = append(rules, rule)
		}
	})

	// Sort rules by descending order of their priority.
	sort.SliceStable(rules, func(i, j int) bool {
		return rules[i].priority > rules[j].priority
	})
	// Add each rule to the final slice of rules.
	for _, rule := range rules {
		frontend.LinkBackend.Map = append(frontend.LinkBackend.Map, rule.item)
	}
	// Return the computed EdgeLB frontend object.
	return frontend
}

// computeEdgeLBFrontendNameForIngress computes the name of the EdgeLB frontend that corresponds to the specified Ingress resource.
func computeEdgeLBFrontendNameForIngress(ingress *extsv1beta1.Ingress, kubernetesClusterName string) string {
	return fmt.Sprintf(edgeLBIngressFrontendNameFormatString, dklbstrings.ReplaceForwardSlashesWithDots(kubernetesClusterName), ingress.Namespace, ingress.Name)
}

// computeEdgeLBBackendMiscStr computes the value to be used as "miscStr" on a given backend given the specified options.
func computeEdgeLBBackendMiscStr(options ...string) string {
	return strings.Join(options, " ")
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
