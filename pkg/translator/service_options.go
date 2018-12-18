package translator

import (
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/mesosphere/dklb/pkg/constants"
)

// ServiceTranslationOption groups together options used to "translate" a Service resource into an EdgeLB pool.
type ServiceTranslationOptions struct {
	BaseTranslationOptions
	// EdgeLBPoolPortMap is the mapping between ports defined in the Service resource and the frontend bind ports used by the EdgeLB pool.
	EdgeLBPoolPortMap map[int32]int32
}

// ComputeServiceTranslationOptions computes the set of options to use for "translating" the specified Service resource into an EdgeLB pool.
// In case options cannot be computed or are invalid, the error message MUST be suitable to be used as the message for a Kubernetes event associated with the resource.
func ComputeServiceTranslationOptions(obj *corev1.Service) (*ServiceTranslationOptions, error) {
	// Create an "ServiceTranslationOptions" struct to hold the computed options.
	res := &ServiceTranslationOptions{
		EdgeLBPoolPortMap: make(map[int32]int32, len(obj.Spec.Ports)),
	}
	var annotations map[string]string

	// If no annotations have been provided, use an empty map as the source of the configuration.
	// Otherwise, use the set of annotations defined on the Service resource,
	if obj.Annotations == nil {
		annotations = make(map[string]string)
	} else {
		annotations = obj.Annotations
	}

	// Parse base translation options.
	b, err := parseBaseTranslationOptions(annotations)
	if err != nil {
		return nil, err
	}
	res.BaseTranslationOptions = *b
	// Parse any port mappings that may have been provided.
	// If no mapping for a port has been specified, the original service port is used.
	for _, port := range obj.Spec.Ports {
		// Compute the key of the annotation that must be checked based on the current port.
		key := fmt.Sprintf("%s%d", constants.EdgeLBPoolPortMapKeyPrefix, port.Port)
		if v, exists := obj.Annotations[key]; !exists || v == "" {
			res.EdgeLBPoolPortMap[port.Port] = port.Port
		} else {
			r, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %q as a frontend bind port: %v", v, err)
			}
			if validation.IsValidPortNum(r) != nil {
				return nil, fmt.Errorf("%d is not a valid port number", r)
			}
			res.EdgeLBPoolPortMap[port.Port] = int32(r)
		}
	}

	// Return the computed set of options
	return res, nil
}
