package constants

const (
	// annotationKeyPrefix is the prefix used by annotations that belong to the MKE domain.
	annotationKeyPrefix = "kubernetes.dcos.io/"
)

const (
	// EdgeLBIngressClassAnnotationKey is the key of the annotation that selects the ingress controller used to satisfy a given Ingress resource.
	// This is the same annotation that is used by Ingress controllers such as "kubernetes/ingress-nginx" or "containous/traefik".
	EdgeLBIngressClassAnnotationKey = "kubernetes.io/ingress.class"
	// EdgeLBIngressClassAnnotationValue is the value that must be used for the annotation that selects the ingress controller used to satisfy a given Ingress resource.
	// Only Ingres resources having this as the value of the aforementioned annotation will be provisioned using EdgeLB.
	EdgeLBIngressClassAnnotationValue = "edgelb"

	// DklbConfigAnnotationKey is the key of the annotation that holds the target EdgeLB pool's specification for a given Service/Ingress resource.
	DklbConfigAnnotationKey = annotationKeyPrefix + "dklb-config"

	// DklbPaused is the key of the annotation that holds whether a given Service/Ingress resource is currently paused.
	// While this annotation is set to "true" on a given Ingress/Service resource, the only actions that dlkb will perform on the resource is validation and defaulting (via the admission webhook).
	// It is used mostly to facilitate end-to-end testing, as it allows to simulate certain scenarios that would otherwise be very hard to simulate.
	// It also allows for performing end-to-end testing on the admission webhook without the need for provisioning EdgeLB pools.
	DklbPaused = annotationKeyPrefix + "dklb-paused"

	// DklbSecretAnnotationKey is the key of the annotation that holds the MD5 hash of the base64 decoded certificate and private key.
	DklbSecretAnnotationKey = annotationKeyPrefix + "dklb-hash"
)
