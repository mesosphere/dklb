package constants

const (
	// EdgeLBRoleInternal is the role used to schedule an EdgeLB pool onto a private DC/OS agent.
	EdgeLBRoleInternal = "*"
	// EdgeLBRolePublic is the role used to schedule an EdgeLB pool onto a public DC/OS agent.
	EdgeLBRolePublic = "slave_public"
)
