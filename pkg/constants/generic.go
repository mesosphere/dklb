package constants

import (
	"time"
)

const (
	// ComponentName is the component name to report when performing leader election and emitting Kubernetes events.
	ComponentName = "dklb"
	// DefaultEdgeLBHost is the default host at which the EdgeLB API server can be reached.
	DefaultEdgeLBHost = "api.edgelb.marathon.l4lb.thisdcos.directory"
	// DefaultEdgeLBPath is the default path at which the EdgeLB API server can be reached.
	DefaultEdgeLBPath = "/"
	// DefaultEdgeLBScheme is the default scheme to use when communicating with the EdgeLB API server.
	DefaultEdgeLBScheme = "http"
	// DefaultResyncPeriod is the (default) maximum amount of time that may elapse between two consecutive synchronizations of Ingress/Service resources and the status of EdgeLB pools.
	DefaultResyncPeriod = 2 * time.Minute
	// KubeNodeTaskPattern is the pattern used to match Mesos tasks that correspond to Kubernetes nodes (either private or public).
	KubeNodeTaskPattern = "^kube-node-.*$"
	// KubeSystemNamespaceName holds the name of the "kube-system" namespace.
	KubeSystemNamespaceName = "kube-system"
)
