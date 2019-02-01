package translator

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/mesosphere/dklb/pkg/constants"
)

// BaseTranslationOptions groups together options used to "translate" an Ingress/Service resource into an EdgeLB pool.
type BaseTranslationOptions struct {
	// CloudLoadBalancerConfigMapName is the name of the configmap specifying cloud load-balancer configuration.
	CloudLoadBalancerConfigMapName *string

	// EdgeLBPoolName is the name of the EdgeLB pool to use for provisioning the Ingress/Service resource.
	EdgeLBPoolName string
	// EdgeLBPoolRole is the role to use for the target EdgeLB pool.
	EdgeLBPoolRole string
	// EdgeLBPoolNetwork is the name of the DC/OS virtual network to use when creating the target EdgeLB pool.
	EdgeLBPoolNetwork string

	// EdgeLBPoolCpus is the amount of CPU to request for the target EdgeLB pool.
	EdgeLBPoolCpus resource.Quantity
	// EdgeLBPoolMem is the amount of memory to request for the target EdgeLB pool.
	EdgeLBPoolMem resource.Quantity
	// EdgeLBPoolSize is the size to request for the target EdgeLB pool.
	EdgeLBPoolSize int

	// EdgeLBPoolCreationStrategy is the strategy to use for provisioning an EdgeLB pool for the Ingress/Service resource.
	EdgeLBPoolCreationStrategy constants.EdgeLBPoolCreationStrategy
	// EdgeLBPoolTranslationPaused indicates whether translation is currently paused for the Ingress/Service resource.
	EdgeLBPoolTranslationPaused bool
}

// ValidateBaseTranslationOptionsUpdate validates the transition between "previousOptions" and "currentOptions".
func ValidateBaseTranslationOptionsUpdate(previousOptions, currentOptions *BaseTranslationOptions) error {
	// If we've been requested to configure a cloud load-balancer for an existing resource, assume the transition to be valid since we override the translation options ourselves.
	if currentOptions.CloudLoadBalancerConfigMapName != nil && previousOptions.CloudLoadBalancerConfigMapName == nil {
		return nil
	}
	// Prevent the annotation specifying the name of the configmap used for configuring the cloud load-balancer from being removed after having been set.
	if currentOptions.CloudLoadBalancerConfigMapName == nil && previousOptions.CloudLoadBalancerConfigMapName != nil {
		return errors.New("the name of the configmap used for configuring the cloud load-balancer cannot be removed")
	}

	// Prevent the name of the EdgeLB pool from changing.
	if currentOptions.EdgeLBPoolName != previousOptions.EdgeLBPoolName {
		return errors.New("the name of the target edgelb pool cannot be changed")
	}
	// Prevent the role of the EdgeLB pool from changing.
	if currentOptions.EdgeLBPoolRole != previousOptions.EdgeLBPoolRole {
		return errors.New("the role of the target edgelb pool cannot be changed")
	}
	// Prevent the CPU request for the EdgeLB pool from changing.
	if currentOptions.EdgeLBPoolCpus != previousOptions.EdgeLBPoolCpus {
		return errors.New("the cpu request for the target edgelb pool cannot be changed")
	}
	// Prevent the memory request for the EdgeLB pool from changing.
	if currentOptions.EdgeLBPoolMem != previousOptions.EdgeLBPoolMem {
		return errors.New("the memory request for the target edgelb pool cannot be changed")
	}
	// Prevent the size of the EdgeLB pool from changing.
	if currentOptions.EdgeLBPoolSize != previousOptions.EdgeLBPoolSize {
		return errors.New("the size of the target edgelb pool cannot be changed")
	}
	// Prevent the virtual network of the target EdgeLB pool from changing.
	if currentOptions.EdgeLBPoolNetwork != previousOptions.EdgeLBPoolNetwork {
		return errors.New("the virtual network of the target edgelb pool cannot be changed")
	}
	return nil
}

