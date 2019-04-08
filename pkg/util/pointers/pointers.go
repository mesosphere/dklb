package pointers

// NewBool returns a pointer to the specified boolean value.
func NewBool(v bool) *bool {
	return &v
}

// NewFloat64 returns a pointer to the specified float64 value.
func NewFloat64(v float64) *float64 {
	return &v
}

// NewInt32 returns a pointer to the specified integer value.
func NewInt32(v int32) *int32 {
	return &v
}

// NewString returns a pointer to the specified string value.
func NewString(v string) *string {
	return &v
}
