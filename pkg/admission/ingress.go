package admission

import (
	extsv1beta1 "k8s.io/api/extensions/v1beta1"

	translatorapi "github.com/mesosphere/dklb/pkg/translator/api"
	kubernetesutil "github.com/mesosphere/dklb/pkg/util/kubernetes"
)

// validateAndMutateIngress validates the current Ingress resource.
// If "previousIng" is not nil, the transition between "previousIng" and "currentIng" is also validated.
func (w *Webhook) validateAndMutateIngress(currentIng, previousIng *extsv1beta1.Ingress) (*extsv1beta1.Ingress, error) {
	// If the current Ingress resource is not annotated to be provisioned by EdgeLB, and we're not transitioning from an Ingress resource annotated to be provisioned by EdgeLB, there is nothing to validate/mutate.
	if !kubernetesutil.IsEdgeLBIngress(currentIng) && (previousIng == nil || !kubernetesutil.IsEdgeLBIngress(previousIng)) {
		return currentIng, nil
	}

	// Create a deep copy of "currentIng" so we can mutate it as required.
	mutatedIng := currentIng.DeepCopy()

	// Grab the EdgeLB pool configuration object from the Ingress resource.
	currentSpec, err := translatorapi.GetIngressEdgeLBPoolSpec(mutatedIng)
	if err != nil {
		return nil, err
	}

	// Mutate the Ingress resource with the expanded, validated EdgeLB pool configuration object.
	if err := translatorapi.SetIngressEdgeLBPoolSpec(mutatedIng, currentSpec); err != nil {
		return nil, err
	}

	// If the current operation is not an UPDATE operation, or if the Ingress is being "converted" to an EdgeLB ingress, there's nothing else to do.
	if previousIng == nil || !kubernetesutil.IsEdgeLBIngress(previousIng) {
		return mutatedIng, nil
	}

	// At this point we know that the current operation is an UPDATE operation.
	// Hence, we must also validate any changes to the EdgeLB pool configuration object.

	// Parse the previous EdgeLB pool configuration object.
	// There should be no errors as the resource has been previously admitted, but we still propagate them just in case.
	previousSpec, err := translatorapi.GetIngressEdgeLBPoolSpec(previousIng)
	if err != nil {
		return nil, err
	}

	// Validate the transition between the previous and the current EdgeLB pool configuration objects.
	if err := currentSpec.ValidateTransition(previousSpec); err != nil {
		return nil, err
	}
	return mutatedIng, nil
}
