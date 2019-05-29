package manager

import (
	"context"
	"fmt"
	"strings"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	edgelbclient "github.com/mesosphere/dcos-edge-lb/pkg/apis/client"
	edgelboperations "github.com/mesosphere/dcos-edge-lb/pkg/apis/client/operations"
	edgelbmodels "github.com/mesosphere/dcos-edge-lb/pkg/apis/models"

	"github.com/mesosphere/dklb/pkg/errors"
)

// EdgeLBManagerOptions groups options that can be used to configure an instance of the EdgeLB Manager.
type EdgeLBManagerOptions struct {
	// BearerToken is the (optional) bearer token to use when communicating with the EdgeLB API server.
	BearerToken string
	// Host is the host at which the EdgeLB API server can be reached.
	Host string
	// InsecureSkipTLSVerify indicates whether to skip verification of the TLS certificate presented by the EdgeLB API server.
	InsecureSkipTLSVerify bool
	// Path is the path at which the EdgeLB API server can be reached.
	Path string
	// PoolGroup is the DC/OS service group in which to create EdgeLB pools.
	PoolGroup string
	// Scheme is the scheme to use when communicating with the EdgeLB API server.
	Scheme string
}

// EdgeLBManager knows how to manage the configuration of EdgeLB pools.
type EdgeLBManager interface {
	// CreatePool creates the specified EdgeLB pool in the EdgeLB API server.
	CreatePool(context.Context, *edgelbmodels.V2Pool) (*edgelbmodels.V2Pool, error)
	// DeletePool deletes the EdgeLB pool with the specified name.
	DeletePool(context.Context, string) error
	// GetPools returns the list of EdgeLB pools known to the EdgeLB API server.
	GetPools(ctx context.Context) ([]*edgelbmodels.V2Pool, error)
	// GetPool returns the EdgeLB pool with the specified name.
	GetPool(context.Context, string) (*edgelbmodels.V2Pool, error)
	// GetPoolMetadata returns the metadata associated with the specified EdgeLB pool
	GetPoolMetadata(context.Context, string) (*edgelbmodels.V2PoolMetadata, error)
	// GetVersion returns the current version of EdgeLB.
	GetVersion(context.Context) (string, error)
	// PoolGroup returns the DC/OS service group in which to create EdgeLB pools.
	PoolGroup() string
	// UpdatePool updates the specified EdgeLB pool in the EdgeLB API server.
	UpdatePool(context.Context, *edgelbmodels.V2Pool) (*edgelbmodels.V2Pool, error)
}

// edgeLBManager is the main implementation of the EdgeLB manager.
// TODO (@bcustodio) Figure out a way to test this.
type edgeLBManager struct {
	// client is a client for the EdgeLB API server.
	client *edgelbclient.DcosEdgeLb
	// poolGroup is the DC/OS service group in which to create EdgeLB pools.
	poolGroup string
}

// NewEdgeLBManager creates a new instance of EdgeLBManager configured according to the provided options.
func NewEdgeLBManager(opts EdgeLBManagerOptions) (EdgeLBManager, error) {
	var (
		t *httptransport.Runtime
	)

	// Trim the "http://" and/or "https://" prefixes from the host if they exist.
	if strings.HasPrefix(opts.Host, "http://") {
		opts.Host = strings.TrimPrefix(opts.Host, "http://")
	}
	if strings.HasPrefix(opts.Host, "https://") {
		opts.Host = strings.TrimPrefix(opts.Host, "https://")
	}

	// Configure the transport.
	if !opts.InsecureSkipTLSVerify {
		// Use the default HTTP client, which does not skip TLS verification.
		t = httptransport.New(opts.Host, opts.Path, []string{opts.Scheme})
	} else {
		// Create an HTTP client that skips TLS verification.
		c, err := httptransport.TLSClient(httptransport.TLSClientOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			panic(err)
		}
		t = httptransport.NewWithClient(opts.Host, opts.Path, []string{opts.Scheme}, c)
	}
	if opts.BearerToken != "" {
		// Use the specified bearer token for authentication.
		t.DefaultAuthentication = httptransport.BearerToken(opts.BearerToken)
	}

	// Return a new instance of "edgeLBManager" that uses the specified transport.
	return &edgeLBManager{
		client:    edgelbclient.New(t, strfmt.Default),
		poolGroup: opts.PoolGroup,
	}, nil
}

