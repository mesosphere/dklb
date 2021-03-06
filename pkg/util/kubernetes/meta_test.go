package kubernetes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mesosphere/dklb/pkg/util/kubernetes"
)

// TestKey tests the "Key" function.
func TestKey(t *testing.T) {
	tests := []struct {
		description string
		input       interface{}
		output      string
	}{
		{
			description: "non-namespaced resource",
			input: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "",
					Name:      "foo",
				},
			},
			output: "foo",
		},
		{
			description: "namespaced resource",
			input: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},
			},
			output: "foo/bar",
		},
		{
			description: "not a resource",
			input:       "foo",
			output:      "(unknown)",
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		assert.Equal(t, test.output, kubernetes.Key(test.input))
	}
}
