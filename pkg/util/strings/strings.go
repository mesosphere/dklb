package strings

import (
	"strings"
)

// RemoveSlashes returns a string built from the specified one by removing all forward slashes.
func RemoveSlashes(v string) string {
	return strings.Replace(v, "/", "", -1)
}