// CreatePool creates the specified EdgeLB pool in the EdgeLB API server.
func (m *edgeLBManager) CreatePool(ctx context.Context, pool *edgelbmodels.V2Pool) (*edgelbmodels.V2Pool, error) {
	p := &edgelboperations.V2CreatePoolParams{
		Context: ctx,
		Pool:    pool,
	}
	r, err := m.client.Operations.V2CreatePool(p)
	if err == nil {
		return r.Payload, nil
	}
	return nil, errors.Unknown(err)
}

// DeletePool deletes the EdgeLB pool with the specified name.
func (m *edgeLBManager) DeletePool(ctx context.Context, name string) error {
	p := &edgelboperations.V2DeletePoolParams{
		Context: ctx,
		Name:    name,
	}
	_, err := m.client.Operations.V2DeletePool(p)
	if err == nil {
		return nil
	}
	if err, ok := err.(*edgelboperations.V2DeletePoolDefault); ok && err.Code() == 404 {
		return errors.NotFound(err)
	}
	return errors.Unknown(err)
}

// GetPools returns the list of EdgeLB pools known to the EdgeLB API server.
func (m *edgeLBManager) GetPools(ctx context.Context) ([]*edgelbmodels.V2Pool, error) {
	p := &edgelboperations.V2GetPoolsParams{
		Context: ctx,
	}
	r, err := m.client.Operations.V2GetPools(p)
	if err != nil {
		return nil, errors.Unknown(err)
	}
	return r.Payload, nil
}

// GetPool returns the EdgeLB pool with the specified name.
func (m *edgeLBManager) GetPool(ctx context.Context, name string) (*edgelbmodels.V2Pool, error) {
	p := &edgelboperations.V2GetPoolParams{
		Context: ctx,
		Name:    name,
	}
	r, err := m.client.Operations.V2GetPool(p)
	if err == nil {
		return r.Payload, nil
	}
	if err, ok := err.(*edgelboperations.V2GetPoolDefault); ok && err.Code() == 404 {
		return nil, errors.NotFound(err)
	}
	return nil, errors.Unknown(err)
}

// GetPoolMetadata returns the metadata associated with the specified EdgeLB pool
func (m *edgeLBManager) GetPoolMetadata(ctx context.Context, name string) (*edgelbmodels.V2PoolMetadata, error) {
	p := &edgelboperations.V2GetPoolMetadataParams{
		Context: ctx,
		Name:    name,
	}
	r, err := m.client.Operations.V2GetPoolMetadata(p)
	if err == nil {
		return r.Payload, nil
	}
	switch err.(type) {
	case *edgelboperations.V2GetPoolMetadataGatewayTimeout:
		// The EdgeLB pool's metadata isn't available yet.
		return nil, errors.NotFound(fmt.Errorf("the edgelb pool's metadata isn't available yet"))
	case *edgelboperations.V2GetPoolMetadataNotFound:
		// The EdgeLB pool was not found.
		return nil, errors.NotFound(fmt.Errorf("edgelb pool not found"))
	default:
		// We've faced an unknown error (which includes the endpoint not being available in the current version of EdgeLB).
		return nil, errors.Unknown(fmt.Errorf("failed to read pool metadata: %v", err))
	}
}

// GetVersion returns the current version of EdgeLB.
func (m *edgeLBManager) GetVersion(ctx context.Context) (string, error) {
	r, err := m.client.Operations.Version(edgelboperations.NewVersionParamsWithContext(ctx))
	if err != nil {
		return "", errors.Unknown(err)
	}
	return r.Payload, nil
}

// PoolGroup returns the DC/OS service group in which to create EdgeLB pools.
func (m *edgeLBManager) PoolGroup() string {
	return m.poolGroup
}

// UpdatePool updates the specified EdgeLB pool in the EdgeLB API server.
func (m *edgeLBManager) UpdatePool(ctx context.Context, pool *edgelbmodels.V2Pool) (*edgelbmodels.V2Pool, error) {
	p := &edgelboperations.V2UpdatePoolParams{
		Context: ctx,
		Name:    pool.Name,
		Pool:    pool,
	}
	r, err := m.client.Operations.V2UpdatePool(p)
	if err == nil {
		return r.Payload, nil
	}
	if err, ok := err.(*edgelboperations.V2UpdatePoolDefault); ok && err.Code() == 404 {
		return nil, errors.NotFound(err)
	}
	return nil, errors.Unknown(err)
}
