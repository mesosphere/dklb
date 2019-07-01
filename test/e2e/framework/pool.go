package framework

import (
	"context"

	"github.com/mesosphere/dcos-edge-lb/pkg/apis/models"
	. "github.com/onsi/gomega" // nolint:golint

	dklberrors "github.com/mesosphere/dklb/pkg/errors"
	"github.com/mesosphere/dklb/pkg/util/retry"
)

// DeleteEdgeLBPool deletes the specified EdgeLB pool and waits for the EdgeLB API server to stop reporting it as existing.
func (f *Framework) DeleteEdgeLBPool(pool *models.V2Pool) {
	// Delete the EdgeLB pool.
	ctx, fn := context.WithTimeout(context.Background(), DefaultEdgeLBOperationTimeout)
	defer fn()
	err := f.EdgeLBManager.DeletePool(ctx, pool.Name)
	Expect(err).NotTo(HaveOccurred(), "failed to delete edgelb pool %q", pool.Name)
	f.WaitEdgeLBPoolDelete(pool)
}

// WaitEdgeLBPoolDelete waits for the EdgeLB API server to stop reporting the
// EdgeLB pool as existing.
func (f *Framework) WaitEdgeLBPoolDelete(pool *models.V2Pool) {
	err := retry.WithTimeout(DefaultRetryTimeout, DefaultRetryInterval, func() (b bool, e error) {
		ctx, cancel := context.WithTimeout(context.Background(), DefaultEdgeLBOperationTimeout)
		defer cancel()
		_, err := f.EdgeLBManager.GetPool(ctx, pool.Name)
		return dklberrors.IsNotFound(err), nil
	})
	Expect(err).NotTo(HaveOccurred(), "failed to wait for edgelb pool %q to be deleted", pool.Name)
}
