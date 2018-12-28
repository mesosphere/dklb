package translator_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/translator"
	ingresstestutil "github.com/mesosphere/dklb/test/util/kubernetes/ingress"
)

// TestComputeIngressTranslationOptions tests parsing of annotations defined in an Ingress resource.
func TestComputeIngressTranslationOptions(t *testing.T) {
	tests := []struct {
		description string
		annotations map[string]string
		options     *translator.IngressTranslationOptions
		error       error
	}{
		// Test computing options for an Ingress resource without the required annotations.
		// Make sure an error is returned.
		{
			description: "compute options for an Ingress resource without the required annotations",
			annotations: map[string]string{},
			options:     nil,
			error:       fmt.Errorf("required annotation %q has not been provided", constants.EdgeLBPoolNameAnnotationKey),
		},
		// Test computing options for an Ingress resource with just the required annotations.
		// Make sure the name of the EdgeLB pool is captured as expected, and that the default values are used everywhere else.
		{
			description: "compute options for an Ingress resource with just the required annotations",
			annotations: map[string]string{
				constants.EdgeLBPoolNameAnnotationKey: "foo",
			},
			options: &translator.IngressTranslationOptions{
				BaseTranslationOptions: translator.BaseTranslationOptions{
					EdgeLBPoolName:             "foo",
					EdgeLBPoolRole:             translator.DefaultEdgeLBPoolRole,
					EdgeLBPoolCpus:             translator.DefaultEdgeLBPoolCpus,
					EdgeLBPoolMem:              translator.DefaultEdgeLBPoolMem,
					EdgeLBPoolSize:             translator.DefaultEdgeLBPoolSize,
					EdgeLBPoolCreationStrategy: translator.DefaultEdgeLBPoolCreationStrategy,
				},
				EdgeLBPoolPort: translator.DefaultEdgeLBPoolPort,
			},
			error: nil,
		},
		// Test computing options for an Ingress resource defining a custom frontend bind port.
		// Make sure the name of the EdgeLB pool and the frontend bind port are captured as expected, and that the default values are used everywhere else.
		{
			description: "compute options for an Ingress resource defining a custom frontend bind port",
			annotations: map[string]string{
				constants.EdgeLBPoolNameAnnotationKey: "foo",
				constants.EdgeLBPoolPortKey:           "14708",
			},
			options: &translator.IngressTranslationOptions{
				BaseTranslationOptions: translator.BaseTranslationOptions{
					EdgeLBPoolName:             "foo",
					EdgeLBPoolRole:             translator.DefaultEdgeLBPoolRole,
					EdgeLBPoolCpus:             translator.DefaultEdgeLBPoolCpus,
					EdgeLBPoolMem:              translator.DefaultEdgeLBPoolMem,
					EdgeLBPoolSize:             translator.DefaultEdgeLBPoolSize,
					EdgeLBPoolCreationStrategy: translator.DefaultEdgeLBPoolCreationStrategy,
				},
				EdgeLBPoolPort: 14708,
			},
			error: nil,
		},
		// Test computing options for an Ingress resource defining an invalid value for the frontend bind port.
		// Make sure an error is returned.
		{
			description: "compute options for an Ingress resource defining an invalid value for the frontend bind port",
			annotations: map[string]string{
				constants.EdgeLBPoolNameAnnotationKey: "foo",
				constants.EdgeLBPoolPortKey:           "74511",
			},
			options: nil,
			error:   fmt.Errorf("%d is not a valid port number", 74511),
		},
		// Test computing options for an Ingress resource defining custom values for all the options.
		// Make sure that all values are adequately captured.
		{
			description: "compute options for an Ingress resource defining custom values for all the options",
			annotations: map[string]string{
				constants.EdgeLBPoolNameAnnotationKey:             "foo",
				constants.EdgeLBPoolRoleAnnotationKey:             "custom_role",
				constants.EdgeLBPoolCpusAnnotationKey:             "250m",
				constants.EdgeLBPoolMemAnnotationKey:              "2Gi",
				constants.EdgeLBPoolSizeAnnotationKey:             "3",
				constants.EdgeLBPoolCreationStrategyAnnotationKey: string(constants.EdgeLBPoolCreationStrategyOnce),
				constants.EdgeLBPoolPortKey:                       "14708",
				constants.EdgeLBPoolTranslationPaused:             "1",
			},
			options: &translator.IngressTranslationOptions{
				BaseTranslationOptions: translator.BaseTranslationOptions{
					EdgeLBPoolName:              "foo",
					EdgeLBPoolRole:              "custom_role",
					EdgeLBPoolCpus:              resource.MustParse("250m"),
					EdgeLBPoolMem:               resource.MustParse("2Gi"),
					EdgeLBPoolSize:              3,
					EdgeLBPoolCreationStrategy:  constants.EdgeLBPoolCreationStrategyOnce,
					EdgeLBPoolTranslationPaused: true,
				},
				EdgeLBPoolPort: 14708,
			},
			error: nil,
		},
	}
	// Run each of the tests defined above.
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		// Create a dummy Ingress resource containing the annotations for the current test.
		r := ingresstestutil.DummyIngressResource("foo", "bar", ingresstestutil.WithAnnotations(test.annotations))
		// Compute the translation options for the resource.
		o, err := translator.ComputeIngressTranslationOptions(r)
		if err != nil {
			// Make sure the error matches the expected one.
			assert.EqualError(t, err, test.error.Error())
		} else {
			// Make sure the translation options match the expected ones.
			assert.Equal(t, test.options, o)
		}
	}
}
