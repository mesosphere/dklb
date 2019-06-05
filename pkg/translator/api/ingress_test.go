package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mesosphere/dklb/pkg/cluster"
	"github.com/mesosphere/dklb/pkg/constants"
)

func TestGetIngressEdgeLBPoolSpec(t *testing.T) {
	// cluster name really shouldn't be a global
	cluster.Name = "test-cluster"
	tests := []struct {
		description   string
		expectedError error
		ingress       *extsv1beta1.Ingress
		validate      func(t *testing.T, spec *IngressEdgeLBPoolSpec)
	}{
		{
			description:   "should translate to an edgelb pool with defaults",
			expectedError: nil,
			ingress: &extsv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-ingress",
				},
				Spec: extsv1beta1.IngressSpec{
					TLS: []extsv1beta1.IngressTLS{
						{SecretName: "test-secret"},
					},
				},
			},
			validate: func(t *testing.T, spec *IngressEdgeLBPoolSpec) {
				assert.Equal(t, *spec.Frontends.HTTP.Mode, IngressEdgeLBHTTPModeEnabled)
				assert.Equal(t, *spec.Frontends.HTTP.Port, DefaultEdgeLBPoolHTTPPort)
				assert.Equal(t, *spec.Frontends.HTTPS.Port, DefaultEdgeLBPoolHTTPSPort)
			},
		},
		{
			description:   "should translate to an edgelb pool with custom frontend",
			expectedError: nil,
			ingress: &extsv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constants.DklbConfigAnnotationKey: `
  frontends:
    http:
      mode: disabled
      port: 8080
    https:
      port: 8443`,
					},
					Namespace: "test-namespace",
					Name:      "test-ingress",
				},
				Spec: extsv1beta1.IngressSpec{
					TLS: []extsv1beta1.IngressTLS{
						{SecretName: "test-secret"},
					},
				},
			},
			validate: func(t *testing.T, spec *IngressEdgeLBPoolSpec) {
				assert.Equal(t, *spec.Frontends.HTTP.Mode, IngressEdgeLBHTTPModeDisabled)
				assert.Equal(t, *spec.Frontends.HTTP.Port, int32(8080))
				assert.Equal(t, *spec.Frontends.HTTPS.Port, int32(8443))
			},
		},
	}

	for _, test := range tests {
		t.Logf("test case: %s", test.description)

		spec, err := GetIngressEdgeLBPoolSpec(test.ingress)
		assert.Equal(t, test.expectedError, err)
		test.validate(t, spec)
	}
}
