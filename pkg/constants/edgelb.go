package constants

const (
	// EdgeLBRoleInternal is the role used to schedule an EdgeLB pool onto a private DC/OS agent.
	EdgelbRoleInternal = "*"
	// EdgeLBRolePublic is the role used to schedule an EdgeLB pool onto a public DC/OS agent.
	EdgelbRolePublic = "slave_public"
)
