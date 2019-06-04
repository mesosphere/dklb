package secretsreflector

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dcos/client-go/dcos"
	logtest "github.com/sirupsen/logrus/hooks/test"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	dklbcache "github.com/mesosphere/dklb/pkg/cache"
	"github.com/mesosphere/dklb/pkg/constants"
	cachetestutil "github.com/mesosphere/dklb/test/util/cache"
	secrettestutil "github.com/mesosphere/dklb/test/util/kubernetes/secret"
)

var (
	defaultTestKubeSecret = secrettestutil.DummySecretResource("namespace-1", "name-1", func(secret *corev1.Secret) {
		secret.Data[corev1.TLSCertKey] = []byte("aGVsbG8K")       // hello
		secret.Data[corev1.TLSPrivateKeyKey] = []byte("d29ybGQK") // world
	})
	defaultTestLogger, defaultTestLoggerHook = logtest.NewNullLogger()
)

func TestSecretReflector_translate(t *testing.T) {
	tests := []struct {
		description    string
		expectedError  error
		expectedResult *dcos.SecretsV1Secret
		secret         *corev1.Secret
	}{
		{
			description:   "should translate a secret",
			expectedError: nil,
			expectedResult: &dcos.SecretsV1Secret{
				Value: "aGVsbG8Kd29ybGQK",
			},
			secret: defaultTestKubeSecret.DeepCopy(),
		},
		{
			description:    "should fail with missing tls.crt field",
			expectedError:  errors.New("invalid secret: \"namespace-1/name-1\" does not contain tls.crt field"),
			expectedResult: nil,
			secret: secrettestutil.DummySecretResource("namespace-1", "name-1", func(secret *corev1.Secret) {
				secret.Data[corev1.TLSPrivateKeyKey] = []byte("aGVsbG8g")
			}),
		},
		{
			description:    "should fail with missing tls.key field",
			expectedError:  errors.New("invalid secret: \"namespace-1/name-1\" does not contain tls.key field"),
			expectedResult: nil,
			secret: secrettestutil.DummySecretResource("namespace-1", "name-1", func(secret *corev1.Secret) {
				secret.Data[corev1.TLSCertKey] = []byte("d29ybGQK")
			}),
		},
	}

	for _, test := range tests {
		t.Logf("test case: %s", test.description)

		// these tests don't require kubeCache and kubeClient
		// because they only test the Kubernetes secret to
		// DC/OS secret object translation
		sr := secretsReflector{
			logger: defaultTestLogger,
		}
		res, err := sr.translate(test.secret)

		assert.Equal(t, test.expectedError, err)
		assert.Equal(t, test.expectedResult, res)
	}
}

func TestSecretReflector_reflect(t *testing.T) {
	tests := []struct {
		description       string
		clusterName       string
		dcosSecretsClient DCOSSecretsClient
		dcosSecret        *dcos.SecretsV1Secret
		expectedError     error
		kubeClient        kubernetes.Interface
		kubeSecret        *corev1.Secret
	}{
		{
			description: "should create DC/OS Secret",
			clusterName: "cluster-1",
			dcosSecret: &dcos.SecretsV1Secret{
				Value: "hello\nworld\n",
			},
			dcosSecretsClient: &fakeDCOSSecretsClient{
				OnCreate: func(path string) error {
					if path != "cluster-1__namespace-1__name-1" {
						return fmt.Errorf("error expected 'cluster-1__namespace-1__name-1' got '%s'", path)
					}
					return nil
				},
			},
			expectedError: nil,
			kubeClient:    fake.NewSimpleClientset(defaultTestKubeSecret),
			kubeSecret:    defaultTestKubeSecret.DeepCopy(),
		},
		{
			description: "should update DC/OS Secret",
			clusterName: "cluster-1",
			dcosSecret: &dcos.SecretsV1Secret{
				Value: "hello\nworld\n",
			},
			dcosSecretsClient: &fakeDCOSSecretsClient{
				OnUpdate: func(path string) error {
					if path != "cluster-1__namespace-1__name-1" {
						return fmt.Errorf("error expected 'cluster-1__namespace-1__name-1' got '%s'", path)
					}
					return nil
				},
			},
			expectedError: nil,
			kubeClient:    fake.NewSimpleClientset(defaultTestKubeSecret),
			kubeSecret: secrettestutil.DummySecretResource("namespace-1", "name-1", func(secret *corev1.Secret) {
				secret.Data[corev1.TLSCertKey] = []byte("aGVsbG8g")
				secret.Data[corev1.TLSPrivateKeyKey] = []byte("d29ybGQK")
				secret.Annotations[constants.DklbSecretAnnotationKey] = "fake"
			}),
		},
		{
			description: "should fail with invalid DC/OS secret path",
			clusterName: "cluster-1",
			dcosSecret: &dcos.SecretsV1Secret{
				Value: "hello\nworld\n",
			},
			dcosSecretsClient: &fakeDCOSSecretsClient{
				OnCreate: func(path string) error {
					if path == "cluster-1__namespace-1__name-1" {
						return errors.New("fake error")
					}
					return nil
				},
			},
			expectedError: errors.New("failed to create DC/OS secret cluster-1__namespace-1__name-1: fake error"),
			kubeClient:    fake.NewSimpleClientset(defaultTestKubeSecret),
			kubeSecret:    defaultTestKubeSecret.DeepCopy(),
		},
		{
			description: "should not reflect secret because md5sum hash matches annotation",
			dcosSecret: &dcos.SecretsV1Secret{
				Value: "hello\nworld\n",
			},
			expectedError: nil,
			kubeSecret: secrettestutil.DummySecretResource("namespace-1", "name-1", func(secret *corev1.Secret) {
				secret.Data[corev1.TLSCertKey] = []byte("aGVsbG8g")
				secret.Data[corev1.TLSPrivateKeyKey] = []byte("d29ybGQK")
				secret.Annotations[constants.DklbSecretAnnotationKey] = "0f723ae7f9bf07744445e93ac5595156"
			}),
		},
		{
			description: "should fail to create DC/OS secret",
			dcosSecret: &dcos.SecretsV1Secret{
				Value: "hello\nworld\n",
			},
			expectedError: errors.New("failed to create DC/OS secret __namespace-1__name-1: fake error"),
			dcosSecretsClient: &fakeDCOSSecretsClient{
				OnCreate: func(path string) error { return errors.New("fake error") },
			},
			kubeSecret: defaultTestKubeSecret.DeepCopy(),
		},
		{
			description: "should fail to update DC/OS secret",
			dcosSecret: &dcos.SecretsV1Secret{
				Value: "hello\nworld\n",
			},
			expectedError: errors.New("failed to update DC/OS secret __namespace-1__name-1: fake error"),
			dcosSecretsClient: &fakeDCOSSecretsClient{
				OnUpdate: func(string) error { return errors.New("fake error") },
			},
			kubeSecret: secrettestutil.DummySecretResource("namespace-1", "name-1", func(secret *corev1.Secret) {
				secret.Data[corev1.TLSCertKey] = []byte("aGVsbG8g")
				secret.Data[corev1.TLSPrivateKeyKey] = []byte("d29ybGQK")
				secret.Annotations[constants.DklbSecretAnnotationKey] = "fake"
			}),
		},
		{
			description: "should fail to update Kubernetes secret dklb-hash annotation",
			dcosSecret: &dcos.SecretsV1Secret{
				Value: "hello\nworld\n",
			},
			expectedError:     errors.New("failed to update Kubernetes secret \"namespace-1/name-1\": secrets \"name-1\" not found"),
			dcosSecretsClient: newFakeDCOSSecretsClient(),
			kubeClient:        fake.NewSimpleClientset(),
			kubeSecret:        defaultTestKubeSecret.DeepCopy(),
		},
	}

	for _, test := range tests {
		t.Logf("test case: %s", test.description)

		sr := secretsReflector{
			dcosSecretsClient:     test.dcosSecretsClient,
			logger:                defaultTestLogger,
			kubeClient:            test.kubeClient,
			kubernetesClusterName: test.clusterName,
		}
		err := sr.reflect(test.kubeSecret, test.dcosSecret)

		assert.Equal(t, test.expectedError, err)
	}
}

