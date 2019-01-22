package features

import (
	"fmt"
	"strconv"
	"strings"
)

// Feature represents a feature of dklb.
type Feature string

// FeatureMap is a mapping between features of dklb and their current status.
type FeatureMap map[Feature]bool

const (
	// RegisterAdmissionWebhook is used to indicate whether dklb should register its admission webhook.
	RegisterAdmissionWebhook Feature = "RegisterAdmissionWebhook"
	// ServeAdmissionWebhook is used to indicate whether dklb should serve its admission webhook.
	ServeAdmissionWebhook Feature = "ServeAdmissionWebhook"
)

var (
	// DefaultFeatureMap represents the default status of the dklb feature gates.
	DefaultFeatureMap = map[Feature]bool{
		RegisterAdmissionWebhook: true,
		ServeAdmissionWebhook:    true,
	}
)

// ParseFeatureMap parses the specified string into a feature map.
func ParseFeatureMap(str string) (FeatureMap, error) {
	// Create the feature map we will be returning.
	res := make(FeatureMap, len(DefaultFeatureMap))
	// Set all features to their default status.
	for feature, status := range DefaultFeatureMap {
		res[feature] = status
	}
	// Remove all whitespaces from the provided string and split the resulting string by "," in order to obtain all the "key=value" pairs.
	kvs := strings.Split(strings.Replace(str, " ", "", -1), ",")
	// Iterate over all the "key=value" pairs and set the status of the corresponding feature in the feature map.
	for _, kv := range kvs {
		// Skip "empty" key/value pairs.
		if len(kv) == 0 {
			continue
		}
		// Split the key/value pair by "=".
		p := strings.Split(kv, "=")
		if len(p) != 2 {
			return nil, fmt.Errorf("invalid key/value pair: %q", kv)
		}
		// Grab the key and its value.
		k, v := p[0], p[1]
		// Make sure the feature corresponding to the key exists.
		if _, exists := DefaultFeatureMap[Feature(k)]; !exists {
			return nil, fmt.Errorf("invalid feature key: %q", k)
		}
		// Attempt to parse the value as a boolean.
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q as a boolean value", v)
		}
		// Set the feature's status in the feature map.
		res[Feature(k)] = b
	}
	// Return the feature map.
	return res, nil
}

// IsEnabled returns a value indicating whether the specified feature is enabled.
func (m FeatureMap) IsEnabled(feature Feature) bool {
	return m[feature]
}
