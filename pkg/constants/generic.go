package constants

import (
	"time"
)

const (
	// ComponentName is the component name to report when performing leader election and emitting Kubernetes events.
	ComponentName = "dklb"
	// DefaultEdgeLBHost is the default host at which the EdgeLB API server can be reached.
	DefaultEdgeLBHost = "api.edgelb.marathon.l4lb.thisdcos.directory"
	// DefaultEdgeLBHost is the default path at which the EdgeLB API server can be reached on the host.
	DefaultEdgeLBPath = "/"
	// DefaultEdgeLBScheme is the default scheme to use when communicating with the EdgeLB API server.
	DefaultEdgeLBScheme = "http"
	// DefaultResyncPeriod is the maximum amount of time that may elapse between two consecutive synchronizations of Ingress/Service resources and the status of EdgeLB pools.
	DefaultResyncPeriod = 1 * time.Minute
)
