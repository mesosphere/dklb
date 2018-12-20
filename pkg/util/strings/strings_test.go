package strings_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mesosphere/dklb/pkg/util/strings"
)

// TestRemoveSlashes tests the "RemoveSlashes" function.
func TestRemoveSlashes(t *testing.T) {
	tests := []struct {
		i string
		o string
	}{
		{
			i: "",
			o: "",
		},
		{
			i: "foo",
			o: "foo",
		},
		{
			i: "dev/kubernetes01",
			o: "devkubernetes01",
		},
		{
			i: "dev/kubernetes/k-01",
			o: "devkubernetesk-01",
		},
	}
	for _, test := range tests {
		assert.Equal(t, test.o, strings.RemoveSlashes(test.i))
	}
}
