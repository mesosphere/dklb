package api

import (
	"github.com/mesosphere/dklb/pkg/constants"
)

var (
	// DefaultEdgeLBPoolCpus is the amount of CPU to request for an EdgeLB pool when a value is not provided.
	DefaultEdgeLBPoolCpus = float64(0.1)
	// DefaultEdgeLBPoolCreationStrategy is the strategy to use for creating an EdgeLB pool when a value is not provided.
	DefaultEdgeLBPoolCreationStrategy = EdgeLBPoolCreationStrategyIfNotPresent
	// DefaultEdgeLBPoolMemory is the amount of memory to request for an EdgeLB pool when a value is not provided.
	DefaultEdgeLBPoolMemory = int32(128)
	// DefaultEdgeLBPoolHTTPPort is the HTTP port to use as the frontend bind port for an EdgeLB pool used to provision an Ingress resource when a value is not provided.
	DefaultEdgeLBPoolHTTPPort = int32(80)
	// DefaultEdgeLBPoolHTTPSPort is the HTTPS port to use as the frontend bind port for an EdgeLB pool used to provision an Ingress resource when a value is not provided.
	DefaultEdgeLBPoolHTTPSPort = int32(443)
	// DefaultEdgeLBPoolRole is the role to use for an EdgeLB pool when a value is not provided.
	DefaultEdgeLBPoolRole = constants.EdgeLBRolePublic
	// DefaultEdgeLBPoolSize is the size to use for an EdgeLB pool when a value is not provided.
	DefaultEdgeLBPoolSize = int32(1)
)
