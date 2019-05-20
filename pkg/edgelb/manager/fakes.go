package manager

import (
	"context"
	"fmt"

	edgelbmodels "github.com/mesosphere/dcos-edge-lb/models"
)

type fakeEdgeLB struct {
}

func NewFakeEdgeLBManager() EdgeLBManager {
	return &fakeEdgeLB{}
}

func (f *fakeEdgeLB) CreatePool(context.Context, *edgelbmodels.V2Pool) (*edgelbmodels.V2Pool, error) {
	fmt.Println("create pool")
	return nil, nil
}

func (f *fakeEdgeLB) DeletePool(context.Context, string) error {
	fmt.Println("delete pool")
	return nil
}

func (f *fakeEdgeLB) GetPools(ctx context.Context) ([]*edgelbmodels.V2Pool, error) {
	fmt.Println("get pools")
	return nil, nil
}

func (f *fakeEdgeLB) GetPool(context.Context, string) (*edgelbmodels.V2Pool, error) {
	fmt.Println("get pool")
	return nil, nil
}

func (f *fakeEdgeLB) GetPoolMetadata(context.Context, string) (*edgelbmodels.V2PoolMetadata, error) {
	fmt.Println("get pool metadata")
	return nil, nil
}

func (f *fakeEdgeLB) GetVersion(context.Context) (string, error) {
	fmt.Println("get version")
	return "", nil
}

func (f *fakeEdgeLB) PoolGroup() string {
	fmt.Println("pool group")
	return ""
}

func (f *fakeEdgeLB) UpdatePool(context.Context, *edgelbmodels.V2Pool) (*edgelbmodels.V2Pool, error) {
	fmt.Println("update pool")
	return nil, nil
}
