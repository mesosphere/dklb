package strings_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mesosphere/dklb/pkg/util/strings"
)

// TestReplaceForwardSlashes tests the "ReplaceForwardSlashes" function.
func TestReplaceForwardSlashes(t *testing.T) {
	tests := []struct {
		description string
		v           string
		r           string
		result      string
	}{
		{
			description: "empty string",
			v:           "",
			r:           ":",
			result:      "",
		},
		{
			description: "string without forward slashes",
			v:           "foo",
			r:           "-",
			result:      "foo",
		},
		{
			description: "string with a single forward slash",
			v:           "dev/kubernetes01",
			r:           "--",
			result:      "dev--kubernetes01",
		},
		{
			description: "string with multiple forward slashes",
			v:           "dev/kubernetes/k-01",
			r:           "--",
			result:      "dev--kubernetes--k-01",
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		assert.Equal(t, test.result, strings.ReplaceForwardSlashes(test.v, test.r))
	}
}

// TestRoundTrip tests the "ReplaceDotsWithForwardSlashes" and "ReplaceForwardSlashesWithDots" functions by performing a round-trip and making sure all outputs match the expectations.
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
		// Call "ReplaceForwardSlashesWithDots" on the input string and make sure the output matches our expectations.
		replaceSlashesOutput := strings.ReplaceForwardSlashesWithDots(test.originalInput)
		assert.Equal(t, test.replaceSlashesOutput, replaceSlashesOutput)
		// Call "ReplaceDotsWithForwardSlashes" on the previous output and make sure the output matches the original input.
		replaceDotsOutput := strings.ReplaceDotsWithForwardSlashes(replaceSlashesOutput)
		assert.Equal(t, test.originalInput, replaceDotsOutput)
	}
}
