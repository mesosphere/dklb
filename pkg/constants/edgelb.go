package constants

const (
	// EdgeLBBackendBalanceLeastConnections holds the value used to request the "leastconn" mode for a backend.
	EdgeLBBackendBalanceLeastConnections = "leastconn"
	// EdgeLBFrontendBindAddress holds the bind address to use in EdgeLB frontends.
	EdgeLBFrontendBindAddress = "0.0.0.0"
	// EdgeLBRolePublic is the role used to schedule an EdgeLB pool onto a public DC/OS agent.
	EdgeLBRolePublic = "slave_public"
)