// parseBaseTranslationOptions attempts to compute base, common translation options from the specified set of annotations.
// In case options cannot be computed or are invalid, the error message MUST be suitable to be used as the message for a Kubernetes event associated with the resource.
func parseBaseTranslationOptions(clusterName, namespace, name string, annotations map[string]string) (*BaseTranslationOptions, error) {
	// Check whether we've been asked to configure a cloud load-balancer for the current resource.
	// In such a scenario, we override the values of the remaining options with our own defaults in order to ensure the requirements of the cloud load-balancer.
	if v, exists := annotations[constants.CloudLoadBalancerConfigMapNameAnnotationKey]; exists && v != "" {
		return &BaseTranslationOptions{
			CloudLoadBalancerConfigMapName: &v,
			EdgeLBPoolName:                 ComputeEdgeLBPoolName(constants.EdgeLBCloudLoadBalancerPoolNamePrefix, clusterName, namespace, name),
			EdgeLBPoolCpus:                 DefaultEdgeLBPoolCpus,
			EdgeLBPoolMem:                  DefaultEdgeLBPoolMem,
			EdgeLBPoolNetwork:              constants.EdgeLBHostNetwork,
			EdgeLBPoolSize:                 DefaultEdgeLBPoolSize,
			EdgeLBPoolRole:                 constants.EdgeLBRolePrivate,
			EdgeLBPoolCreationStrategy:     constants.EdgeLBPoolCreationStrategyIfNotPresent,
		}, nil
	}

	// At this point we know we haven't been asked to configure a cloud load-balancer.

	// Create a "BaseTranslationOptions" struct to hold the computed options.
	res := &BaseTranslationOptions{
		CloudLoadBalancerConfigMapName: nil,
	}

	// Parse or compute the name of the target EdgeLB pool.
	poolName := annotations[constants.EdgeLBPoolNameAnnotationKey]
	if poolName == "" {
		poolName = ComputeEdgeLBPoolName("", clusterName, namespace, name)
	}
	if !regexp.MustCompile(constants.EdgeLBPoolNameRegex).MatchString(poolName) {
		return nil, fmt.Errorf("%q is not valid as an edgelb pool name", poolName)
	}
	res.EdgeLBPoolName = poolName

	// Parse the role of the target EdgeLB pool.
	if v, exists := annotations[constants.EdgeLBPoolRoleAnnotationKey]; !exists || v == "" {
		res.EdgeLBPoolRole = DefaultEdgeLBPoolRole
	} else {
		res.EdgeLBPoolRole = v
	}

	// Grab the name of the DC/OS virtual network to use when creating the target EdgeLB pool.
	networkName := annotations[constants.EdgeLBPoolNetworkAnnotationKey]
	// If the target EdgeLB pool's role is "slave_public" and a non-empty name for the DC/OS virtual network has been specified, we should fail and warn the user.
	if res.EdgeLBPoolRole == constants.EdgeLBRolePublic && networkName != constants.EdgeLBHostNetwork {
		return nil, fmt.Errorf("cannot join a dcos virtual network when the pool's role is %q", res.EdgeLBPoolRole)
	}
	// If the target EdgeLB pool's role is NOT "slave_public" and no custom name for the DC/OS virtual network has been specified, we should use the default name.
	if res.EdgeLBPoolRole != constants.EdgeLBRolePublic && networkName == constants.EdgeLBHostNetwork {
		networkName = DefaultEdgeLBPoolNetwork
	}
	res.EdgeLBPoolNetwork = networkName

	// Parse the CPU request for the target EdgeLB pool.
	if v, exists := annotations[constants.EdgeLBPoolCpusAnnotationKey]; !exists || v == "" {
		res.EdgeLBPoolCpus = DefaultEdgeLBPoolCpus
	} else {
		r, err := resource.ParseQuantity(v)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q as the amount of cpus to request: %v", v, err)
		}
		res.EdgeLBPoolCpus = r
	}

	// Parse the memory request for the target EdgeLB pool.
	if v, exists := annotations[constants.EdgeLBPoolMemAnnotationKey]; !exists || v == "" {
		res.EdgeLBPoolMem = DefaultEdgeLBPoolMem
	} else {
		r, err := resource.ParseQuantity(v)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q as the amount of memory to request: %v", v, err)
		}
		res.EdgeLBPoolMem = r
	}

	// Parse the size request for the target EdgeLB pool.
	if v, exists := annotations[constants.EdgeLBPoolSizeAnnotationKey]; !exists || v == "" {
		res.EdgeLBPoolSize = DefaultEdgeLBPoolSize
	} else {
		r, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q as the size to request for the edgelb pool: %v", v, err)
		}
		if r <= 0 {
			return nil, fmt.Errorf("%d is not a valid size", r)
		}
		res.EdgeLBPoolSize = r
	}

	// Parse the creation strategy to use for creating the target EdgeLB pool.
	if v, exists := annotations[constants.EdgeLBPoolCreationStrategyAnnotationKey]; !exists || v == "" {
		res.EdgeLBPoolCreationStrategy = DefaultEdgeLBPoolCreationStrategy
	} else {
		switch v {
		case string(constants.EdgeLBPoolCreationStrategyIfNotPresent):
			res.EdgeLBPoolCreationStrategy = constants.EdgeLBPoolCreationStrategyIfNotPresent
		case string(constants.EdgeLBPoolCreationStrategyNever):
			res.EdgeLBPoolCreationStrategy = constants.EdgeLBPoolCreationStrategyNever
		case string(constants.EdgeLBPoolCreationStrategyOnce):
			res.EdgeLBPoolCreationStrategy = constants.EdgeLBPoolCreationStrategyOnce
		default:
			return nil, fmt.Errorf("failed to parse %q as a pool creation strategy", v)
		}
	}

	// Parse whether translation is currently paused.
	if v, exists := annotations[constants.EdgeLBPoolTranslationPaused]; !exists || v == "" {
		res.EdgeLBPoolTranslationPaused = false
	} else {
		p, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q as a boolean value: %v", v, err)
		}
		res.EdgeLBPoolTranslationPaused = p
	}

	// Return the computed set of options.
	return res, nil
}
