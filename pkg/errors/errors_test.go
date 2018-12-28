package errors_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mesosphere/dklb/pkg/errors"
)

// TestIsNotFound tests the creation and verification of "NotFound" errors.
func TestIsNotFound(t *testing.T) {
	tests := []struct {
		description string
		error       error
		isNotFound  bool
	}{
		{
			description: "error is of type \"NotFound\"",
			error:       errors.NotFound(fmt.Errorf("resource %q not found", "foo")),
			isNotFound:  true,
		},
		{
			description: "error is not of type \"NotFound\"",
			error:       fmt.Errorf("resource %q not found", "foo"),
			isNotFound:  false,
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		assert.Equal(t, test.isNotFound, errors.IsNotFound(test.error))
	}
}

// TestIsUnknown tests the creation and verification of "Unknown" errors.
func TestUnknown(t *testing.T) {
	tests := []struct {
		description string
		error       error
		isUnknown   bool
	}{
		{
			description: "error is of type \"Unknown\"",
			error:       errors.Unknown(fmt.Errorf("failed to connect to server %s", "1.2.3.4")),
			isUnknown:   true,
		},
		{
			description: "error is not of type \"Unknown\"",
			error:       fmt.Errorf("failed to connect to server %s", "1.2.3.4"),
			isUnknown:   false,
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		assert.Equal(t, test.isUnknown, errors.IsUnknown(test.error))
	}
}
