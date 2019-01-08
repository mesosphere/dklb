package translator

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"

	servicetestutil "github.com/mesosphere/dklb/test/util/kubernetes/service"
)

// TestBackendNameForServicePort tests the "backendNameForServicePort" function.
func TestBackendNameForServicePort(t *testing.T) {
	tests := []struct {
		description string
		clusterName string
		service     *v1.Service
		port        v1.ServicePort
		backendName string
	}{
		{
			description: "cluster name having slashes",
			clusterName: "dev/kubernetes01",
			service:     servicetestutil.DummyServiceResource("foo", "bar"),
			port: v1.ServicePort{
				Port: 12345,
			},
			backendName: "dev.kubernetes01:foo:bar:12345",
		},
		{
			description: "service name has digits",
			clusterName: "kubernetes-cluster",
			service:     servicetestutil.DummyServiceResource("foobar-baz01", "baz02"),
			port: v1.ServicePort{
				Port: 80,
			},
			backendName: "kubernetes-cluster:foobar-baz01:baz02:80",
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		assert.Equal(t, test.backendName, backendNameForServicePort(test.clusterName, test.service, test.port))
	}
}

// TestFrontendNameForServicePort tests the "frontendNameForServicePort" function.
func TestFrontendNameForServicePort(t *testing.T) {
	tests := []struct {
		description string
		clusterName string
		service     *v1.Service
		port        v1.ServicePort
		frontend    string
	}{
		{
			description: "cluster name has slashes",
			clusterName: "dev/kubernetes01",
			service:     servicetestutil.DummyServiceResource("foo", "bar"),
			port: v1.ServicePort{
				Port: 12345,
			},
			frontend: "dev.kubernetes01:foo:bar:12345",
		},
		{
			description: "service name has digits",
			clusterName: "kubernetes-cluster",
			service:     servicetestutil.DummyServiceResource("foobar-baz01", "baz02"),
			port: v1.ServicePort{
				Port: 80,
			},
			frontend: "kubernetes-cluster:foobar-baz01:baz02:80",
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		assert.Equal(t, test.frontend, frontendNameForServicePort(test.clusterName, test.service, test.port))
	}
}

// TestServiceOwnedEdgeLBObjectMetadata_IsOwnedBy tests the "IsOwnedBy" function.
func TestServiceOwnedEdgeLBObjectMetadata_IsOwnedBy(t *testing.T) {
	tests := []struct {
		description string
		clusterName string
		metadata    serviceOwnedEdgeLBObjectMetadata
		service     *v1.Service
		result      bool
	}{
		{
			description: "cluster name, and service namespace and name match",
			clusterName: "dev/kubernetes01",
			metadata: serviceOwnedEdgeLBObjectMetadata{
				ClusterName: "dev/kubernetes01",
				Namespace:   "foo",
				Name:        "bar",
				ServicePort: 12345,
			},
			service: servicetestutil.DummyServiceResource("foo", "bar"),
			result:  true,
		},
		{
			description: "service name mismatch",
			clusterName: "dev/kubernetes01",
			metadata: serviceOwnedEdgeLBObjectMetadata{
				ClusterName: "dev/kubernetes01",
				Namespace:   "foo",
				Name:        "bar",
				ServicePort: 12345,
			},
			service: servicetestutil.DummyServiceResource("foo", "not-the-name"),
			result:  false,
		},
		{
			description: "service namespace mismatch",
			clusterName: "dev/kubernetes01",
			metadata: serviceOwnedEdgeLBObjectMetadata{
				ClusterName: "dev/kubernetes01",
				Namespace:   "foo",
				Name:        "bar",
				ServicePort: 12345,
			},
			service: servicetestutil.DummyServiceResource("not-the-namespace", "bar"),
			result:  false,
		},
		{
			description: "cluster name mismatch",
			clusterName: "not-the-cluster-name",
			metadata: serviceOwnedEdgeLBObjectMetadata{
				ClusterName: "dev/kubernetes01",
				Namespace:   "foo",
				Name:        "bar",
				ServicePort: 12345,
			},
			service: servicetestutil.DummyServiceResource("foo", "bar"),
			result:  false,
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		assert.Equal(t, test.result, test.metadata.IsOwnedBy(test.clusterName, test.service))
	}
}

// TestComputeServiceOwnedEdgeLBObjectMetadata tests the "computeServiceOwnedEdgeLBObjectMetadata" function.
func TestComputeServiceOwnedEdgeLBObjectMetadata(t *testing.T) {
	tests := []struct {
		description string
		name        string
		metadata    *serviceOwnedEdgeLBObjectMetadata
		err         error
	}{
		{
			description: "name doesn't have all required components",
			name:        "foo:backend",
			metadata:    nil,
			err:         errors.New("invalid backend/frontend name for service"),
		},
		{
			description: "name has invalid fourth component",
			name:        "dev.kubernetes01:foo:bar:XYZ",
			metadata:    nil,
			err:         errors.New("invalid backend/frontend name for service"),
		},
		{
			description: "name is valid",
			name:        "dev.kubernetes01:foo:bar:80",
			metadata: &serviceOwnedEdgeLBObjectMetadata{
				ClusterName: "dev/kubernetes01",
				Namespace:   "foo",
				Name:        "bar",
				ServicePort: int32(80),
			},
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		r, err := computeServiceOwnedEdgeLBObjectMetadata(test.name)
		if err != nil {
			assert.Equal(t, test.err, err)
		} else {
			assert.Equal(t, test.metadata, r)
		}
	}
}

// TestComputeServiceOwnedEdgeLBObjectMetadataRoundTrip tests that the computed names for backends/frontends can be adequately "traced back" to the original Service resource by computeServiceOwnedEdgeLBObjectMetadata.
func TestComputeServiceOwnedEdgeLBObjectMetadataRoundTrip(t *testing.T) {
	for _, fn := range []func(clusterName string, service *v1.Service, port v1.ServicePort) string{backendNameForServicePort, frontendNameForServicePort} {
		service := servicetestutil.DummyServiceResource("foo", "bar")
		port := v1.ServicePort{
			Port: 12345,
		}
		metadata, err := computeServiceOwnedEdgeLBObjectMetadata(fn(testClusterName, service, port))
		assert.NoError(t, err)
		assert.Equal(t, testClusterName, metadata.ClusterName)
		assert.Equal(t, service.Namespace, metadata.Namespace)
		assert.Equal(t, service.Name, metadata.Name)
		assert.Equal(t, port.Port, metadata.ServicePort)
	}
}
