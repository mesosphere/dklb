package translator

import (
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/mesosphere/dklb/pkg/constants"
)

// ServiceTranslationOptions groups together options used to "translate" a Service resource into an EdgeLB pool.
type ServiceTranslationOptions struct {
	BaseTranslationOptions
	// EdgeLBPoolPortMap is the mapping between ports defined in the Service resource and the frontend bind ports used by the EdgeLB pool.
	EdgeLBPoolPortMap map[int32]int32
}

// ComputeServiceTranslationOptions computes the set of options to use for "translating" the specified Service resource into an EdgeLB pool.
// In case options cannot be computed or are invalid, the error message MUST be suitable to be used as the message for a Kubernetes event associated with the resource.
func ComputeServiceTranslationOptions(clusterName string, obj *corev1.Service) (*ServiceTranslationOptions, error) {
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
	b, err := parseBaseTranslationOptions(clusterName, obj.Namespace, obj.Name, annotations)
	if err != nil {
		return nil, err
	}
	res.BaseTranslationOptions = *b
	// Parse any port mappings that may have been provided.
	// If no mapping for a port has been specified, the original service port is used.
	// If a duplicate mapping is detected, an error is returned.
	// bindPorts is a mapping between frontend bind ports (i.e. values of "kubernetes.dcos.io/edgelb-pool-portmap.X" annotations) and the names of service ports.
	bindPorts := make(map[int]string, len(obj.Spec.Ports))
	for _, port := range obj.Spec.Ports {
		// Fail immediately if the protocol is not TCP (or the empty string, meaning TCP as well).
		if port.Protocol != "" && port.Protocol != corev1.ProtocolTCP {
			return nil, fmt.Errorf("protocol %q is not supported", port.Protocol)
		}
		// Compute the key of the annotation that must be checked based on the current port.
		key := fmt.Sprintf("%s%d", constants.EdgeLBPoolPortMapKeyPrefix, port.Port)
		if v, exists := obj.Annotations[key]; !exists || v == "" {
			res.EdgeLBPoolPortMap[port.Port] = port.Port
		} else {
			// Parse the annotation's value into an integer
			r, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %q as a frontend bind port: %v", v, err)
			}
			// Check whether the resulting integer is a valid port number.
			if validation.IsValidPortNum(r) != nil {
				return nil, fmt.Errorf("%d is not a valid port number", r)
			}
			// Check whether the port is already being used in the current Service resource.
			if portName, exists := bindPorts[r]; exists {
				return nil, fmt.Errorf("port %d is mapped to both %q and %q service ports", r, portName, port.Name)
			}
			// Mark the current port as being used, and add it to the set of port mappings.
			bindPorts[r] = port.Name
			res.EdgeLBPoolPortMap[port.Port] = int32(r)
		}
	}

	// Return the computed set of options
	return res, nil
}

// ValidateServiceTranslationOptionsUpdate validates the transition between "previousOptions" and "currentOptions".
func ValidateServiceTranslationOptionsUpdate(previousOptions, currentOptions *ServiceTranslationOptions) error {
	return ValidateBaseTranslationOptionsUpdate(&previousOptions.BaseTranslationOptions, &currentOptions.BaseTranslationOptions)
}
