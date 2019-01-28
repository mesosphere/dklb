package constants

import (
	"time"
)

const (
	// ComponentName is the component name to report when performing leader election and emitting Kubernetes events.
	ComponentName = "dklb"
	// DefaultBackendServiceName is the name of the Service resource that exposes dklb as a default backend for Ingress resources.
	DefaultBackendServiceName = "dklb"
	// DefaultBackendServicePort is the service port defined in the Service resource that exposes dklb as a default backend for Ingress resources.
	DefaultBackendServicePort = 80
	// DefaultEdgeLBHost is the default host at which the EdgeLB API server can be reached.
	DefaultEdgeLBHost = "api.edgelb.marathon.l4lb.thisdcos.directory"
	// DefaultEdgeLBPath is the default path at which the EdgeLB API server can be reached.
	DefaultEdgeLBPath = "/"
	// DefaultEdgeLBPoolGroup is the name of the DC/OS service group in which to create EdgeLB pools by default.
	DefaultEdgeLBPoolGroup = "dcos-edgelb/pools"
	// DefaultEdgeLBScheme is the default scheme to use when communicating with the EdgeLB API server.
	DefaultEdgeLBScheme = "http"
	// DefaultResyncPeriod is the (default) maximum amount of time that may elapse between two consecutive synchronizations of Ingress/Service resources and the status of EdgeLB pools.
	DefaultResyncPeriod = 2 * time.Minute
	// KubeNodeTaskPattern is the pattern used to match Mesos tasks that correspond to Kubernetes nodes (either private or public).
	KubeNodeTaskPattern = "^kube-node-.*$"
	// KubeSystemNamespaceName holds the name of the "kube-system" namespace.
	KubeSystemNamespaceName = "kube-system"
)
