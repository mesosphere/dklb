package translator

import (
	"fmt"
	"strconv"

	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/mesosphere/dklb/pkg/constants"
)

// IngressTranslationOptions groups together options used to "translate" an Ingress resource into an EdgeLB pool.
type IngressTranslationOptions struct {
	BaseTranslationOptions
	// EdgeLBPoolPortKey is the port to be used as a frontend bind port by the EdgeLB pool.
	EdgeLBPoolPort int32
}

// ComputeIngressTranslationOptions computes the set of options to use for "translating" the specified Ingress resource into an EdgeLB pool.
// In case options cannot be computed or are invalid, the error message MUST be suitable to be used as the message for a Kubernetes event associated with the resource.
func ComputeIngressTranslationOptions(obj *extsv1beta1.Ingress) (*IngressTranslationOptions, error) {
	// Create an "IngressTranslationOptions" struct to hold the computed options.
	res := &IngressTranslationOptions{}
	var annotations map[string]string

	// If no annotations have been provided, use an empty map as the source of the configuration.
	// Otherwise, use the set of annotations defined on the Ingress resource.
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
	// Parse the port to use as a frontend bind port.
	// TODO (@bcustodio) Split into HTTP/HTTPS port when TLS support is introduced.
	if v, exists := annotations[constants.EdgeLBPoolPortKey]; !exists || v == "" {
		res.EdgeLBPoolPort = DefaultEdgeLBPoolPort
	} else {
		r, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q as the frontend bind port to use: %v", v, err)
		}
		if validation.IsValidPortNum(r) != nil {
			return nil, fmt.Errorf("%d is not a valid port number", r)
		}
		res.EdgeLBPoolPort = int32(r)
	}

	// Return the computed set of options
	return res, nil
}
