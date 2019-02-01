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
		// Test computing options for an Ingress resource without any annotations.
		// Make sure the name of the EdgeLB pool is computed as expected, and that the default values are used everywhere else.
		{
			description: "compute options for an Ingress resource without any annotations",
			annotations: map[string]string{},
			options: &translator.IngressTranslationOptions{
				BaseTranslationOptions: translator.BaseTranslationOptions{
					CloudLoadBalancerConfigMapName: nil,
					EdgeLBPoolName:                 "dev--kubernetes01--foo--bar",
					EdgeLBPoolRole:                 translator.DefaultEdgeLBPoolRole,
					EdgeLBPoolNetwork:              constants.EdgeLBHostNetwork,
					EdgeLBPoolCpus:                 translator.DefaultEdgeLBPoolCpus,
					EdgeLBPoolMem:                  translator.DefaultEdgeLBPoolMem,
					EdgeLBPoolSize:                 translator.DefaultEdgeLBPoolSize,
					EdgeLBPoolCreationStrategy:     translator.DefaultEdgeLBPoolCreationStrategy,
				},
				EdgeLBPoolPort: translator.DefaultEdgeLBPoolPort,
			},
			error: nil,
		},
		// Test computing options for an Ingress resource specifying the name of the target EdgeLB pool.
		// Make sure the name of the EdgeLB pool is captured as expected, and that the default values are used everywhere else.
		{
			description: "compute options for an Ingress resource specifying the name of the target EdgeLB pool",
			annotations: map[string]string{
				constants.EdgeLBPoolNameAnnotationKey: "foo",
			},
			options: &translator.IngressTranslationOptions{
				BaseTranslationOptions: translator.BaseTranslationOptions{
					CloudLoadBalancerConfigMapName: nil,
					EdgeLBPoolName:                 "foo",
					EdgeLBPoolRole:                 translator.DefaultEdgeLBPoolRole,
					EdgeLBPoolNetwork:              constants.EdgeLBHostNetwork,
					EdgeLBPoolCpus:                 translator.DefaultEdgeLBPoolCpus,
					EdgeLBPoolMem:                  translator.DefaultEdgeLBPoolMem,
					EdgeLBPoolSize:                 translator.DefaultEdgeLBPoolSize,
					EdgeLBPoolCreationStrategy:     translator.DefaultEdgeLBPoolCreationStrategy,
				},
				EdgeLBPoolPort: translator.DefaultEdgeLBPoolPort,
			},
			error: nil,
		},
		// Test computing options for an Ingress resource defining a custom frontend bind port.
		// Make sure the frontend bind port is captured as expected, and that the default values are used everywhere else.
		{
			description: "compute options for an Ingress resource defining a custom frontend bind port",
			annotations: map[string]string{
				constants.EdgeLBPoolPortAnnotationKey: "14708",
			},
			options: &translator.IngressTranslationOptions{
				BaseTranslationOptions: translator.BaseTranslationOptions{
					CloudLoadBalancerConfigMapName: nil,
					EdgeLBPoolName:                 "dev--kubernetes01--foo--bar",
					EdgeLBPoolRole:                 translator.DefaultEdgeLBPoolRole,
					EdgeLBPoolNetwork:              constants.EdgeLBHostNetwork,
					EdgeLBPoolCpus:                 translator.DefaultEdgeLBPoolCpus,
					EdgeLBPoolMem:                  translator.DefaultEdgeLBPoolMem,
					EdgeLBPoolSize:                 translator.DefaultEdgeLBPoolSize,
					EdgeLBPoolCreationStrategy:     translator.DefaultEdgeLBPoolCreationStrategy,
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
				constants.EdgeLBPoolPortAnnotationKey: "74511",
			},
			options: nil,
			error:   fmt.Errorf("%d is not a valid port number", 74511),
		},
		// Test computing options for an Ingress resource defining custom values for all the options (except cloud load-balancer configuration).
		// Make sure that all values are adequately captured.
		{
			description: "compute options for an Ingress resource defining custom values for all the options (except cloud load-balancer configuration)",
			annotations: map[string]string{
				constants.EdgeLBPoolNameAnnotationKey:             "foo",
				constants.EdgeLBPoolRoleAnnotationKey:             "custom_role",
				constants.EdgeLBPoolNetworkAnnotationKey:          "foo_network",
				constants.EdgeLBPoolCpusAnnotationKey:             "250m",
				constants.EdgeLBPoolMemAnnotationKey:              "2Gi",
				constants.EdgeLBPoolSizeAnnotationKey:             "3",
				constants.EdgeLBPoolCreationStrategyAnnotationKey: string(constants.EdgeLBPoolCreationStrategyOnce),
				constants.EdgeLBPoolPortAnnotationKey:             "14708",
				constants.EdgeLBPoolTranslationPaused:             "1",
			},
			options: &translator.IngressTranslationOptions{
				BaseTranslationOptions: translator.BaseTranslationOptions{
					CloudLoadBalancerConfigMapName: nil,
					EdgeLBPoolName:                 "foo",
					EdgeLBPoolRole:                 "custom_role",
					EdgeLBPoolNetwork:              "foo_network",
					EdgeLBPoolCpus:                 resource.MustParse("250m"),
					EdgeLBPoolMem:                  resource.MustParse("2Gi"),
					EdgeLBPoolSize:                 3,
					EdgeLBPoolCreationStrategy:     constants.EdgeLBPoolCreationStrategyOnce,
					EdgeLBPoolTranslationPaused:    true,
				},
				EdgeLBPoolPort: 14708,
			},
			error: nil,
		},
		// Test computing options for an Ingress resource with a custom but invalid port mapping.
		// Make sure an error is returned.
		{
			description: "compute options for an Ingress resource with a custom but invalid port mapping",
			annotations: map[string]string{
				constants.EdgeLBPoolPortAnnotationKey: "74511",
			},
			options: nil,
			error:   fmt.Errorf("%d is not a valid port number", 74511),
		},
		// Test computing options for an Ingress resource having an invalid port mapping.
		// Make sure an error is returned.
		{
			description: "compute options for a Service resource having an invalid port mapping",
			annotations: map[string]string{
				constants.EdgeLBPoolPortAnnotationKey: "foo",
			},
			options: nil,
			error:   fmt.Errorf("failed to parse %q as the frontend bind port to use: %v", "foo", "strconv.Atoi: parsing \"foo\": invalid syntax"),
		},
	}
	// Run each of the tests defined above.
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		// Create a dummy Ingress resource containing the annotations for the current test.
		r := ingresstestutil.DummyIngressResource("foo", "bar", ingresstestutil.WithAnnotations(test.annotations))
		// Compute the translation options for the resource.
		o, err := translator.ComputeIngressTranslationOptions(testClusterName, r)
		if err != nil {
			// Make sure the error matches the expected one.
			assert.Equal(t, test.error, err)
		} else {
			// Make sure the translation options match the expected ones.
			assert.Equal(t, test.options, o)
		}
	}
}
