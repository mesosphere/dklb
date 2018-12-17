package constants

const (
	// EdgeLBIngressClassAnnotationKey is the key of the annotation that must be set on Ingress resources that are to be provisioned by EdgeLB.
	EdgeLBIngressClassAnnotationKey = "kubernetes.io/ingress.class"
	// EdgeLBIngressClassAnnotationValue is the value of the annotation that must be set on Ingress resources that are to be provisioned by EdgeLB.
	EdgeLBIngressClassAnnotationValue = "edgelb"
)
