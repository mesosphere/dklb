package translator

import (
	"errors"

	extsv1beta1 "k8s.io/api/extensions/v1beta1"

	"github.com/mesosphere/dklb/pkg/edgelb/manager"
)

// IngressTranslator is the base implementation of IngressTranslator.
type IngressTranslator struct {
	// ingress is the Ingress resource to be translated.
	ingress *extsv1beta1.Ingress
	// options is the set of options used to perform translation.
	options IngressTranslationOptions
	// manager is the instance of the EdgeLB manager to use for managing EdgeLB pools.
	manager manager.EdgeLBManager
}

// NewIngressTranslator returns a service translator that can be used to translate the specified Ingress resource into an EdgeLB pool.
func NewIngressTranslator(service *extsv1beta1.Ingress, options IngressTranslationOptions, manager manager.EdgeLBManager) *IngressTranslator {
	return &IngressTranslator{
		ingress: service,
		options: options,
		manager: manager,
	}
}

// Translate performs translation of the associated Ingress resource into an EdgeLB pool.
func (st *IngressTranslator) Translate() error {
	return errors.New("not implemented")
}
