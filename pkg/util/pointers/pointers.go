package pointers

// NewBool returns a pointer to the specified boolean value.
func NewBool(v bool) *bool {
	return &v
}

// NewInt32 returns a pointer to the specified integer value.
func NewInt32(v int32) *int32 {
	return &v
}
