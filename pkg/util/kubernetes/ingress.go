package kubernetes

import (
	extsv1beta1 "k8s.io/api/extensions/v1beta1"

	"github.com/mesosphere/dklb/pkg/constants"
)

// ForEachIngressBackend iterates over Ingress backends defined on the specified Ingress resource, calling "fn" with each Ingress backend object and the associated host and path whenever applicable.
func ForEachIngresBackend(ingress *extsv1beta1.Ingress, fn func(host *string, path *string, backend extsv1beta1.IngressBackend)) {
	if ingress.Spec.Backend != nil {
		// Use nil values for "host" and "path" so that the caller can identify the current Ingress backend as the default one if it needs to.
		fn(nil, nil, *ingress.Spec.Backend)
	}
	for _, rule := range ingress.Spec.Rules {
		// Pin "rule" so we can take its address.
		rule := rule
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				// Pin "path" so we can take its address.
				path := path
				// Use the specified (possibly empty) values for "host" and "path".
				fn(&rule.Host, &path.Path, path.Backend)
			}
		}
	}
}

// IsEdgeLBIngress returns a value indicating whether the specified Ingress resource is meant to be provisioned by EdgeLB.
func IsEdgeLBIngress(ingress *extsv1beta1.Ingress) bool {
	// If the required annotation is not present, return false.
	v, exists := ingress.Annotations[constants.EdgeLBIngressClassAnnotationKey]
	if !exists {
		return false
	}
	// Return whether the value of the annotation matches the expected one.
	return v == constants.EdgeLBIngressClassAnnotationValue
}
