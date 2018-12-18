package errors

// errorNotFound represents an error thrown when a given resource does not exist.
type errorNotFound struct {
	error
}

// NotFound creates an "errorNotFound" error from the specified error.
func NotFound(err error) error {
	if err == nil {
		return nil
	}
	return errorNotFound{err}
}

// IsNotFound returns whether the specified error is of type "NotFound".
func IsNotFound(err error) bool {
	_, ok := err.(errorNotFound)
	return ok
}

// errorUnknown represents a generic error without a well-defined cause.
type errorUnknown struct {
	error
}

// Unknown creates an "unknown" error from the specified error.
func Unknown(err error) error {
	if err == nil {
		return nil
	}
	return errorUnknown{err}
}

// IsUnknown returns whether the specified error is of type "Unknown".
func IsUnknown(err error) bool {
	_, ok := err.(errorUnknown)
	return ok
}
