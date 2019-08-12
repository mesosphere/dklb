package api

import (
	"context"
	"fmt"
	"time"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"

	"github.com/mesosphere/dklb/pkg/cluster"
	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/errors"
	"github.com/mesosphere/dklb/pkg/util/strings"
)

const (
	// edgeLBPoolNameComponentSeparator is the string used as to separate the components of an EdgeLB pool's name.
	// "--" is chosen as the value since the name of an EdgeLB pool must match the "^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$" regular expression.
	edgeLBPoolNameComponentSeparator = "--"
	// edgeLBPoolNameMaxLength is the maximum number of characters that may comprise the name of an EdgeLB pool.
	edgeLBPoolNameMaxLength = 63
	// edgeLBPoolNameSuffixLength is the length of the random suffix generated for EdgeLB pools.
	edgeLBPoolNameSuffixLength = 5
)

// newRandomEdgeLBPoolName returns a string meant to be used as the name of an EdgeLB pool.
// The computed name is of the form "<EdgeLB group>--[<prefix>--]<cluster-name>--<suffix>", where "<prefix>" is the specified (possibly empty) string and "<suffix>" is a randomly-generated suffix.
// It is guaranteed not to exceed 63 characters.
// It is also guaranteed, to the best of our ability, not to clash with the names of any pre-existing EdgeLB pool.
func newRandomEdgeLBPoolName(prefix string) string {
	// If the specified prefix is non-empty, append it with the component separator (i.e. "<prefix>--").
	if prefix != "" {
		prefix = prefix + edgeLBPoolNameComponentSeparator
	}
	// Grab a random string to use as the suffix, and prepend it with the component separator (i.e. "--<suffix>").
	suffix := edgeLBPoolNameComponentSeparator + strings.RandomStringWithLength(edgeLBPoolNameSuffixLength)
	// Compute the maximum length of the "<cluster-name>" component. We need to account for the EdgeLB pool group name
	// where the pool will created or else the pool will fail to create with the following error:
	//
	// com.mesosphere.sdk.state.ConfigStoreException: Configuration failed
	// validation without any prior target configurationavailable for fallback.
	// Initial launch with invalid configuration? 1 Errors: 1: Field:
	// 'service.name'; Value:
	// 'dcos-edgelb/pools/abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghjik';
	// Message: 'Service name (without slashes) exceeds 63 characters. In order
	// for service DNS to work correctly, the service name (without slashes)
	// must not exceed 63 characters'; Fatal: false (reason: LOGIC_ERROR)
	//
	maxClusterNameLength := edgeLBPoolNameMaxLength - (len(constants.DefaultEdgeLBPoolGroup) + 1) - len(prefix) - len(suffix)

	// Compute a "safe" version of the cluster's name.
	clusterName := strings.ReplaceForwardSlashes(cluster.Name, edgeLBPoolNameComponentSeparator)
	if len(clusterName) > maxClusterNameLength {
		clusterName = clusterName[:maxClusterNameLength]
	}
	// Join the prefix, cluster name and suffix in order to obtain a candidate name.
	candidate := prefix + clusterName + suffix
	// If we haven't been given an instance of the EdgeLB manager to work with, return the candidate name immediately.
	if manager == nil {
		return candidate
	}
	// Otherwise, check whether an EdgeLB pool with the candidate name already exists.
	ctx, fn := context.WithTimeout(context.TODO(), 5*time.Second)
	defer fn()
	_, err := manager.GetPool(ctx, candidate)
	if err != nil && errors.IsNotFound(err) {
		// EdgeLB reports that no EdgeLB pool with the candidate name exists, so we are good to go.
		return candidate
	}
	// At this point we know that either an EdgeLB pool with the candidate name already exists, or that there has been a networking/unknown error while reaching out to EdgeLB.
	// In both cases we call ourselves again, hoping that a better candidate is generated next time around, and that no errors other than "404 NOT FOUND" occur.
	return newRandomEdgeLBPoolName(prefix)
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

// IsIngressTLSEnabled checks if ingress has TLS spec defined.
func IsIngressTLSEnabled(ingress *extsv1beta1.Ingress) bool {
	return len(ingress.Spec.TLS) > 0
}
