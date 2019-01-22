package admission

import (
	extsv1beta1 "k8s.io/api/extensions/v1beta1"

	"github.com/mesosphere/dklb/pkg/translator"
	kubernetesutil "github.com/mesosphere/dklb/pkg/util/kubernetes"
)

// validateAndMutateIngress validates the current "Ingress" resource.
// If "previousIng" is specified, the transition between "previousIng" and "currentIng" is also validated.
// It returns a copy of the current "Ingress" resource mutated in order to explicitly set all the supported annotations.
func (w *Webhook) validateAndMutateIngress(currentIng, previousIng *extsv1beta1.Ingress) (*extsv1beta1.Ingress, error) {
	// Create a deep copy of "currentIng" so we can mutate it as required.
	mutatedIng := currentIng.DeepCopy()

	// If the current "Ingress" resource is not to be provisioned by EdgeLB, there is nothing to validate/mutate.
	if !kubernetesutil.IsEdgeLBIngress(currentIng) {
		return mutatedIng, nil
	}

	// Compute the translation options for the current "Ingress" resource, in order to make sure the resource is valid.
	currentOptions, err := translator.ComputeIngressTranslationOptions(w.clusterName, currentIng)
	if err != nil {
		return nil, err
	}

	// If there is a previous version of the current "Ingress" resource, we must also validate any changes that may have been performed.
	if previousIng != nil {
		// Compute the translation options for the previous "Ingress" resource.
		// There should be no errors as the resource has been previously admitted, but we still propagate them just in case.
		previousOptions, err := translator.ComputeIngressTranslationOptions(w.clusterName, previousIng)
		if err != nil {
			return nil, err
		}
		// Validate the update operation.
		if err := translator.ValidateIngressTranslationOptionsUpdate(previousOptions, currentOptions); err != nil {
			return nil, err
		}
	}

	// At this point we know that the current "Ingress" resource is valid.

	// Initialize the "Annotations" field of "currentIng" as necessary.
	if mutatedIng.Annotations == nil {
		mutatedIng.Annotations = make(map[string]string)
	}

	// Explicitly set the value of each annotation on the resource and return.
	setDefaultsOnIngress(mutatedIng, currentOptions)
	return mutatedIng, nil
}
