package translator_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/translator"
	servicetestutil "github.com/mesosphere/dklb/test/util/kubernetes/service"
)

// TestComputeServiceTranslationOptions tests parsing of annotations defined in a Service resource.
func TestComputeServiceTranslationOptions(t *testing.T) {
	tests := []struct {
		description string
		annotations map[string]string
		ports       []corev1.ServicePort
		options     *translator.ServiceTranslationOptions
		error       error
	}{
		// Test computing options for a Service resource without the required annotations.
		// Make sure an error is returned.
		{
			description: "compute options for a Service resource without the required annotations",
			annotations: map[string]string{},
			options:     nil,
			error:       fmt.Errorf("required annotation %q has not been provided", constants.EdgeLBPoolNameAnnotationKey),
		},
		// Test computing options for a Service resource with just the required annotations.
		// Make sure the name of the EdgeLB pool is captured as expected, and that the default values are used everywhere else.
		{
			description: "compute options for a Service resource with just the required annotations",
			annotations: map[string]string{
				constants.EdgeLBPoolNameAnnotationKey: "foo",
			},
			ports: []corev1.ServicePort{
				{
					Port: 80,
				},
			},
			options: &translator.ServiceTranslationOptions{
				BaseTranslationOptions: translator.BaseTranslationOptions{
					EdgeLBPoolName:             "foo",
					EdgeLBPoolRole:             translator.DefaultEdgeLBPoolRole,
					EdgeLBPoolCpus:             translator.DefaultEdgeLBPoolCpus,
					EdgeLBPoolMem:              translator.DefaultEdgeLBPoolMem,
					EdgeLBPoolSize:             translator.DefaultEdgeLBPoolSize,
					EdgeLBPoolCreationStrategy: translator.DefaultEdgeLBPoolCreationStrategy,
				},
				EdgeLBPoolPortMap: map[int32]int32{
					80: 80,
				},
			},
			error: nil,
		},
		// Test computing options for a Service resource with a custom port mapping.
		// Make sure the name of the EdgeLB pool and the port mapping are captured as expected, and that the default values are used everywhere else.
		{
			description: "compute options for a Service resource with a custom port mapping",
			annotations: map[string]string{
				constants.EdgeLBPoolNameAnnotationKey:                         "foo",
				fmt.Sprintf("%s%d", constants.EdgeLBPoolPortMapKeyPrefix, 80): "8080",
			},
			ports: []corev1.ServicePort{
				{
					Port: 80,
				},
				{
					Port: 443,
				},
			},
			options: &translator.ServiceTranslationOptions{
				BaseTranslationOptions: translator.BaseTranslationOptions{
					EdgeLBPoolName:             "foo",
					EdgeLBPoolRole:             translator.DefaultEdgeLBPoolRole,
					EdgeLBPoolCpus:             translator.DefaultEdgeLBPoolCpus,
					EdgeLBPoolMem:              translator.DefaultEdgeLBPoolMem,
					EdgeLBPoolSize:             translator.DefaultEdgeLBPoolSize,
					EdgeLBPoolCreationStrategy: translator.DefaultEdgeLBPoolCreationStrategy,
				},
				EdgeLBPoolPortMap: map[int32]int32{
					80:  8080,
					443: 443,
				},
			},
			error: nil,
		},
		// Test computing options for a Service resource with a custom but invalid port mapping.
		// Make sure an error is returned.
		{
			description: "compute options for a Service resource with a custom but invalid port mapping",
			annotations: map[string]string{
				constants.EdgeLBPoolNameAnnotationKey:                          "foo",
				fmt.Sprintf("%s%d", constants.EdgeLBPoolPortMapKeyPrefix, 443): "74511",
			},
			ports: []corev1.ServicePort{
				{
					Port: 80,
				},
				{
					Port: 443,
				},
			},
			options: nil,
			error:   fmt.Errorf("%d is not a valid port number", 74511),
		},
		// Test computing options for a Service resource using an unsupported protocol.
		// Make sure an error is returned.
		{
			description: "compute options for a Service resource using an unsupported protocol",
			annotations: map[string]string{
				constants.EdgeLBPoolNameAnnotationKey: "foo",
			},
			ports: []corev1.ServicePort{
				{
					Port:     80,
					Protocol: corev1.ProtocolUDP,
				},
			},
			options: nil,
			error:   fmt.Errorf("protocol %q is not supported", corev1.ProtocolUDP),
		},
		// Test computing options for a Service resource having duplicate port mappings.
		// Make sure an error is returned.
		{
			description: "compute options for a Service resource having duplicate port mappings",
			annotations: map[string]string{
				constants.EdgeLBPoolNameAnnotationKey:                           "foo",
				fmt.Sprintf("%s%d", constants.EdgeLBPoolPortMapKeyPrefix, 8080): "18080",
				fmt.Sprintf("%s%d", constants.EdgeLBPoolPortMapKeyPrefix, 8081): "18080",
			},
			ports: []corev1.ServicePort{
				{
					Name: "http-1",
					Port: 8080,
				},
				{
					Name: "http-2",
					Port: 8081,
				},
			},
			options: nil,
			error:   fmt.Errorf("port %d is mapped to both %q and %q service ports", 18080, "http-1", "http-2"),
		},
		// Test computing options for a Service resource having an invalid port mapping.
		// Make sure an error is returned.
		{
			description: "compute options for a Service resource having an invalid port mapping",
			annotations: map[string]string{
				constants.EdgeLBPoolNameAnnotationKey:                           "foo",
				fmt.Sprintf("%s%d", constants.EdgeLBPoolPortMapKeyPrefix, 8080): "18080",
				fmt.Sprintf("%s%d", constants.EdgeLBPoolPortMapKeyPrefix, 8081): "foo",
			},
			ports: []corev1.ServicePort{
				{
					Port: 8080,
				},
				{
					Port: 8081,
				},
			},
			options: nil,
			error:   fmt.Errorf("failed to parse %q as a frontend bind port: %v", "foo", "strconv.Atoi: parsing \"foo\": invalid syntax"),
		},
		// Test computing options for a Service resource having an invalid CPU request.
		// Make sure an error is returned.
		{
			description: "compute options for a Service resource having an invalid CPU request",
			annotations: map[string]string{
				constants.EdgeLBPoolNameAnnotationKey: "foo",
				constants.EdgeLBPoolCpusAnnotationKey: "foo",
			},
			ports: []corev1.ServicePort{
				{
					Port: 80,
				},
			},
			options: nil,
			error:   fmt.Errorf("failed to parse %q as the amount of cpus to request: %s", "foo", "quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'"),
		},
		// Test computing options for a Service resource having an invalid memory request.
		// Make sure an error is returned.
		{
			description: "compute options for a Service resource having an invalid memory request",
			annotations: map[string]string{
				constants.EdgeLBPoolNameAnnotationKey: "foo",
				constants.EdgeLBPoolMemAnnotationKey:  "foo",
			},
			ports: []corev1.ServicePort{
				{
					Port: 80,
				},
			},
			options: nil,
			error:   fmt.Errorf("failed to parse %q as the amount of memory to request: %s", "foo", "quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'"),
		},
		// Test computing options for a Service resource having an invalid (malformed) size request.
		// Make sure an error is returned.
		{
			description: "compute options for a Service resource having an invalid (malformed) size request",
			annotations: map[string]string{
				constants.EdgeLBPoolNameAnnotationKey: "foo",
				constants.EdgeLBPoolSizeAnnotationKey: "foo",
			},
			ports: []corev1.ServicePort{
				{
					Port: 80,
				},
			},
			options: nil,
			error:   fmt.Errorf("failed to parse %q as the size to request for the edgelb pool: %s", "foo", "strconv.Atoi: parsing \"foo\": invalid syntax"),
		},
		// Test computing options for a Service resource having an invalid (negative) size request.
		// Make sure an error is returned.
		{
			description: "compute options for a Service resource having an invalid (negative) size request",
			annotations: map[string]string{
				constants.EdgeLBPoolNameAnnotationKey: "foo",
				constants.EdgeLBPoolSizeAnnotationKey: "-1",
			},
			ports: []corev1.ServicePort{
				{
					Port: 80,
				},
			},
			options: nil,
			error:   fmt.Errorf("%d is not a valid size", -1),
		},
	}
	// Run each of the tests defined above.
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		// Create a dummy Service resource containing the annotations for the current test.
		r := servicetestutil.DummyServiceResource("foo", "bar", servicetestutil.WithAnnotations(test.annotations), servicetestutil.WithPorts(test.ports))
		// Compute the translation options for the resource.
		o, err := translator.ComputeServiceTranslationOptions(r)
		if err != nil {
			// Make sure the error matches the expected one.
			assert.EqualError(t, err, test.error.Error())
		} else {
			// Make sure the translation options match the expected ones.
			assert.Equal(t, test.options, o)
		}
	}
}
