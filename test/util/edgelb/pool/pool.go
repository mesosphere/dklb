package pool

import (
	"github.com/mesosphere/dcos-edge-lb/models"
)

// EdgeLBPoolCustomizer represents a function that can be used to customize an EdgeLB pool object.
type EdgeLBPoolCustomizer func(pool *models.V2Pool)

// DummyEdgeLBPool returns a dummy, minimal EdgeLB pool with the specified name.
// If any customization functions are specified, they are run before the resource is returned.
func DummyEdgeLBPool(name string, opts ...EdgeLBPoolCustomizer) *models.V2Pool {
	p := &models.V2Pool{
		Name:    name,
		Haproxy: &models.V2Haproxy{},
	}
	for _, fn := range opts {
		fn(p)
	}
	return p
}
