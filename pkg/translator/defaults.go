package translator

import (
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/mesosphere/dklb/pkg/constants"
)

const (
	// DefaultEdgeLBPoolCreationStrategy is the strategy to use for creating an EdgeLB pool when a value is not provided.
	DefaultEdgeLBPoolCreationStrategy = constants.EdgeLBPoolCreationStrategyIfNotPresesent
	// DefaultEdgeLBPoolPort is the port to use as the frontend bind port for an EdgeLB pool used to provision an Ingress resource when a value is not provided.
	// TODO (@bcustodio) Split into HTTP/HTTPS port when TLS support is introduced.
	DefaultEdgeLBPoolPort = 80
	// DefaultEdgeLBPoolRole is the role to use for an EdgeLB pool when a value is not provided.
	DefaultEdgeLBPoolRole = constants.EdgeLBRolePublic
	// DefaultEdgeLBPoolSize is the size to use for an EdgeLB pool when a value is not provided.
	DefaultEdgeLBPoolSize = 1
)

const (
	// defaultEdgeLBManagerTimeout is the default timeout used when interacting with the EdgeLB manager.
	defaultEdgeLBManagerTimeout = 10 * time.Second
)

var (
	// DefaultEdgeLBPoolCpus is the amount of CPU to request for an EdgeLB pool when a value is not provided.
	DefaultEdgeLBPoolCpus = resource.MustParse("100m")
	// DefaultEdgeLBPoolMem is the amount of memory to request for an EdgeLB pool when a value is not provided.
	DefaultEdgeLBPoolMem = resource.MustParse("128Mi")
)

var (
	// DefaultEdgeLBPoolNamespace is the name of the namespace where to create EdgeLB pools.
	DefaultEdgeLBPoolNamespace = "dcos-edgelb/pools"
)
