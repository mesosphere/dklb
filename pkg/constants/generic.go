package constants

import (
	"time"
)

const (
	// ComponentName is the component name to report when performing leader election and emitting Kubernetes events.
	ComponentName = "dklb"
	// DefaultResyncPeriod is the maximum amount of time that may elapse between two consecutive synchronizations of Ingress/Service resources and the status of EdgeLB pools.
	DefaultResyncPeriod = 1 * time.Minute
)
