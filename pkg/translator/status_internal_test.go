package translator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mesosphere/dcos-edge-lb/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/api/core/v1"

	"github.com/mesosphere/dklb/pkg/errors"
	"github.com/mesosphere/dklb/pkg/util/pointers"
	edgelbmanagertestutil "github.com/mesosphere/dklb/test/util/edgelb/manager"
	"github.com/mesosphere/dklb/test/util/kubernetes/service"
)

const (
	// dummyELBHostname is a dummy hostname that matches the format used by AWS ELB.
	dummyELBHostname = "dcos-lb-xA3som7gz-Q8ayl-1hYp-DcW-534ec0015f964f71.elb.us-west-2.amazonaws.com"
)

// TestComputeLoadBalancerStatus tests the "computeLoadBalancerStatus" function.
func TestComputeLoadBalancerStatus(t *testing.T) {
	tests := []struct {
		description                string
		metadata                   *models.V2PoolMetadata
		err                        error
		expectedLoadBalancerStatus *v1.LoadBalancerStatus
		expectedErr                error
	}{
		{
			description:                "returns a nil status when metadata cannot be read",
			metadata:                   nil,
			err:                        errors.NotFound(fmt.Errorf("pool not found")),
			expectedLoadBalancerStatus: nil,
			expectedErr:                errors.NotFound(fmt.Errorf("pool not found")),
		},
		{
			description: "returns all reported dns names, private ips and public ips",
			metadata: &models.V2PoolMetadata{
				Name: "foo",
				Frontends: []*models.V2PoolMetadataFrontend{
					{
						Endpoints: []*models.V2PoolMetadataFrontendEndpoint{
							{
								Private: []string{
									"1.2.3.4",
									"5.6.7.8",
								},
								Public: []string{
									"8.7.6.5",
									"4.3.2.1",
								},
							},
						},
						Name: "dev.kubernetes01:foo:bar:8080",
					},
					{
						Endpoints: []*models.V2PoolMetadataFrontendEndpoint{
							{
								Private: []string{
									"7.7.7.7",
								},
								Public: []string{
									"9.9.9.9",
								},
							},
						},
						Name: "must-not-be-reported",
					},
					{
						Endpoints: []*models.V2PoolMetadataFrontendEndpoint{
							{
								Private: []string{
									"1.2.3.4",
									"5.6.7.8",
								},
								Public: []string{
									"8.7.6.5",
									"4.3.2.1",
								},
							},
						},
						Name: "dev.kubernetes01:foo:bar:9090",
					},
				},
				Elb: []*models.V2CloudProviderElb{
					{
						DNS: dummyELBHostname,
						Listeners: []*models.V2CloudProviderAwsElbListener{
							{
								LinkFrontend: pointers.NewString("dev.kubernetes01:foo:bar:8080"),
							},
						},
					},
					{
						DNS: "mustnotbereport.ed",
						Listeners: []*models.V2CloudProviderAwsElbListener{
							{
								LinkFrontend: pointers.NewString("must-not-be-reported"),
							},
						},
					},
					{
						DNS: "bar.com",
						Listeners: []*models.V2CloudProviderAwsElbListener{
							{
								LinkFrontend: pointers.NewString("dev.kubernetes01:foo:bar:9090"),
							},
						},
					},
					{
						DNS: "",
						Listeners: []*models.V2CloudProviderAwsElbListener{
							{
								LinkFrontend: pointers.NewString("dev.kubernetes01:foo:bar:9090"),
							},
						},
					},
				},
			},
			expectedLoadBalancerStatus: &v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						Hostname: "bar.com",
					},
					{
						Hostname: strings.ToLower(dummyELBHostname),
					},
					{
						IP: "1.2.3.4",
					},
					{
						IP: "5.6.7.8",
					},
					{
						IP: "4.3.2.1",
					},
					{
						IP: "8.7.6.5",
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		// Create and customize a mock EdgeLB manager.
		manager := new(edgelbmanagertestutil.MockEdgeLBManager)
		manager.On("GetPoolMetadata", mock.Anything, mock.Anything).Return(test.metadata, test.err)
		// Call "computeLoadBalancerStatus" and make sure the returned object matches the expectations.
		status := computeLoadBalancerStatus(manager, "foo", testClusterName, service.DummyServiceResource("foo", "bar"))
		manager.AssertExpectations(t)
		assert.Equal(t, test.expectedLoadBalancerStatus, status)
	}
}
