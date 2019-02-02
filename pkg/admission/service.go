package admission

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/mesosphere/dklb/pkg/translator"
)

// validateAndMutateService validates the current "Service" resource.
// If "previousSvc" is specified, the transition between "previousSvc" and "currentSvc" is also validated.
// It returns a copy of the current "Service" resource mutated in order to explicitly set all the supported annotations.
func (w *Webhook) validateAndMutateService(currentSvc, previousSvc *corev1.Service) (*corev1.Service, error) {
	// Create a deep copy of "currentSvc" so we can mutate it as required.
	mutatedSvc := currentSvc.DeepCopy()

	// If the current "Service" resource is not of type "LoadBalancer", there is nothing to validate/mutate.
	if currentSvc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return mutatedSvc, nil
	}

	// Compute the translation options for the current "Service" resource, in order to make sure the resource is valid.
	currentOptions, err := translator.ComputeServiceTranslationOptions(w.clusterName, currentSvc)
	if err != nil {
		return nil, err
	}

	// If there is a previous version of the current "Service" resource, we must also validate any changes that may have been performed.
	if previousSvc != nil && previousSvc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		// Compute the translation options for the previous "Service" resource.
		// There should be no errors as the resource has been previously admitted, but we still propagate them just in case.
		previousOptions, err := translator.ComputeServiceTranslationOptions(w.clusterName, previousSvc)
		if err != nil {
			return nil, err
		}
		// Validate the update operation.
		if err := translator.ValidateServiceTranslationOptionsUpdate(previousOptions, currentOptions); err != nil {
			return nil, err
		}
	}

	// At this point we know that the current "Service" resource is valid.

	// Explicitly set the value of each annotation on the resource and return.
	setDefaultsOnService(mutatedSvc, currentOptions)
	return mutatedSvc, nil
}
