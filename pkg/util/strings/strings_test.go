package strings_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mesosphere/dklb/pkg/util/strings"
)

// TestRoundTrip tests the "ReplaceDots" and "ReplaceSlashes" functions by performing a round-trip and making sure all outputs match the expectations.
func TestRoundTrip(t *testing.T) {
	tests := []struct {
		description          string
		originalInput        string
		replaceSlashesOutput string
	}{
		{
			description:          "empty string",
			originalInput:        "",
			replaceSlashesOutput: "",
		},
		{
			description:          "string without dots",
			originalInput:        "foo",
			replaceSlashesOutput: "foo",
		},
		{
			description:          "string with a single dot",
			originalInput:        "dev/kubernetes01",
			replaceSlashesOutput: "dev.kubernetes01",
		},
		{
			description:          "string with multiple dots",
			originalInput:        "dev/kubernetes/k-01",
			replaceSlashesOutput: "dev.kubernetes.k-01",
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		// Call "ReplaceSlashes" on the input string and make sure the output matches our expectations.
		replaceSlashesOutput := strings.ReplaceSlashes(test.originalInput)
		assert.Equal(t, test.replaceSlashesOutput, replaceSlashesOutput)
		// Call "ReplaceDots" on the previous output and make sure the output matches the original input.
		replaceDotsOutput := strings.ReplaceDots(replaceSlashesOutput)
		assert.Equal(t, test.originalInput, replaceDotsOutput)
	}
}
