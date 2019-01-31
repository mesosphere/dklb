package translator

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/mesosphere/dklb/pkg/edgelb/manager"
)

// computeLoadBalancerStatus builds an "LoadBalancerStatus" object representing the status of the EdgeLB pool with the specified name.
// The returned object contains all reported DNS names, private IPs and public IPs (in this order).
func computeLoadBalancerStatus(manager manager.EdgeLBManager, poolName, clusterName string, obj runtime.Object) (*corev1.LoadBalancerStatus, error) {
	// Retrieve the pool's metadata from the EdgeLB API server.
	ctx, fn := context.WithTimeout(context.Background(), defaultEdgeLBManagerTimeout)
	defer fn()
	m, err := manager.GetPoolMetadata(ctx, poolName)
	if err != nil {
		return nil, err
	}
	// Compute the set of DNS names, private IPs and public IPs for the pool by iterating over the list of reported frontends.
	var (
		dnsNames   = make(map[string]bool)
		privateIPs = make(map[string]bool)
		publicIPs  = make(map[string]bool)
	)
	for _, frontend := range m.Frontends {
		// Check whether the current frontend belongs to the Service/Ingress resource being processed.
		var (
			isOwnedByObj bool
		)
		switch t := obj.(type) {
		case *corev1.Service:
			m, err := computeServiceOwnedEdgeLBObjectMetadata(frontend.Name)
			isOwnedByObj = err == nil && m.IsOwnedBy(clusterName, t)
		case *extsv1beta1.Ingress:
			m, err := computeIngressOwnedEdgeLBObjectMetadata(frontend.Name)
			isOwnedByObj = err == nil && m.IsOwnedBy(clusterName, t)
		default:
			return nil, fmt.Errorf("invalid object: %v", t)
		}
		// If the current frontend doesn't belong to the Service/Ingress resource being processed, we skip it.
		if !isOwnedByObj {
			continue
		}
		// Add all reported DNS names to the set of DNS names.
		for _, name := range frontend.DNS {
			dnsNames[name] = true
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
	return res, nil
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
