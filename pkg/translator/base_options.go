package translator

import (
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/mesosphere/dklb/pkg/constants"
)

// BaseTranslationOptions groups together options used to "translate" an Ingress/Service resource into an EdgeLB pool.
type BaseTranslationOptions struct {
	// EdgeLBPoolName is the name of the EdgeLB pool to use for provisioning the Ingress/Service resource.
	EdgeLBPoolName string
	// EdgeLBPoolRole is the role to use for the target EdgeLB pool.
	EdgeLBPoolRole string

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

// parseBaseTranslationOptions attempts to compute base, common translation options from the specified set of annotations.
// In case options cannot be computed or are invalid, the error message MUST be suitable to be used as the message for a Kubernetes event associated with the resource.
func parseBaseTranslationOptions(clusterName, namespace, name string, annotations map[string]string) (*BaseTranslationOptions, error) {
	// Create a "BaseTranslationOptions" struct to hold the computed options.
	res := &BaseTranslationOptions{}

	// Parse or compute the name of the target EdgeLB pool.
	if v, exists := annotations[constants.EdgeLBPoolNameAnnotationKey]; !exists || v == "" {
		res.EdgeLBPoolName = computeEdgeLBPoolName(clusterName, namespace, name)
	} else {
		res.EdgeLBPoolName = v
	}

	// Parse the role of the target EdgeLB pool.
	if v, exists := annotations[constants.EdgeLBPoolRoleAnnotationKey]; !exists || v == "" {
		res.EdgeLBPoolRole = DefaultEdgeLBPoolRole
	} else {
		res.EdgeLBPoolRole = v
	}

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
