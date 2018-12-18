package translator

import (
	"errors"

	corev1 "k8s.io/api/core/v1"

	"github.com/mesosphere/dklb/pkg/edgelb/manager"
)

// ServiceTranslator is the base implementation of ServiceTranslator.
type ServiceTranslator struct {
	// service is the Service resource to be translated.
	service *corev1.Service
	// options is the set of options used to perform translation.
	options ServiceTranslationOptions
	// manager is the instance of the EdgeLB manager to use for managing EdgeLB pools.
	manager manager.EdgeLBManager
}

// NewServiceTranslator returns a service translator that can be used to translate the specified Service resource into an EdgeLB pool.
func NewServiceTranslator(service *corev1.Service, options ServiceTranslationOptions, manager manager.EdgeLBManager) *ServiceTranslator {
	return &ServiceTranslator{
		service: service,
		options: options,
		manager: manager,
	}
}

// Translate performs translation of the associated Service resource into an EdgeLB pool.
func (st *ServiceTranslator) Translate() error {
	return errors.New("not implemented")
}
