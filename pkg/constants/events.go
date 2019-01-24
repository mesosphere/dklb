package constants

const (
	// ReasonNoDefaultBackendSpecified is the reason used in Kubernetes events emitted whenever an Ingress resource doesn't define a default backend.
	ReasonNoDefaultBackendSpecified = "NoDefaultBackendSpecified"
	// ReasonInvalidBackendService is the reason used in Kubernetes events emitted due to a missing or otherwise invalid Service resource referenced by an Ingress resource.
	ReasonInvalidBackendService = "InvalidBackendService"
	// ReasonInvalidAnnotations is the reason used in Kubernetes events emitted due to missing/invalid annotations on a Service/Ingress resource.
	ReasonInvalidAnnotations = "InvalidAnnotations"
	// ReasonTranslationError is the reason used in Kubernetes events emitted due to failed translation of a Service/Ingress resource into an EdgeLB pool.
	// TODO (@bcustodio) Understand if we should break this down into more fine-grained reasons (e.g. "InvalidSpec", "NetworkingError", ...).
	ReasonTranslationError = "TranslationError"
)
