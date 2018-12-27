package framework

import (
	"time"
)

const (
	// DefaultRetryInterval is the default interval to use in "retry" operations.
	DefaultRetryInterval = 10 * time.Second
	// DefaultRetryTimeout is the default timeout to use in "retry" operations.
	DefaultRetryTimeout = 3 * time.Minute
	// DefaultEdgeLBOperationTimeout is the default timeout to use for EdgeLB operations.
	DefaultEdgeLBOperationTimeout = 5 * time.Second
)
