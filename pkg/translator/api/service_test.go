package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mesosphere/dklb/pkg/cluster"
	"github.com/mesosphere/dklb/pkg/constants"
)

func TestGetServiceEdgeLBPoolSpecConstraints(t *testing.T) {
	// cluster name really shouldn't be a global
	cluster.Name = "test-cluster"
	tests := []struct {
		description   string
		expectedError error
		service       *corev1.Service
		validate      func(t *testing.T, spec *ServiceEdgeLBPoolSpec)
	}{
		{
			description: "should translate to an edgelb pool without a constraint",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-service",
				},
			},
			validate: func(t *testing.T, spec *ServiceEdgeLBPoolSpec) {
				var nilString *string
				assert.Equal(t, spec.BaseEdgeLBPoolSpec.Constraints, nilString)
			},
		},
		{
			description: "should translate to an edgelb pool with a constraint",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constants.DklbConfigAnnotationKey: `
constraints: "[[\"hostname\",\"MAX_PER\",\"1\"],[\"@zone\",\"GROUP_BY\",\"3\"]]"
`,
					},
					Namespace: "test-namespace",
					Name:      "test-service",
				},
			},
			validate: func(t *testing.T, spec *ServiceEdgeLBPoolSpec) {
				assert.Equal(t, *spec.Constraints, "[[\"hostname\",\"MAX_PER\",\"1\"],[\"@zone\",\"GROUP_BY\",\"3\"]]")
			},
		},
	}

	for _, test := range tests {
		t.Logf("test case: %s", test.description)

		spec, err := GetServiceEdgeLBPoolSpec(test.service)
		assert.Equal(t, err, nil)
		test.validate(t, spec)
	}
}
