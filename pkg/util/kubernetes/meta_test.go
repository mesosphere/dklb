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
		i interface{}
		o string
	}{
		{
			i: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "",
					Name:      "foo",
				},
			},
			o: "foo",
		},
		{
			i: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},
			},
			o: "foo/bar",
		},
		{
			i: "foo",
			o: "(unknown)",
		},
		{
			i: 1,
			o: "(unknown)",
		},
	}
	for _, test := range tests {
		assert.Equal(t, test.o, kubernetes.Key(test.i))
	}
}
