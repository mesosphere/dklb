package strings

import (
	"strings"
)

// ReplaceDots returns a string built from the specified one by replacing all dots ("/") with a forward slash ("/").
func ReplaceDots(v string) string {
	return strings.Replace(v, ".", "/", -1)
}

// ReplaceSlashes returns a string built from the specified one by replacing all forward slashes ("/") with a dot (".").
func ReplaceSlashes(v string) string {
	return strings.Replace(v, "/", ".", -1)
}
