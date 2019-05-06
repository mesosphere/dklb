package secretsreflector

import (
	"context"
	"crypto/md5"
	"encoding/base64"
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
)

const (
	defaultSecretStore = "default"
	defaultTimeout     = 5 * time.Second
)

// SecretsReflector defines the interface exposed by this package.
type SecretsReflector interface {
	Reflect(namespace, name string) error
}

// DCOSSecretsAPI defines the interface required by the
// SecretsReflector to manage DC/OS secrets.
type DCOSSecretsClient interface {
	CreateSecret(ctx context.Context, store string, pathToSecret string, secretsV1Secret dcos.SecretsV1Secret) (*http.Response, error)
	DeleteSecret(ctx context.Context, store string, pathToSecret string) (*http.Response, error)
}

// secretsReflector implements SecretsReflector interface.
type secretsReflector struct {
	dcosSecretsClient     DCOSSecretsClient
	kubeCache             dklbcache.KubernetesResourceCache
	kubeClient            kubernetes.Interface
	kubernetesClusterName string
	logger                log.FieldLogger
}

// New returns an instance of SecretsReflector.
func New(kubernetesClusterName string, dcosSecretsClient DCOSSecretsClient, kubeCache dklbcache.KubernetesResourceCache, kubeClient kubernetes.Interface) SecretsReflector {
	return &secretsReflector{
		dcosSecretsClient:     dcosSecretsClient,
		kubeCache:             kubeCache,
		kubeClient:            kubeClient,
		kubernetesClusterName: kubernetesClusterName,
		logger:                log.WithField("component", "secrets_reflector"),
	}
}

// Reflect reads kubernetes secret with name from namespace,
// translates it to a DCOS secret and checks to if it needs to
// re-create it DC/OS.
func (s *secretsReflector) Reflect(namespace, name string) error {
	// get secret from kubernetes
	kubeSecret, err := s.kubeCache.GetSecret(namespace, name)
	if err != nil {
		s.logger.Errorf("failed to get secret %s/%s", namespace, name)
		return err
	}
	// translate kubernetes secret to a dcos secret
	dcosSecret, err := s.translate(kubeSecret)
	if err != nil {
		s.logger.Errorf("failed to translate secret: %s", err)
		return err
	}
	// check if we need to update/create the dcos secret
	return s.reflect(kubeSecret, dcosSecret)
}

// translate kubernetes secret kubeSecret to structure expected by
// DCOS or an error if it failed.
func (s *secretsReflector) translate(kubeSecret *corev1.Secret) (*dcos.SecretsV1Secret, error) {
	// get base64 representation of kubernetes secret tls.crt and tls.pem fields
	crt64, ok := kubeSecret.Data["tls.crt"]
	if !ok {
		err := fmt.Errorf("invalid secret: %s/%s does not contain tls.crt field", kubeSecret.Namespace, kubeSecret.Name)
		return nil, err
	}
	key64, ok := kubeSecret.Data["tls.key"]
	if !ok {
		err := fmt.Errorf("invalid secret: %s/%s does not contain tls.key field", kubeSecret.Namespace, kubeSecret.Name)
		return nil, err
	}
	// base64 decode tls.crt and tls.pem fields
	crt, err := base64.StdEncoding.DecodeString(string(crt64))
	if err != nil {
		err = fmt.Errorf("invalid secret: error decoding %s/%s tls.crt field: %v", kubeSecret.Namespace, kubeSecret.Name, err)
		return nil, err
	}
	key, err := base64.StdEncoding.DecodeString(string(key64))
	if err != nil {
		err = fmt.Errorf("invalid secret: error decoding %s/%s tls.key field: %v", kubeSecret.Namespace, kubeSecret.Name, err)
		return nil, err
	}
	// concatenate crt and private key and put the result in an
	// DCOS Secret
	var sb strings.Builder
	sb.Write(crt)
	sb.Write(key)

	dcosSecret := &dcos.SecretsV1Secret{
		Value: sb.String(),
	}
	return dcosSecret, nil
}

// reflect checks if kubernetes secret was updated by verifying if the
// MD5 hash of certificate and private key changed and if required
// proceeds to re-create a DCOS secret to match it.
func (s *secretsReflector) reflect(kubeSecret *corev1.Secret, dcosSecret *dcos.SecretsV1Secret) error {
	hashRaw := md5.Sum([]byte(dcosSecret.Value))
	hash := fmt.Sprintf("%x", hashRaw)
	expectedHash := kubeSecret.Annotations[constants.DklbSecretAnnotationKey]
	// hash did not change so we can exit now
	if len(hash) > 0 && len(expectedHash) > 0 && hash == expectedHash {
		s.logger.Infof("no changes to secret %s/%s detected, skipping update", kubeSecret.Namespace, kubeSecret.Name)
		return nil
	}
	// generate the DCOS secret name
	dcosSecretName := fmt.Sprintf("%s__%s__%s", s.kubernetesClusterName, kubeSecret.Namespace, kubeSecret.Name)

	ctx, fn := context.WithTimeout(context.Background(), defaultTimeout)
	defer fn()
	// delete the secret from DCOS secret store and ignore any
	// errors (ex: AlreadyExists or NotFound)
	s.logger.Infof("(re)creating DCOS secret %s", dcosSecretName)
	s.dcosSecretsClient.DeleteSecret(ctx, defaultSecretStore, dcosSecretName)
	// create the DCOS secret
	_, err := s.dcosSecretsClient.CreateSecret(ctx, defaultSecretStore, dcosSecretName, *dcosSecret)
	if err != nil {
		s.logger.Errorf("failed to (re)create DCOS secret %s: %s", dcosSecretName, err)
		return err
	}
	// add the hash annotation to the kubernetes secret and update it
	if kubeSecret.Annotations == nil {
		kubeSecret.Annotations = make(map[string]string)
	}
	kubeSecret.Annotations[constants.DklbSecretAnnotationKey] = hash
	s.logger.Infof("detected changes to secret %s/%s: updating annotation", kubeSecret.Namespace, kubeSecret.Name)
	if _, err := s.kubeClient.CoreV1().Secrets(kubeSecret.Namespace).Update(kubeSecret); err != nil {
		s.logger.Errorf("failed to update secret %s/%s: %s", kubeSecret.Namespace, kubeSecret.Name, err)
		return err
	}
	return nil
}
