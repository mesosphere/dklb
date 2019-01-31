package manager

import (
	"context"

	"github.com/stretchr/testify/mock"

	edgelbmodels "github.com/mesosphere/dcos-edge-lb/models"
)

// MockEdgeLBManager is a mock implementation of the "EdgeLBManager" interface.
type MockEdgeLBManager struct {
	mock.Mock
}

// CreatePool creates the specified EdgeLB pool in the EdgeLB API server.
func (m *MockEdgeLBManager) CreatePool(ctx context.Context, pool *edgelbmodels.V2Pool) (*edgelbmodels.V2Pool, error) {
	args := m.Called(ctx, pool)
	return args.Get(0).(*edgelbmodels.V2Pool), args.Error(1)
}

// DeletePool deletes the EdgeLB pool with the specified name.
func (m *MockEdgeLBManager) DeletePool(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

// GetPools returns the list of EdgeLB pools known to the EdgeLB API server.
func (m *MockEdgeLBManager) GetPools(ctx context.Context) ([]*edgelbmodels.V2Pool, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*edgelbmodels.V2Pool), args.Error(1)
}

// GetPool returns the EdgeLB pool with the specified name.
func (m *MockEdgeLBManager) GetPool(ctx context.Context, name string) (*edgelbmodels.V2Pool, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*edgelbmodels.V2Pool), args.Error(1)
}

// GetVersion returns the current version of EdgeLB.
func (m *MockEdgeLBManager) GetVersion(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(0)
}

// PoolGroup returns the DC/OS service group in which to create EdgeLB pools.
func (m *MockEdgeLBManager) PoolGroup() string {
	args := m.Called()
	return args.String(0)
}

// UpdatePool updates the specified EdgeLB pool in the EdgeLB API server.
func (m *MockEdgeLBManager) UpdatePool(ctx context.Context, pool *edgelbmodels.V2Pool) (*edgelbmodels.V2Pool, error) {
	args := m.Called(ctx)
	return args.Get(0).(*edgelbmodels.V2Pool), args.Error(1)
}
