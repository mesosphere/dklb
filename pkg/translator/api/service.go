package api

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// ServiceEdgeLBPoolFrontendSpec contains the specification of a single EdgeLB frontend associated with a given Service resource.
type ServiceEdgeLBPoolFrontendSpec struct {
	// Port is the frontend bind port to use when exposing the current service port.
	Port *int32 `yaml:"port"`
	// ServicePort is the current service port.
	ServicePort int32 `yaml:"servicePort"`
}

// ServiceEdgeLBPoolSpec contains the specification of the target EdgeLB pool for a given Service resource.
type ServiceEdgeLBPoolSpec struct {
	BaseEdgeLBPoolSpec `yaml:",inline"`
	// Frontends contains the specification of the EdgeLB frontends associated with the Service resource.
	Frontends []ServiceEdgeLBPoolFrontendSpec `yaml:"frontends"`
}

// NewDefaultServiceEdgeLBPoolSpecForService returns a new EdgeLB pool specification for the provided Service resource that uses default values.
func NewDefaultServiceEdgeLBPoolSpecForService(service *corev1.Service) *ServiceEdgeLBPoolSpec {
	r := &ServiceEdgeLBPoolSpec{}
	r.SetDefaults(service)
	return r
}

// SetDefaults sets default values whenever a value hasn't been specifically provided.
func (o *ServiceEdgeLBPoolSpec) SetDefaults(service *corev1.Service) {
	// Set defaults on the base object.
	o.BaseEdgeLBPoolSpec.setDefaults()

	// Make sure that there is a frontend per service port defined on the Service resource.
	// By default, the frontend port is taken to be the same as service port.
	// If a custom frontend port is specified for a given service port, that custom frontend port is used instead.
	// During this process, frontends that don't correspond to any port defined on the Service resource are trimmed.
	frontends := make([]ServiceEdgeLBPoolFrontendSpec, 0, len(service.Spec.Ports))
	for _, port := range service.Spec.Ports {
		frontendPort := port.Port
		for _, frontendSpec := range o.Frontends {
			if frontendSpec.ServicePort == port.Port && frontendSpec.Port != nil {
				frontendPort = *frontendSpec.Port
				break
			}
		}
		frontends = append(frontends, ServiceEdgeLBPoolFrontendSpec{Port: &frontendPort, ServicePort: port.Port})
	}
	o.Frontends = frontends
}

// Validate checks whether the current object is valid.
func (o *ServiceEdgeLBPoolSpec) Validate(svc *corev1.Service) error {
	// Set default values where applicable for easier validation.
	o.SetDefaults(svc)

	// Validate the base spec.
	if err := o.BaseEdgeLBPoolSpec.Validate(); err != nil {
		return err
	}

	// visitedServicePorts contains the set of service ports that have already been visited.
	// It is used to prevent duplicate mappings.
	visitedServicePorts := make(map[int32]bool)
	// visitedFrontendPorts contains the set of frontend ports that have already been visited.
	// It is used to prevent duplicate mappings.
	visitedFrontendPorts := make(map[int32]bool)
	// Iterate over the set of frontends, validating each service and frontend port.
	for _, fe := range o.Frontends {
		// Make sure that the current service port is valid.
		if validation.IsValidPortNum(int(fe.ServicePort)) != nil {
			return fmt.Errorf("%d is not a valid port number", fe.ServicePort)
		}
		// Make sure that the current service port has not already been specified.
		if _, exists := visitedServicePorts[fe.ServicePort]; exists {
			return fmt.Errorf("service port %d has been specified twice", fe.ServicePort)
		}
		// Mark the current service port as having been visited.
		visitedServicePorts[fe.ServicePort] = true
		// Make sure that the current frontend port is valid.
		if validation.IsValidPortNum(int(*fe.Port)) != nil {
			return fmt.Errorf("%d is not a valid port number", *fe.Port)
		}
		// Make sure that the current frontend port has not already been specified.
		if _, exists := visitedFrontendPorts[*fe.Port]; exists {
			return fmt.Errorf("frontend port %d has been specified twice", *fe.Port)
		}
		// Mark the current frontend port as having been visited.
		visitedFrontendPorts[*fe.Port] = true
	}
	return nil
}

// ValidateTransition validates the transition between "previous" and the current object.
func (o *ServiceEdgeLBPoolSpec) ValidateTransition(previous *ServiceEdgeLBPoolSpec) error {
	return o.BaseEdgeLBPoolSpec.ValidateTransition(&previous.BaseEdgeLBPoolSpec)
}
