package constants

// EdgeLBPoolCreationStrategy represents a strategy used to create EdgeLB pools.
type EdgeLBPoolCreationStrategy string

const (
	// EdgeLBPoolCreationStrategyIfNotPresesent denotes the strategy that creates an EdgeLB pool whenever a pool with the same name doesn't already exist.
	EdgeLBPoolCreationStrategyIfNotPresesent = EdgeLBPoolCreationStrategy("IfNotPresent")
	// EdgeLBPoolCreationStrategyNever denotes the strategy that never creates an EdgeLB pool, expecting an existing one instead.
	EdgeLBPoolCreationStrategyNever = EdgeLBPoolCreationStrategy("Never")
	// PoolCreationStrategyOnce denotes the strategy that creates an EdgeLB pool only if a pool for a given Ingress/Service resource has never been created.
	EdgeLBPoolCreationStragegyOnce = EdgeLBPoolCreationStrategy("Once")
)

// EdgeLBLoadBalancerType represents the type (internal vs. external) of load-balancer to provision.
type EdgeLBLoadBalancerType string

const (
	// EdgeLBLoadBalancerTypeInternal denotes that an internal load-balancer should be provisioned.
	EdgeLBLoadBalancerTypeInternal = EdgeLBLoadBalancerType("internal")
	// EdgeLBLoadBalancerTypePublic denotes that a public load-balancer should be provisioned.
	EdgeLBLoadBalancerTypePublic = EdgeLBLoadBalancerType("public")
)

const (
	// annotationKeyPrefix is the prefix used by annotations that belong to the MKE domain.
	annotationKeyPrefix = "kubernetes.dcos.io/"
)

const (
	// EdgeLBIngressClassAnnotationKey is the key of the annotation that must be set on Ingress resources that are to be provisioned by EdgeLB.
	EdgeLBIngressClassAnnotationKey = "kubernetes.io/ingress.class"
	// EdgeLBIngressClassAnnotationValue is the value of the annotation that must be set on Ingress resources that are to be provisioned by EdgeLB.
	EdgeLBIngressClassAnnotationValue = "edgelb"

	// EdgeLBLoadBalancerTypeAnnotationKey is the key of the annotation that defines the type (internal vs. external) of load-balancer to provision.
	EdgeLBLoadBalancerTypeAnnotationKey = annotationKeyPrefix + "load-balancer-type"

	// EdgeLBPoolCreationStrategyAnnotationKey is the key of the annotation that holds the strategy to use for provisioning the target EdgeLB pool.
	EdgeLBPoolCreationStrategyAnnotationKey = annotationKeyPrefix + "edgelb-pool-creation-strategy"
	// EdgeLBPoolCpusAnnotationKey is the key of the annotation that holds the CPU request for the target EdgeLB pool.
	EdgeLBPoolCpusAnnotationKey = annotationKeyPrefix + "edgelb-pool-cpus"
	// EdgeLBPoolMemAnnotationKey is the key of the annotation that holds the memory request for the target EdgeLB pool.
	EdgeLBPoolMemAnnotationKey = annotationKeyPrefix + "edgelb-pool-mem"
	// EdgeLBPoolNameAnnotationKey is the key of the annotation that holds the name of the EdgeLB pool to use for provisioning a given Ingress/Service resource.
	EdgeLBPoolNameAnnotationKey = annotationKeyPrefix + "edgelb-pool-name"
	// EdgeLBPoolRoleAnnotationKey is the key of the annotation that holds the role of the target EdgeLB pool.
	EdgeLBPoolRoleAnnotationKey = annotationKeyPrefix + "edgelb-pool-role"
	// EdgeLBPoolSizeAnnotationKey is the key of the annotation that holds the size to request for the target EdgeLB pool.
	EdgeLBPoolSizeAnnotationKey = annotationKeyPrefix + "edgelb-pool-size"

	// EdgeLBPoolPortKey is the key of the annotation that holds the port to use as a frontend bind port by the target EdgeLB pool.
	EdgeLBPoolPortKey = annotationKeyPrefix + "edgelb-pool-port"

	// EdgeLBPoolPortMapKeyPrefix is the prefix of the key of the annotation that holds the port to use as a frontend bind port by the target EdgeLB pool.
	EdgeLBPoolPortMapKeyPrefix = annotationKeyPrefix + "edgelb-pool-portmap."
)
