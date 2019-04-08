package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/mesosphere/dcos-edge-lb/models"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/util/pointers"
)

// BaseEdgeLBPoolSpec contains EdgeLB pool configuration properties that are common to both Service and Ingress resources.
type BaseEdgeLBPoolSpec struct {
	// CloudProviderConfiguration is the raw, JSON-encoded configuration to set on the target EdgeLB pool's ".cloudProvider" field.
	CloudProviderConfiguration *string `yaml:"cloudProviderConfiguration"`
	// CPUs is the amount of CPU to request for the target EdgeLB pool.
	CPUs *float64 `yaml:"cpus"`
	// Memory is the amount of memory to request for the target EdgeLB pool.
	Memory *int32 `yaml:"memory"`
	// Name is the name of the target EdgeLB pool.
	Name *string `yaml:"name"`
	// Network is the name of the DC/OS virtual network where to place the target EdgeLB pool.
	Network *string `yaml:"network"`
	// Role is the role to request for the target EdgeLB pool.
	Role *string `yaml:"role"`
	// Size is the size to request for the target EdgeLB pool.
	Size *int32 `yaml:"size"`
	//  Strategies groups together strategies used to customize the management of the target EdgeLB pool.
	Strategies *EdgeLBPoolManagementStrategies `yaml:"strategies"`
}

// SetDefaults sets default values wherever a value hasn't been specifically provided.
func (o *BaseEdgeLBPoolSpec) setDefaults() {
	// Set defaults for for pure dklb functionality.
	if o.CloudProviderConfiguration == nil {
		o.CloudProviderConfiguration = pointers.NewString("")
	}
	if o.CPUs == nil {
		o.CPUs = &DefaultEdgeLBPoolCpus
	}
	if o.Memory == nil {
		o.Memory = &DefaultEdgeLBPoolMemory
	}
	if o.Name == nil {
		o.Name = pointers.NewString(NewRandomEdgeLBPoolName(""))
	}
	if o.Role == nil {
		o.Role = pointers.NewString(DefaultEdgeLBPoolRole)
	}
	if o.Network == nil && *o.Role == constants.EdgeLBRolePublic {
		o.Network = pointers.NewString(constants.EdgeLBHostNetwork)
	}
	if o.Network == nil && *o.Role != constants.EdgeLBRolePublic {
		o.Network = pointers.NewString(constants.DefaultDCOSVirtualNetworkName)
	}
	if o.Size == nil {
		o.Size = &DefaultEdgeLBPoolSize
	}
	if o.Strategies == nil {
		o.Strategies = &EdgeLBPoolManagementStrategies{
			Creation: &DefaultEdgeLBPoolCreationStrategy,
		}
	}
	// Check whether cloud-provider configuration is being specified, and override the defaults where necessary.
	if *o.CloudProviderConfiguration != "" {
		// If the target EdgeLB pool's name doesn't start with the prefix used for cloud-provider pools, we generate a new name using that prefix.
		if !strings.HasPrefix(*o.Name, constants.EdgeLBCloudProviderPoolNamePrefix) {
			o.Name = pointers.NewString(NewRandomEdgeLBPoolName(constants.EdgeLBCloudProviderPoolNamePrefix))
		}
		// If the target EdgeLB pool's network is not the host network, we override it.
		if *o.Network != constants.EdgeLBHostNetwork {
			o.Network = pointers.NewString(constants.EdgeLBHostNetwork)
		}
	}
}

// Validate checks whether the current object is valid.
func (o *BaseEdgeLBPoolSpec) Validate() error {
	// Set default values where applicable for easier validation.
	o.setDefaults()

	// Make sure that the name of the target EdgeLB pool is valid.
	if !regexp.MustCompile(constants.EdgeLBPoolNameRegex).MatchString(*o.Name) {
		return fmt.Errorf("%q is not a valid edgelb pool name", *o.Name)
	}
	// Validate the CPU request.
	if *o.CPUs < 0 {
		return fmt.Errorf("%f is not a valid cpu request", *o.CPUs)
	}
	// Validate the memory request.
	if *o.Memory < 0 {
		return fmt.Errorf("%d is not a valid memory request", *o.Memory)
	}
	// Validate the size request.
	if *o.Size <= 0 {
		return fmt.Errorf("%d is not a valid size request", *o.Size)
	}
	// Validate the cloud-provider configuration.
	if *o.CloudProviderConfiguration != "" {
		cp := &models.V2CloudProvider{}
		if err := json.Unmarshal([]byte(*o.CloudProviderConfiguration), cp); err != nil {
			return fmt.Errorf("the cloud-provider configuration is not valid: %v", err)
		}
		if !strings.HasPrefix(*o.Name, constants.EdgeLBCloudProviderPoolNamePrefix) {
			return fmt.Errorf("the name of the target edgelb pool must start with the %q prefix", constants.EdgeLBCloudProviderPoolNamePrefix)
		}
		if *o.Network != constants.EdgeLBHostNetwork {
			return fmt.Errorf("cannot join a virtual network when a cloud-provider configuration is provided")
		}
	} else {
		// If the target EdgeLB pool's role is "slave_public" and a non-empty name for the DC/OS virtual network has been specified, we should fail and warn the user.
		if *o.Role == constants.EdgeLBRolePublic && *o.Network != constants.EdgeLBHostNetwork {
			return fmt.Errorf("cannot join a virtual network when the pool's role is %q", *o.Role)
		}
		// If the target EdgeLB pool's role is NOT "slave_public" and no custom name for the DC/OS virtual network has been specified, we should fail and warn the user.
		if *o.Role != constants.EdgeLBRolePublic && *o.Network == constants.EdgeLBHostNetwork {
			return fmt.Errorf("cannot join the host network when the pool's role is %q", *o.Role)
		}
	}
	return nil
}

// ValidateTransition validates the transition between "previous" and the current object.
func (o *BaseEdgeLBPoolSpec) ValidateTransition(previous *BaseEdgeLBPoolSpec) error {
	// If we're transitioning to a cloud-provider configuration, we don't need to perform any additional validations, as a new EdgeLB pool will always be created.
	if *previous.CloudProviderConfiguration == "" && *o.CloudProviderConfiguration != "" {
		return nil
	}
	// Prevent the cloud-provider configuration from being removed.
	if *previous.CloudProviderConfiguration != "" && *o.CloudProviderConfiguration == "" {
		return fmt.Errorf("the cloud-provider configuration cannot be removed")
	}
	// Prevent the name of the EdgeLB pool from changing.
	if *previous.Name != *o.Name {
		return errors.New("the name of the target edgelb pool cannot be changed")
	}
	// Prevent the role of the EdgeLB pool from changing.
	if *previous.Role != *o.Role {
		return errors.New("the role of the target edgelb pool cannot be changed")
	}
	// Prevent the virtual network of the target EdgeLB pool from changing.
	if *previous.Network != *o.Network {
		return errors.New("the virtual network of the target edgelb pool cannot be changed")
	}
	return nil
}

// EdgeLBPoolManagementStrategies groups together strategies used to customize the management of EdgeLB pools.
type EdgeLBPoolManagementStrategies struct {
	Creation *EdgeLBPoolCreationStrategy `yaml:"creation"`
}
