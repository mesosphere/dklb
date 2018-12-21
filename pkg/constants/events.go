package constants

const (
	// ReasonInvalidAnnotations is the reason used in Kubernetes events emitted due to missing/invalid annotations on a Service/Ingress resource.
	ReasonInvalidAnnotations = "InvalidAnnotations"
	// ReasonTranslationError is the reason used in Kubernetes events emitted due to failed translation of a Service/Ingress resource into an EdgeLB pool.
	// TODO (@bcustodio) Understand if we should break this down into more fine-grained reasons (e.g. "InvalidSpec", "NetworkingError", ...).
	ReasonTranslationError = "TranslationError"
)
