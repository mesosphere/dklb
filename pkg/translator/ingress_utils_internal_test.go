package translator

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"

	ingresstestutil "github.com/mesosphere/dklb/test/util/kubernetes/ingress"
)

// TestIngressOwnedEdgeLBObjectMetadata_IsOwnedBy tests the "IsOwnedBy" function.
func TestIngressOwnedEdgeLBObjectMetadata_IsOwnedBy(t *testing.T) {
	tests := []struct {
		description string
		clusterName string
		metadata    ingressOwnedEdgeLBObjectMetadata
		ingress     *extsv1beta1.Ingress
		result      bool
	}{
		{
			description: "cluster name, and ingress namespace and name match",
			clusterName: "dev/kubernetes01",
			metadata: ingressOwnedEdgeLBObjectMetadata{
				ClusterName: "dev/kubernetes01",
				Namespace:   "foo",
				Name:        "bar",
			},
			ingress: ingresstestutil.DummyIngressResource("foo", "bar"),
			result:  true,
		},
		{
			description: "ingress name mismatch",
			clusterName: "dev/kubernetes01",
			metadata: ingressOwnedEdgeLBObjectMetadata{
				ClusterName: "dev/kubernetes01",
				Namespace:   "foo",
				Name:        "bar",
			},
			ingress: ingresstestutil.DummyIngressResource("foo", "not-the-name"),
			result:  false,
		},
		{
			description: "ingress namespace mismatch",
			clusterName: "dev/kubernetes01",
			metadata: ingressOwnedEdgeLBObjectMetadata{
				ClusterName: "dev/kubernetes01",
				Namespace:   "foo",
				Name:        "bar",
			},
			ingress: ingresstestutil.DummyIngressResource("not-the-namespace", "bar"),
			result:  false,
		},
		{
			description: "cluster name mismatch",
			clusterName: "not-the-cluster-name",
			metadata: ingressOwnedEdgeLBObjectMetadata{
				ClusterName: "dev/kubernetes01",
				Namespace:   "foo",
				Name:        "bar",
			},
			ingress: ingresstestutil.DummyIngressResource("foo", "bar"),
			result:  false,
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		assert.Equal(t, test.result, test.metadata.IsOwnedBy(test.clusterName, test.ingress))
	}
}

// TestComputeIngressOwnedEdgeLBObjectMetadata tests the "computeIngressOwnedEdgeLBObjectMetadata" function.
func TestComputeIngressOwnedEdgeLBObjectMetadata(t *testing.T) {
	tests := []struct {
		description string
		name        string
		metadata    *ingressOwnedEdgeLBObjectMetadata
		err         error
	}{
		{
			description: "name doesn't have all required components",
			name:        "foo:backend",
			metadata:    nil,
			err:         errors.New("invalid backend/frontend name for ingress"),
		},
		{
			description: "name has invalid fourth component",
			name:        "dev.kubernetes01:foo:bar:XYZ",
			metadata:    nil,
			err:         errors.New("invalid backend/frontend name for ingress"),
		},
		{
			description: "name is a valid ingress-owned edgelb backend name",
			name:        "dev.kubernetes01:foo:bar:baz:http",
			metadata: &ingressOwnedEdgeLBObjectMetadata{
				ClusterName: "dev/kubernetes01",
				Namespace:   "foo",
				Name:        "bar",
				IngressBackend: &extsv1beta1.IngressBackend{
					ServiceName: "baz",
					ServicePort: intstr.FromString("http"),
				},
			},
		},
		{
			description: "name is a valid ingress-owned edgelb frontend name",
			name:        "dev.kubernetes01:foo:bar",
			metadata: &ingressOwnedEdgeLBObjectMetadata{
				ClusterName: "dev/kubernetes01",
				Namespace:   "foo",
				Name:        "bar",
			},
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		r, err := computeIngressOwnedEdgeLBObjectMetadata(test.name)
		if err != nil {
			assert.Equal(t, test.err, err)
		} else {
			assert.Equal(t, test.metadata, r)
		}
	}
}
