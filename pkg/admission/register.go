package admission

import (
	"encoding/base64"
	"fmt"
	"reflect"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/mesosphere/dklb/pkg/constants"
)

const (
	// dklbServiceName is the name of the service used to back the admission webhook.
	dklbServiceName = "dklb"
	// mutatingWebhookConfigurationResourceName is the name with which the admission webhook should be registered.
	mutatingWebhookConfigurationResourceName = "dklb"
	// webhookName is the name of the admission webhook.
	webhookName = "dklb.kubernetes.dcos.io"
)

var (
	// failurePolicy is the failure policy to use when registering the admission webhook.
	// A failure policy of "Ignore" means that a resource will be accepted whenever Kubernetes cannot reach the webhook.
	// The failure policy must not be "Fail" as that would make it impossible to create/update Ingress/Service resources whenever dklb is not reachable.
	// However, a failure policy of "Ignore" means that, in these scenarios, invalid Ingress/Service resources may be admitted.
	failurePolicy = admissionregistrationv1beta1.Ignore
)

// RegisterWebhook registers the admission webhook.
func RegisterWebhook(kubeClient kubernetes.Interface, tlsCaBundle string) error {
	// Base64-decode "tlsCaBundle", as we'll need the (raw) PEM-encoded string.
	caBundle, err := base64.StdEncoding.DecodeString(tlsCaBundle)
	if err != nil {
		return fmt.Errorf("failed to base64-decode the ca bundle: %v", err)
	}
	// Create the webhook configuration object containing the desired configuration
	desiredCfg := &admissionregistrationv1beta1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: mutatingWebhookConfigurationResourceName,
		},
		Webhooks: []admissionregistrationv1beta1.Webhook{
			{
				Name: webhookName,
				Rules: []admissionregistrationv1beta1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1beta1.OperationType{
							admissionregistrationv1beta1.Create,
							admissionregistrationv1beta1.Update,
						},
						Rule: admissionregistrationv1beta1.Rule{
							APIGroups: []string{
								corev1.SchemeGroupVersion.Group,
							},
							APIVersions: []string{
								corev1.SchemeGroupVersion.Version,
							},
							Resources: []string{
								"services",
							},
						},
					},
					{
						Operations: []admissionregistrationv1beta1.OperationType{
							admissionregistrationv1beta1.Create,
							admissionregistrationv1beta1.Update,
						},
						Rule: admissionregistrationv1beta1.Rule{
							APIGroups: []string{
								extsv1beta1.SchemeGroupVersion.Group,
							},
							APIVersions: []string{
								extsv1beta1.SchemeGroupVersion.Version,
							},
							Resources: []string{
								"ingresses",
							},
						},
					},
				},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Name:      dklbServiceName,
						Namespace: constants.KubeSystemNamespaceName,
						Path:      &admissionPath,
					},
					CABundle: caBundle,
				},
				FailurePolicy: &failurePolicy,
			},
		},
	}

	// Attempt to register the webhook.
	_, err = kubeClient.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Create(desiredCfg)
	if err == nil {
		return nil
	}
	if !errors.IsAlreadyExists(err) {
		// The webhook is not registered yet but we've got an unexpected error while registering it.
		return err
	}

	// At this point the webhook is already registered but the spec of the corresponding "MutatingWebhookConfiguration" resource may differ.

	// Read the latest version of the "MutatingWebhookConfiguration" resource.
	currentCfg, err := kubeClient.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Get(mutatingWebhookConfigurationResourceName, metav1.GetOptions{})
	if err != nil {
		// We've failed to fetch the latest version of the config
		return err
	}
	if reflect.DeepEqual(currentCfg.Webhooks, desiredCfg.Webhooks) {
		// If the specs match there's nothing to do
		return nil
	}

	// Attempt to update the resource by setting the resulting resource's ".spec" field according to the desired value.
	currentCfg.Webhooks = desiredCfg.Webhooks
	if _, err := kubeClient.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Update(currentCfg); err != nil {
		return err
	}
	return nil
}
