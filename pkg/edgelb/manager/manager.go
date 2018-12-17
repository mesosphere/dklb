package manager

import (
	"context"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	"github.com/mesosphere/dcos-edge-lb/client"
	edgelboperations "github.com/mesosphere/dcos-edge-lb/client/operations"
)

// EdgeLBManager knows how to manage the configuration of EdgeLB pools.
type EdgeLBManager struct {
	client *client.DcosEdgeLb
}

// EdgeLBManagerOptions groups options that can be used to configure an instance of the EdgeLB Manager.
type EdgeLBManagerOptions struct {
	// BearerToken is the (optional) bearer token to use when performing requests.
	BearerToken string
	// Host is the host at which the EdgeLB API server can be reached.
	Host string
	// InsecureSkipTLSVerify indicates whether to skip verification of the TLS certificate presented by the EdgeLB API server.
	InsecureSkipTLSVerify bool
	// Path is the path at which the EdgeLB API server can be reached.
	Path string
	// Scheme is the scheme to use when communicating with the EdgeLB API server.
	Scheme string
}

// NewEdgeLBManager creates a new instance of EdgeLBManager configured according to the provided options.
func NewEdgeLBManager(opts EdgeLBManagerOptions) *EdgeLBManager {
	var (
		t *httptransport.Runtime
	)

	if !opts.InsecureSkipTLSVerify {
		// Do not skip TLS verification.
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

	return &EdgeLBManager{
		client: client.New(t, strfmt.Default),
	}
}

// GetVersion returns the current version of EdgeLB.
func (m *EdgeLBManager) GetVersion(ctx context.Context) (string, error) {
	r, err := m.client.Operations.Version(edgelboperations.NewVersionParamsWithContext(ctx))
	if err != nil {
		return "", err
	}
	return r.Payload, nil
}
