package prettyprint

import (
	"github.com/davecgh/go-spew/spew"
)

// Sprint returns a prettified string representation of an object.
func Sprint(val interface{}) string {
	return spew.Sdump(val)
}
