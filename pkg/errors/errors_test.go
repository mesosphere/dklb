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
		error      error
		isNotFound bool
	}{
		{
			error:      errors.NotFound(fmt.Errorf("resource %q not found", "foo")),
			isNotFound: true,
		},
		{
			error:      fmt.Errorf("resource %q not found", "foo"),
			isNotFound: false,
		},
	}
	for _, test := range tests {
		assert.Equal(t, test.isNotFound, errors.IsNotFound(test.error))
	}
}

// TestIsUnknown tests the creation and verification of "Unknown" errors.
func TestUnknown(t *testing.T) {
	tests := []struct {
		error     error
		isUnknown bool
	}{
		{
			error:     errors.Unknown(fmt.Errorf("failed to connect to server %s", "1.2.3.4")),
			isUnknown: true,
		},
		{
			error:     fmt.Errorf("failed to connect to server %s", "1.2.3.4"),
			isUnknown: false,
		},
	}
	for _, test := range tests {
		assert.Equal(t, test.isUnknown, errors.IsUnknown(test.error))
	}
}
