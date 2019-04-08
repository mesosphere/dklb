package api

import (
	"fmt"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"

	"github.com/mesosphere/dklb/pkg/cluster"
	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/util/strings"
)

const (
	// edgeLBPoolNameComponentSeparator is the string used as to separate the components of an EdgeLB pool's name.
	// "--" is chosen as the value since the name of an EdgeLB pool must match the "^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$" regular expression.
	edgeLBPoolNameComponentSeparator = "--"
	// edgeLBPoolNameMaxLength is the maximum number of characters that may comprise the name of an EdgeLB pool.
	edgeLBPoolNameMaxLength = 63
	// edgeLBPoolNameSufficLength is the length of the random suffix generated for EdgeLB pools.
	edgeLBPoolNameSuffixLength = 5
)

// NewRandomEdgeLBPoolName returns a string meant to be used as the name of an EdgeLB pool.
// The computed name is of the form "[<prefix>--]<cluster-name>--<suffix>", where "<prefix>" is the specified (possibly empty) string and "<suffix>" is a randomly-generated suffix.
// It is guaranteed not to exceed 63 characters.
func NewRandomEdgeLBPoolName(prefix string) string {
	// If the specified prefix is non-empty, append it with the component separator (i.e. "<prefix>--").
	if prefix != "" {
		prefix = prefix + edgeLBPoolNameComponentSeparator
	}
	// Grab a random string to use as the suffix, and prepend it with the component separator (i.e. "--<suffix>").
	suffix := edgeLBPoolNameComponentSeparator + strings.RandomStringWithLength(edgeLBPoolNameSuffixLength)
	// Compute the maximum length of the "<cluster-name>" component.
	maxClusterNameLength := edgeLBPoolNameMaxLength - len(prefix) - len(suffix)
	// Compute a "safe" version of the cluster's name.
	clusterName := strings.ReplaceForwardSlashes(cluster.Name, edgeLBPoolNameComponentSeparator)
	if len(clusterName) > maxClusterNameLength {
		clusterName = clusterName[:edgeLBPoolNameMaxLength]
	}
	// Return the computed name.
	return prefix + clusterName + suffix
}

// GetIngressEdgeLBPoolSpec attempts to parse the contents of the "kubernetes.dcos.io/dklb-config" annotation of the specified Ingress resource as the specification of the target EdgeLB pool.
// Parsing is strict in the sense that any unrecognized fields will originate a parsing error.
// If no value has been provided for the "kubernetes.dcos.io/dklb-config" annotation, a default EdgeLB pool specification object is returned.
func GetIngressEdgeLBPoolSpec(ingress *extsv1beta1.Ingress) (*IngressEdgeLBPoolSpec, error) {
	v, exists := ingress.Annotations[constants.DklbConfigAnnotationKey]
	if !exists || v == "" {
		return NewDefaultIngressEdgeLBPoolSpecForIngress(ingress), nil
	}
	r := &IngressEdgeLBPoolSpec{}
	if err := yaml.UnmarshalStrict([]byte(v), r); err != nil {
		return nil, fmt.Errorf("failed to parse the value of %q as a configuration object: %v", constants.DklbConfigAnnotationKey, err)
	}
	if err := r.Validate(ingress); err != nil {
		return nil, err
	}
	return r, nil
}

// GetServiceEdgeLBPoolSpec attempts to parse the contents of the "kubernetes.dcos.io/dklb-config" annotation of the specified Service resource as the specification of the target EdgeLB pool.
// Parsing is strict in the sense that any unrecognized fields will originate a parsing error.
// If no value has been provided for the "kubernetes.dcos.io/dklb-config" annotation, a default EdgeLB pool specification object is returned.
func GetServiceEdgeLBPoolSpec(service *corev1.Service) (*ServiceEdgeLBPoolSpec, error) {
	v, exists := service.Annotations[constants.DklbConfigAnnotationKey]
	if !exists || v == "" {
		return NewDefaultServiceEdgeLBPoolSpecForService(service), nil
	}
	r := &ServiceEdgeLBPoolSpec{}
	if err := yaml.UnmarshalStrict([]byte(v), r); err != nil {
		return nil, fmt.Errorf("failed to parse the value of %q as a configuration object: %v", constants.DklbConfigAnnotationKey, err)
	}
	if err := r.Validate(service); err != nil {
		return nil, err
	}
	return r, nil
}

// SetIngressEdgeLBPoolSpec updates the provided Ingress resource with the provided EdgeLB pool specification.
func SetIngressEdgeLBPoolSpec(ingress *extsv1beta1.Ingress, obj *IngressEdgeLBPoolSpec) error {
	b, err := yaml.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration object: %v", err)
	}
	if ingress.GetAnnotations() == nil {
		ingress.SetAnnotations(make(map[string]string, 1))
	}
	ingress.GetAnnotations()[constants.DklbConfigAnnotationKey] = string(b)
	return nil
}

// SetServiceEdgeLBPoolSpec updates the provided Service resource with the provided EdgeLB pool specification.
func SetServiceEdgeLBPoolSpec(service *corev1.Service, obj *ServiceEdgeLBPoolSpec) error {
	b, err := yaml.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration object: %v", err)
	}
	if service.GetAnnotations() == nil {
		service.SetAnnotations(make(map[string]string, 1))
	}
	service.GetAnnotations()[constants.DklbConfigAnnotationKey] = string(b)
	return nil
}
