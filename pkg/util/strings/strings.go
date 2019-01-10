package strings

import (
	"strings"
)

// ReplaceDotsWithForwardSlashes returns a string built from the specified one by replacing all dots ("/") with a forward slash ("/").
func ReplaceDotsWithForwardSlashes(v string) string {
	return strings.Replace(v, ".", "/", -1)
}

// ReplaceForwardSlashes returns a string built from the specified one by replacing all forward slashes ("/") with the provided replacement string.
func ReplaceForwardSlashes(v, r string) string {
	return strings.Replace(v, "/", r, -1)
}

// ReplaceForwardSlashesWithDots returns a string built from the specified one by replacing all forward slashes ("/") with a dot (".").
func ReplaceForwardSlashesWithDots(v string) string {
	return ReplaceForwardSlashes(v, ".")
}
