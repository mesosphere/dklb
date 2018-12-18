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
		annotations map[string]string
		ports       []corev1.ServicePort
		options     *translator.ServiceTranslationOptions
		error       error
	}{
		// Test computing options for an Service resource without the required annotations.
		// Make sure an error is returned.
		{
			annotations: map[string]string{},
			options:     nil,
			error:       fmt.Errorf("required annotation %q has not been provided", constants.EdgeLBPoolNameAnnotationKey),
		},
		// Test computing options for an Service resource with just the required annotations.
		// Make sure the name of the EdgeLB pool is captured as expected, and that the default values are used everywhere else.
		{
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
				EdgeLBPoolPortMap: map[int]int{
					80: 80,
				},
			},
			error: nil,
		},
		// Test computing options for an Service resource with a custom port mapping.
		// Make sure the name of the EdgeLB pool and the port mapping are captured as expected, and that the default values are used everywhere else.
		{
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
				EdgeLBPoolPortMap: map[int]int{
					80:  8080,
					443: 443,
				},
			},
			error: nil,
		},

		// Test computing options for an Service resource with a custom but invalid port mappings.
		// Make sure an error is returned.
		{
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
	}
	// Run each of the tests defined above.
	for _, test := range tests {
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