package translator

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/validation"
	"strconv"

	corev1 "k8s.io/api/core/v1"

	"github.com/mesosphere/dklb/pkg/constants"
)

// ServiceTranslationOption groups together options used to "translate" a Service resource into an EdgeLB pool.
type ServiceTranslationOptions struct {
	BaseTranslationOptions
	// EdgeLBPoolPortMap is the mapping between ports defined in the Service resource and the frontend bind ports used by the EdgeLB pool.
	EdgeLBPoolPortMap map[int]int
}

// ComputeServiceTranslationOptions computes the set of options to use for "translating" the specified Service resource into an EdgeLB pool.
func ComputeServiceTranslationOptions(obj *corev1.Service) (*ServiceTranslationOptions, error) {
	// Create an "ServiceTranslationOptions" struct to hold the computed options.
	res := &ServiceTranslationOptions{
		EdgeLBPoolPortMap: make(map[int]int, len(obj.Spec.Ports)),
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
		// Convert the original service port to an "int" for easier manipulation.
		port := int(port.Port)
		// Compute the key of the annotation that must be checked based on the current port.
		key := fmt.Sprintf("%s%d", constants.EdgeLBPoolPortMapKeyPrefix, port)
		if v, exists := obj.Annotations[key]; !exists || v == "" {
			res.EdgeLBPoolPortMap[port] = port
		} else {
			r, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %q as a frontend bind port: %v", v, err)
			}
			if validation.IsValidPortNum(r) != nil {
				return nil, fmt.Errorf("%d is not a valid port number", r)
			}
			res.EdgeLBPoolPortMap[port] = r
		}
	}

	// Return the computed set of options
	return res, nil
}
