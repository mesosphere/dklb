package constants

const (
	// EdgeLBBackendBackup holds the value used as part of "miscStr" in order to instruct EdgeLB to use a given server only as "backup".
	EdgeLBBackendBackup = "backup"
	// EdgeLBBackendBalanceLeastConnections holds the value used to request the "leastconn" mode for a backend.
	EdgeLBBackendBalanceLeastConnections = "leastconn"
	// EdgeLBBackendInsecureSkipTLSVerify holds the value used as part of "miscStr" in order to disable verification of the TLS certificate presented by a given backend (if any).
	EdgeLBBackendInsecureSkipTLSVerify = "ssl verify none"
	// EdgeLBBackendTLSCheck holds the value used as part of "miscStr" in order to instruct EdgeLB to perform health-checks over TLS.
	EdgeLBBackendTLSCheck = "check-ssl"
	// EdgeLBCloudProviderPoolNamePrefix is the prefix used in the names of EdgeLB pools requesting a cloud load-balancer to be configured.
	EdgeLBCloudProviderPoolNamePrefix = "cloud"
	// EdgeLBFrontendBindAddress holds the bind address to use in EdgeLB frontends.
	EdgeLBFrontendBindAddress = "0.0.0.0"
	// EdgeLBRolePublic is the role used to schedule an EdgeLB pool onto a public DC/OS agent.
	EdgeLBRolePublic = "slave_public"
	// EdgeLBRolePrivate is the value used to schedule an EdgeLB pool onto a private DC/OS agent.
	EdgeLBRolePrivate = "*"
	// EdgeLBHostNetwork is the value used to schedule an EdgeLB pool onto the host network.
	EdgeLBHostNetwork = ""
	// EdgeLBPoolNameRegex is the regular expression used to validate the name of an EdgeLB pool.
	EdgeLBPoolNameRegex = "^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$"
)
