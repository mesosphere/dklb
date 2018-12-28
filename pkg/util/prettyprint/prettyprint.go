package prettyprint

import (
	"strings"

	"github.com/davecgh/go-spew/spew"
)

// LoggingFunc represents a logging function accepting formatting arguments.
type LoggingFunc func(string, ...interface{})

// Logf logs a prettified string representation of an object at "debug" level to the specified entry.
func Logf(fn LoggingFunc, obj interface{}, message string, args ...interface{}) {
	fn(message, args)
	for _, line := range strings.Split(spew.Sdump(obj), "\n") {
		if line != "" {
			fn(line)
		}
	}
}
