package admission

import (
	corev1 "k8s.io/api/core/v1"

	translatorapi "github.com/mesosphere/dklb/pkg/translator/api"
)

// validateAndMutateService validates the current Service resource.
// If "previousSvc" is not nil, the transition between "previousSvc" and "currentSvc" is also validated.
func (w *Webhook) validateAndMutateService(currentSvc, previousSvc *corev1.Service) (*corev1.Service, error) {
	// If the current Service resource is not of type LoadBalancer, and if we're not transitioning from a Service resource of type LoadBalancer, there is nothing to validate/mutate.
	if currentSvc.Spec.Type != corev1.ServiceTypeLoadBalancer && (previousSvc == nil || previousSvc.Spec.Type != corev1.ServiceTypeLoadBalancer) {
		return currentSvc, nil
	}

	// Create a deep copy of "currentSvc" so we can mutate it as required.
	mutatedSvc := currentSvc.DeepCopy()

	// Grab the EdgeLB pool configuration object from the Service resource.
	currentSpec, err := translatorapi.GetServiceEdgeLBPoolSpec(mutatedSvc)
	if err != nil {
		return nil, err
	}

	// Mutate the Service resource with the expanded, validated EdgeLB pool configuration object.
	if err := translatorapi.SetServiceEdgeLBPoolSpec(mutatedSvc, currentSpec); err != nil {
		return nil, err
	}

	// If the current operation is not an UPDATE operation, or if the Service is being "converted" to a Service of type LoadBalancer, there's nothing else to do.
	if previousSvc == nil || previousSvc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return mutatedSvc, nil
	}

	// At this point we know that the current operation is an UPDATE operation.
	// Hence, we must also validate any changes to the EdgeLB pool configuration object.

	// Parse the previous EdgeLB pool configruration object.
	// There should be no errors as the resource has been previously admitted, but we still propagate them just in case.
	previousSpec, err := translatorapi.GetServiceEdgeLBPoolSpec(previousSvc)
	if err != nil {
		return nil, err
	}

	// Validate the transition between the previous and the current EdgeLB pool configuration objects.
	if err := currentSpec.ValidateTransition(previousSpec); err != nil {
		return nil, err
	}
	return mutatedSvc, nil
}
