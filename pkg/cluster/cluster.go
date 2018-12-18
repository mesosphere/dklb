package cluster

var (
	// KubernetesClusterFrameworkName is the name of the Mesos framework that corresponds to the current Kubernetes cluster.
	// Used as the value of "pool.haproxy.backend.service.frameworkName" in pool configurations.
	KubernetesClusterFrameworkName string
)