func TestSecretReflector(t *testing.T) {
	emptyKubeSecret := secrettestutil.DummySecretResource("namespace-1", "name-1", func(secret *corev1.Secret) {
	})

	tests := []struct {
		description       string
		clusterName       string
		dcosSecretsClient DCOSSecretsClient
		dcosSecret        *dcos.SecretsV1Secret
		expectedError     error
		kubeCache         dklbcache.KubernetesResourceCache
		kubeClient        kubernetes.Interface
		kubeSecret        *corev1.Secret
	}{
		{
			description:       "should reflect a secret",
			clusterName:       "cluster-1",
			dcosSecretsClient: newFakeDCOSSecretsClient(),
			expectedError:     nil,
			kubeCache:         dklbcache.NewInformerBackedResourceCache(cachetestutil.NewFakeSharedInformerFactory(defaultTestKubeSecret)),
			kubeClient:        fake.NewSimpleClientset(defaultTestKubeSecret),
			kubeSecret:        defaultTestKubeSecret.DeepCopy(),
		},
		{
			description:       "should fail to retrieve a secret from cache",
			clusterName:       "cluster-1",
			dcosSecretsClient: newFakeDCOSSecretsClient(),
			expectedError:     errors.New("failed to get secret \"namespace-1/name-1\": secret \"name-1\" not found"),
			kubeCache:         dklbcache.NewInformerBackedResourceCache(cachetestutil.NewFakeSharedInformerFactory()),
			kubeSecret:        defaultTestKubeSecret.DeepCopy(),
		},
		{
			description:       "should fail to translate a secret",
			clusterName:       "cluster-1",
			dcosSecretsClient: newFakeDCOSSecretsClient(),
			expectedError:     errors.New("failed to translate secret: invalid secret: \"namespace-1/name-1\" does not contain tls.crt field"),
			kubeCache:         dklbcache.NewInformerBackedResourceCache(cachetestutil.NewFakeSharedInformerFactory(emptyKubeSecret)),
			kubeClient:        fake.NewSimpleClientset(emptyKubeSecret),
			kubeSecret:        emptyKubeSecret.DeepCopy(),
		},
	}

	for _, test := range tests {
		t.Logf("test case: %s", test.description)

		sr := secretsReflector{
			dcosSecretsClient:     test.dcosSecretsClient,
			logger:                defaultTestLogger,
			kubeCache:             test.kubeCache,
			kubeClient:            test.kubeClient,
			kubernetesClusterName: test.clusterName,
		}
		err := sr.Reflect(test.kubeSecret.Namespace, test.kubeSecret.Name)

		assert.Equal(t, test.expectedError, err)
	}
}

// Mostly for the test coverage increase :)
func TestSecretReflectorNew(t *testing.T) {
	t.Log("test case: constructor")

	clusterName := "cluster-1"
	dcosSecretsClient := newFakeDCOSSecretsClient()
	kubeCache := dklbcache.NewInformerBackedResourceCache(cachetestutil.NewFakeSharedInformerFactory(defaultTestKubeSecret))
	kubeClient := fake.NewSimpleClientset(defaultTestKubeSecret)

	sr := New(clusterName, dcosSecretsClient, kubeCache, kubeClient)
	assert.NotNil(t, sr)
}
