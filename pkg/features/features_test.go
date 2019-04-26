package features_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mesosphere/dklb/pkg/features"
)

// TestParseFeatureMap tests the "ParseFeatureMap" function.
func TestParseFeatureMap(t *testing.T) {
	tests := []struct {
		description        string
		str                string
		expectedFeatureMap features.FeatureMap
		expectedError      error
	}{
		{
			description: "empty feature gate string",
			str:         "",
			expectedFeatureMap: features.FeatureMap{
				features.RegisterAdmissionWebhook: true,
				features.ServeAdmissionWebhook:    true,
			},
			expectedError: nil,
		},
		{
			description:        "malformed feature gate string",
			str:                "foobar",
			expectedFeatureMap: nil,
			expectedError:      fmt.Errorf("invalid key/value pair: %q", "foobar"),
		},
		{
			description: "valid feature gate string disabling a feature",
			str:         "RegisterAdmissionWebhook=false",
			expectedFeatureMap: features.FeatureMap{
				features.RegisterAdmissionWebhook: false,
				features.ServeAdmissionWebhook:    true,
			},
			expectedError: nil,
		},
		{
			description: "feature gate string with \"empty\" key/value pair",
			str:         "ServeAdmissionWebhook=false,",
			expectedFeatureMap: features.FeatureMap{
				features.RegisterAdmissionWebhook: true,
				features.ServeAdmissionWebhook:    false,
			},
			expectedError: nil,
		},
		{
			description:        "feature gate string with invalid key",
			str:                "UnknownFeature=true",
			expectedFeatureMap: nil,
			expectedError:      fmt.Errorf("invalid feature key: %q", "UnknownFeature"),
		},
		{
			description:        "feature gate string with invalid value",
			str:                "RegisterAdmissionWebhook=foo",
			expectedFeatureMap: nil,
			expectedError:      fmt.Errorf("failed to parse %q as a boolean value", "foo"),
		},
	}
	for _, test := range tests {
		m, err := features.ParseFeatureMap(test.str)
		assert.Equal(t, test.expectedFeatureMap, m, "test case: %s", test.description)
		assert.Equal(t, test.expectedError, err, "test case: %s", test.description)
	}
}

// TestIsEnabled tests the "IsEnabled" function.
func TestIsEnabled(t *testing.T) {
	m := features.FeatureMap{
		features.RegisterAdmissionWebhook: true,
	}
	assert.True(t, m.IsEnabled(features.RegisterAdmissionWebhook))
	m = features.FeatureMap{
		features.RegisterAdmissionWebhook: false,
	}
	assert.False(t, m.IsEnabled(features.RegisterAdmissionWebhook))
}
