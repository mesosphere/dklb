package api

import (
	"fmt"

	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// IngressEdgeLBPoolHTTPFrontendSpec contains the specification of the HTTP EdgeLB frontend associated with a given Ingress resource.
type IngressEdgeLBPoolHTTPFrontendSpec struct {
	// Port is the port to use as the frontend bind port for HTTP traffic.
	Port *int32 `yaml:"port"`
}

// IngressEdgeLBPoolFrontendsSpec contains the specification of the EdgeLB frontends associated with a given Ingress resource.
type IngressEdgeLBPoolFrontendsSpec struct {
	// HTTP contains the specification of the HTTP EdgeLB frontend associated with the Ingress resource.
	HTTP *IngressEdgeLBPoolHTTPFrontendSpec `yaml:"http"`
}

// IngressEdgeLBPoolSpec contains the specification of the target EdgeLB pool for a given Ingress resource.
type IngressEdgeLBPoolSpec struct {
	BaseEdgeLBPoolSpec `yaml:",inline"`
	// Frontends contains the specification of the EdgeLB frontends associated with the Ingress resource.
	Frontends *IngressEdgeLBPoolFrontendsSpec `yaml:"frontends"`
}

// NewDefaultIngressEdgeLBPoolSpecForIngress returns a new EdgeLB pool specification for the provided Ingress resource that uses default values.
func NewDefaultIngressEdgeLBPoolSpecForIngress(ingress *extsv1beta1.Ingress) *IngressEdgeLBPoolSpec {
	r := &IngressEdgeLBPoolSpec{}
	r.SetDefaults(ingress)
	return r
}

// SetDefaults sets default values whenever a value hasn't been specifically provided.
func (o *IngressEdgeLBPoolSpec) SetDefaults(_ *extsv1beta1.Ingress) {
	// Set defaults on the base object.
	o.BaseEdgeLBPoolSpec.setDefaults()

	// Set defaults everywhere else.
	if o.Frontends == nil {
		o.Frontends = &IngressEdgeLBPoolFrontendsSpec{}
	}
	if o.Frontends.HTTP == nil {
		o.Frontends.HTTP = &IngressEdgeLBPoolHTTPFrontendSpec{}
	}
	if o.Frontends.HTTP.Port == nil {
		o.Frontends.HTTP.Port = &DefaultEdgeLBPoolPort
	}
}

// Validate checks whether the current object is valid.
func (o *IngressEdgeLBPoolSpec) Validate(obj *extsv1beta1.Ingress) error {
	// Set default values where applicable for easier validation.
	o.SetDefaults(obj)

	// Validate the base spec.
	if err := o.BaseEdgeLBPoolSpec.Validate(); err != nil {
		return err
	}
	// Validate that the HTTP port is valid.
	if err := validation.IsValidPortNum(int(*o.Frontends.HTTP.Port)); err != nil {
		return fmt.Errorf("%d is not a valid port number", *o.Frontends.HTTP.Port)
	}
	return nil
}

// ValidateTransition validates the transition between "previous" and the current object.
func (o *IngressEdgeLBPoolSpec) ValidateTransition(previous *IngressEdgeLBPoolSpec) error {
	return o.BaseEdgeLBPoolSpec.ValidateTransition(&previous.BaseEdgeLBPoolSpec)
}
