package prettyprint

import (
	"encoding/json"
	"strings"

	"github.com/davecgh/go-spew/spew"
)

// LoggingFunc represents a logging function accepting formatting arguments.
type LoggingFunc func(string, ...interface{})

// LogfSpew logs a prettified string representation of an object using the specified log function.
func LogfSpew(fn LoggingFunc, obj interface{}, message string, args ...interface{}) {
	fn(message, args)
	for _, line := range strings.Split(spew.Sdump(obj), "\n") {
		if line != "" {
			fn(line)
		}
	}
}

// LogfJSON logs the JSON representation of an object using the specified log function.
func LogfJSON(fn LoggingFunc, obj interface{}, message string, args ...interface{}) {
	fn(message, args)
	b, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		fn("failed to marshal object: %v", err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if line != "" {
			fn(line)
		}
	}
}
