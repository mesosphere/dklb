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

// TestComputeIngressOwnedEdgeLBObjectMetadata tests the "forEachIngressBackend" function.
func TestForEachIngressBackend(t *testing.T) {
	ingress := ingresstestutil.DummyIngressResource("foo", "bar", func(ingress *extsv1beta1.Ingress) {
		ingress.Spec.Backend = &extsv1beta1.IngressBackend{
			ServiceName: "1",
		}
		ingress.Spec.Rules = []extsv1beta1.IngressRule{
			{
				Host: "foo.bar",
				IngressRuleValue: extsv1beta1.IngressRuleValue{
					HTTP: &extsv1beta1.HTTPIngressRuleValue{
						Paths: []extsv1beta1.HTTPIngressPath{
							{
								Path: "/foo",
								Backend: extsv1beta1.IngressBackend{
									ServiceName: "2",
								},
							},
							{
								Path: "/bar",
								Backend: extsv1beta1.IngressBackend{
									ServiceName: "3",
								},
							},
						},
					},
				},
			},
			{
				Host: "bar.baz",
				IngressRuleValue: extsv1beta1.IngressRuleValue{
					HTTP: &extsv1beta1.HTTPIngressRuleValue{
						Paths: []extsv1beta1.HTTPIngressPath{
							{
								Path: "/baz",
								Backend: extsv1beta1.IngressBackend{
									ServiceName: "4",
								},
							},
						},
					},
				},
			},
		}
	})

	// Visit each Ingress backend, adding the corresponding host and path to "visitedHosts" and "visitedPaths", respectively, having the target service's name as the key.
	visitedHosts := make(map[string]*string)
	visitedPaths := make(map[string]*string)
	forEachIngresBackend(ingress, func(host *string, path *string, backend extsv1beta1.IngressBackend) {
		visitedHosts[backend.ServiceName] = host
		visitedPaths[backend.ServiceName] = path
	})

	// Make sure that all Ingress backends have been visited.
	assert.Equal(t, 4, len(visitedHosts))
	assert.Equal(t, 4, len(visitedPaths))

	// Make sure that all Ingress backends have been visited with the correct host.
	assert.Nil(t, visitedHosts["1"])
	assert.Equal(t, "foo.bar", *visitedHosts["2"])
	assert.Equal(t, "foo.bar", *visitedHosts["3"])
	assert.Equal(t, "bar.baz", *visitedHosts["4"])

	// Make sure that all Ingress backends have been visited with the correct path.
	assert.Nil(t, visitedPaths["1"])
	assert.Equal(t, "/foo", *visitedPaths["2"])
	assert.Equal(t, "/bar", *visitedPaths["3"])
	assert.Equal(t, "/baz", *visitedPaths["4"])
}
