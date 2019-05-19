package kubernetes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/util/kubernetes"
	ingresstestutil "github.com/mesosphere/dklb/test/util/kubernetes/ingress"
)

// TestForEachIngressBackend tests the "ForEachIngressBackend" function.
func TestForEachIngressBackend(t *testing.T) {
	ingress := ingresstestutil.DummyEdgeLBIngressResource("foo", "bar", func(ingress *extsv1beta1.Ingress) {
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
	kubernetes.ForEachIngresBackend(ingress, func(host *string, path *string, backend extsv1beta1.IngressBackend) {
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

func TestIsEdgeLBIngress(t *testing.T) {
	tests := []struct {
		description    string
		expectedResult bool
		ingress        *extsv1beta1.Ingress
	}{
		{
			description: "should detect ingress type dklb",
			ingress: &extsv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constants.EdgeLBIngressClassAnnotationKey: constants.EdgeLBIngressClassAnnotationValue,
					},
				},
			},
			expectedResult: true,
		},
		{
			description:    "should not detect ingress type dklb with missing annotation",
			ingress:        &extsv1beta1.Ingress{},
			expectedResult: false,
		},
		{
			description: "should not detect ingress type dklb with other ingress type",
			ingress: &extsv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constants.EdgeLBIngressClassAnnotationKey: "nginx",
					},
				},
			},
			expectedResult: false,
		},
	}

	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		res := kubernetes.IsEdgeLBIngress(test.ingress)
		assert.Equal(t, test.expectedResult, res)
	}
}
