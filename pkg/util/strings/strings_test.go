package strings_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mesosphere/dklb/pkg/util/strings"
)

// TestRemoveSlashes tests the "RemoveSlashes" function.
func TestRemoveSlashes(t *testing.T) {
	tests := []struct {
		description string
		input       string
		output      string
	}{
		{
			description: "empty string",
			input:       "",
			output:      "",
		},
		{
			description: "string without slashes",
			input:       "foo",
			output:      "foo",
		},
		{
			description: "string with a single slash",
			input:       "dev/kubernetes01",
			output:      "devkubernetes01",
		},
		{
			description: "string with multiple slashes",
			input:       "dev/kubernetes/k-01",
			output:      "devkubernetesk-01",
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		assert.Equal(t, test.output, strings.RemoveSlashes(test.input))
	}
}
