package pointers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mesosphere/dklb/pkg/util/pointers"
)

// TestNewBool tests the "NewBool" function.
func TestNewBool(t *testing.T) {
	b1 := pointers.NewBool(true)
	b2 := pointers.NewBool(true)
	b3 := pointers.NewBool(false)
	assert.True(t, b1 != b2)
	assert.True(t, *b1 == *b2)
	assert.True(t, b1 != b3)
	assert.True(t, *b1 != *b3)
}

// TestNewInt32 tests the "NewInt32" function.
func TestNewInt32(t *testing.T) {
	v1 := pointers.NewInt32(1)
	v2 := pointers.NewInt32(1)
	assert.True(t, v1 != v2)
	assert.True(t, *v1 == *v2)
}

// TestNewString tests the "NewString" function.
func TestNewString(t *testing.T) {
	v1 := pointers.NewString("foo")
	v2 := pointers.NewString("foo")
	assert.True(t, v1 != v2)
	assert.True(t, *v1 == *v2)
}
