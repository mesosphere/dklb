package secretsreflector

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dcos/client-go/dcos"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	dklbcache "github.com/mesosphere/dklb/pkg/cache"
	"github.com/mesosphere/dklb/pkg/constants"
	dklbstrings "github.com/mesosphere/dklb/pkg/util/strings"
)

const (
	defaultSecretStore = "default"
	defaultTimeout     = 5 * time.Second
)

// SecretsReflector defines the interface exposed by this package.
type SecretsReflector interface {
	Reflect(uid, namespace, name string) error
}

// DCOSSecretsAPI defines the interface required by the SecretsReflector to manage DC/OS secrets.
type DCOSSecretsClient interface {
	CreateSecret(ctx context.Context, store string, pathToSecret string, secretsV1Secret dcos.SecretsV1Secret) (*http.Response, error)
	UpdateSecret(ctx context.Context, store string, pathToSecret string, secretsV1Secret dcos.SecretsV1Secret) (*http.Response, error)
}

// secretsReflector implements the SecretsReflector interface.
type secretsReflector struct {
	dcosSecretsClient DCOSSecretsClient
	kubeCache         dklbcache.KubernetesResourceCache
	kubeClient        kubernetes.Interface
	logger            log.FieldLogger
}

// New returns an instance of SecretsReflector.
func New(dcosSecretsClient DCOSSecretsClient, kubeCache dklbcache.KubernetesResourceCache, kubeClient kubernetes.Interface) SecretsReflector {
	return &secretsReflector{
		dcosSecretsClient: dcosSecretsClient,
		kubeCache:         kubeCache,
		kubeClient:        kubeClient,
		logger:            log.WithField("component", "secrets_reflector"),
	}
}

// Reflect reads Kubernetes secret with the provided namespace and name, translates it to a DC/OS secret
// and checks if it needs to be recreated in DC/OS.
func (s *secretsReflector) Reflect(uid, namespace, name string) error {
	// Get the secret from Kubernetes
	kubeSecret, err := s.kubeCache.GetSecret(namespace, name)
	if err != nil {
		return fmt.Errorf("failed to get secret \"%s/%s\": %s", namespace, name, err)
	}
	// Translate Kubernetes secret to a dcos secret
	dcosSecret, err := s.translate(kubeSecret)
	if err != nil {
		return fmt.Errorf("failed to translate secret: %s", err)
	}
	// Check if we need to update/create the dcos secret
	return s.reflect(uid, kubeSecret, dcosSecret)
}

// translate Kubernetes secret kubeSecret to structure expected by DC/OS or an error if it failed.
func (s *secretsReflector) translate(kubeSecret *corev1.Secret) (*dcos.SecretsV1Secret, error) {
	crt, ok := kubeSecret.Data[corev1.TLSCertKey]
	if !ok {
		err := fmt.Errorf("invalid secret: \"%s/%s\" does not contain %s field", kubeSecret.Namespace, kubeSecret.Name, corev1.TLSCertKey)
		return nil, err
	}
	key, ok := kubeSecret.Data[corev1.TLSPrivateKeyKey]
	if !ok {
		err := fmt.Errorf("invalid secret: \"%s/%s\" does not contain %s field", kubeSecret.Namespace, kubeSecret.Name, corev1.TLSPrivateKeyKey)
		return nil, err
	}
	// Concatenate certificate and private key and put the result
	// in an DC/OS Secret
	var sb strings.Builder
	sb.Write(crt)
	sb.Write(key)

	dcosSecret := &dcos.SecretsV1Secret{
		Value: sb.String(),
	}
	return dcosSecret, nil
}

// reflect checks if Kubernetes secret was updated by verifying if the MD5 hash of certificate
// and private key changed, and, if required proceeds to re-create the DC/OS secret.
func (s *secretsReflector) reflect(uid string, kubeSecret *corev1.Secret, dcosSecret *dcos.SecretsV1Secret) error {
	hashRaw := md5.Sum([]byte(dcosSecret.Value))
	hash := fmt.Sprintf("%x", hashRaw)
	expectedHash, withPreviousAnnotation := kubeSecret.Annotations[constants.DklbSecretAnnotationKey]
	// Hash did not change so we can exit now
	if len(dcosSecret.Value) > 0 && withPreviousAnnotation && hash == expectedHash {
		s.logger.Infof("no changes to secret \"%s/%s\" detected, skipping update", kubeSecret.Namespace, kubeSecret.Name)
		return nil
	}
	// Generate the DC/OS secret name
	dcosSecretName := ComputeDCOSSecretName(uid, kubeSecret.Name)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	// Check if we need to update or create the DC/OS secret
	if withPreviousAnnotation {
		_, err := s.dcosSecretsClient.UpdateSecret(ctx, defaultSecretStore, dcosSecretName, *dcosSecret)
		if err != nil {
			return fmt.Errorf("failed to update DC/OS secret %s: %s", dcosSecretName, err)
		}
	} else {
		resp, err := s.dcosSecretsClient.CreateSecret(ctx, defaultSecretStore, dcosSecretName, *dcosSecret)
		if err != nil {
			// if we get a 409 Conflict it means dklb was
			// restarted/redeployed and the secret was
			// already created previously, so we'll try to
			// update it
			if resp != nil && resp.StatusCode == http.StatusConflict {
				_, err := s.dcosSecretsClient.UpdateSecret(ctx, defaultSecretStore, dcosSecretName, *dcosSecret)
				if err != nil {
					return fmt.Errorf("failed to update DC/OS secret %s: %s", dcosSecretName, err)
				}
			} else {
				return fmt.Errorf("failed to create DC/OS secret %s: %s", dcosSecretName, err)
			}
		}
	}
	// Add the hash annotation to the Kubernetes secret and update it
	if kubeSecret.Annotations == nil {
		kubeSecret.Annotations = make(map[string]string)
	}
	kubeSecret.Annotations[constants.DklbSecretAnnotationKey] = hash
	s.logger.Infof("detected changes to secret \"%s/%s\": updating annotation", kubeSecret.Namespace, kubeSecret.Name)
	if _, err := s.kubeClient.CoreV1().Secrets(kubeSecret.Namespace).Update(kubeSecret); err != nil {
		return fmt.Errorf("failed to update Kubernetes secret \"%s/%s\": %s", kubeSecret.Namespace, kubeSecret.Name, err)
	}
	return nil
}

func ComputeDCOSSecretName(uid, kubeSecretName string) string {
	return fmt.Sprintf("%s__%s", uid, kubeSecretName)
}

func ComputeDCOSSecretFileName(dcosSecretName string) string {
	return dklbstrings.ReplaceForwardSlashes(dcosSecretName, "_")
}
