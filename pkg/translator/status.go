package translator

import (
	"context"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/mesosphere/dklb/pkg/edgelb/manager"
	"github.com/mesosphere/dklb/pkg/util/kubernetes"
)

// computeLoadBalancerStatus builds a "LoadBalancerStatus" object representing the status of the EdgeLB pool with the specified name.
// If we're successful in reading the EdgeLB pool's metadata, the returned object contains all reported DNS names, private IPs and public IPs (in this order).
// Otherwise, the returned object is nil.
func computeLoadBalancerStatus(manager manager.EdgeLBManager, poolName string, obj runtime.Object) *corev1.LoadBalancerStatus {
	// Retrieve the pool's metadata from the EdgeLB API server.
	ctx, fn := context.WithTimeout(context.Background(), defaultEdgeLBManagerTimeout)
	defer fn()
	m, err := manager.GetPoolMetadata(ctx, poolName)
	if err != nil {
		// If we've got an error while reading the EdgeLB pool's metadata, log it but do not propagate as that would mark translation as failed.
		log.Warnf("unable to report status for %q: %v", kubernetes.Key(obj), err)
		return nil
	}
	// Compute the set of DNS names, private IPs and public IPs for the pool.
	var (
		dnsNames   = make(map[string]bool)
		privateIPs = make(map[string]bool)
		publicIPs  = make(map[string]bool)
	)
	// Add all reported DNS names to the set of DNS names.
	if m.Aws != nil {
		for _, elb := range m.Aws.Elbs {
			if elb.DNS != "" {
				for _, listener := range elb.Listeners {
					// Check whether the target frontend belongs to the Service/Ingress resource being processed.
					if listener.LinkFrontend == nil {
						continue
					}
					var (
						isOwnedByObj bool
					)
					switch t := obj.(type) {
					case *corev1.Service:
						m, err := computeServiceOwnedEdgeLBObjectMetadata(*listener.LinkFrontend)
						isOwnedByObj = err == nil && m.IsOwnedBy(t)
					case *extsv1beta1.Ingress:
						m, err := computeIngressOwnedEdgeLBObjectMetadata(*listener.LinkFrontend)
						isOwnedByObj = err == nil && m.IsOwnedBy(t)
					default:
						return nil
					}
					// If the target frontend belongs to the Service/Ingress resource being processed, we add the DNS name of the ELB being processed.
					if isOwnedByObj {
						dnsNames[strings.ToLower(elb.DNS)] = true
					}
				}
			}
		}
	}
	// Iterate over all reported frontends, adding the corresponding private and public IPs.
	for _, frontend := range m.Frontends {
		// Check whether the current frontend belongs to the Service/Ingress resource being processed.
		var (
			isOwnedByObj bool
		)
		switch t := obj.(type) {
		case *corev1.Service:
			m, err := computeServiceOwnedEdgeLBObjectMetadata(frontend.Name)
			isOwnedByObj = err == nil && m.IsOwnedBy(t)
		case *extsv1beta1.Ingress:
			m, err := computeIngressOwnedEdgeLBObjectMetadata(frontend.Name)
			isOwnedByObj = err == nil && m.IsOwnedBy(t)
		default:
			return nil
		}
		// If the current frontend doesn't belong to the Service/Ingress resource being processed, we skip it.
		if !isOwnedByObj {
			continue
		}
		// Iterate over the list of reported endpoints.
		for _, endpoint := range frontend.Endpoints {
			// Add all reported private IPs to the set of private IPs.
			for _, ip := range endpoint.Private {
				privateIPs[ip] = true
			}
			// Add all reported public IPs to the set of public IPs.
			for _, ip := range endpoint.Public {
				publicIPs[ip] = true
			}
		}
	}
	// Build the "LoadBalancerStatus" object by adding each DNS name, private IP and public IP.
	res := &corev1.LoadBalancerStatus{}
	for _, name := range sortedMapKeys(dnsNames) {
		res.Ingress = append(res.Ingress, corev1.LoadBalancerIngress{
			Hostname: name,
		})
	}
	for _, ip := range sortedMapKeys(privateIPs) {
		res.Ingress = append(res.Ingress, corev1.LoadBalancerIngress{
			IP: ip,
		})
	}
	for _, ip := range sortedMapKeys(publicIPs) {
		res.Ingress = append(res.Ingress, corev1.LoadBalancerIngress{
			IP: ip,
		})
	}
	return res
}

// sortedMapKeys returns a slice containing all keys in the specified map, sorted in increasing order.
func sortedMapKeys(m map[string]bool) []string {
	sortedKeys := make([]string, 0, len(m))
	for k := range m {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)
	return sortedKeys
}
