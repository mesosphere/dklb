package secretsreflector

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dcos/client-go/dcos"
	logtest "github.com/sirupsen/logrus/hooks/test"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	dklbcache "github.com/mesosphere/dklb/pkg/cache"
	"github.com/mesosphere/dklb/pkg/constants"
	cachetestutil "github.com/mesosphere/dklb/test/util/cache"
	secrettestutil "github.com/mesosphere/dklb/test/util/kubernetes/secret"
)

var (
	defaultTestKubeSecret = secrettestutil.DummySecretResource("namespace-1", "name-1", func(secret *corev1.Secret) {
		secret.Data["tls.crt"] = []byte("aGVsbG8gd29ybGQK")
		secret.Data["tls.key"] = []byte("aGVsbG8gd29ybGQK")
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
				Value: "hello world\nhello world\n",
			},
			secret: defaultTestKubeSecret.DeepCopy(),
		},
		{
			description:    "should fail with missing tls.crt field",
			expectedError:  errors.New("invalid secret: namespace-1/name-1 does not contain tls.crt field"),
			expectedResult: nil,
			secret: secrettestutil.DummySecretResource("namespace-1", "name-1", func(secret *corev1.Secret) {
				secret.Data["tls.key"] = []byte("aGVsbG8gd29ybGQK")
			}),
		},
		{
			description:    "should fail with missing tls.key field",
			expectedError:  errors.New("invalid secret: namespace-1/name-1 does not contain tls.key field"),
			expectedResult: nil,
			secret: secrettestutil.DummySecretResource("namespace-1", "name-1", func(secret *corev1.Secret) {
				secret.Data["tls.crt"] = []byte("aGVsbG8gd29ybGQK")
			}),
		},
	}

	for _, test := range tests {
		t.Logf("test case: %s", test.description)

		// these tests don't require kubecache and kubeclient
		// because they only test the kubernetes secret to
		// DCOS secret object translation
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
			description: "should reflect secret",
			clusterName: "cluster-1",
			dcosSecret: &dcos.SecretsV1Secret{
				Value: "hello world\nhello world\n",
			},
			dcosSecretsClient: &fakeDCOSSecretsClient{
				ValidatePath: func(path string) error {
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
			description: "should fail with invalid DCOS secret path",
			clusterName: "cluster-1",
			dcosSecret: &dcos.SecretsV1Secret{
				Value: "hello world\nhello world\n",
			},
			dcosSecretsClient: &fakeDCOSSecretsClient{
				ValidatePath: func(path string) error {
					if path == "cluster-1__namespace-1__name-1" {
						return fmt.Errorf("error: expected 'cluster-1__namespace-1__name-1' got '%s'", path)
					}
					return nil
				},
			},
			expectedError: errors.New("error: expected 'cluster-1__namespace-1__name-1' got 'cluster-1__namespace-1__name-1'"),
			kubeClient:    fake.NewSimpleClientset(defaultTestKubeSecret),
			kubeSecret:    defaultTestKubeSecret.DeepCopy(),
		},
		{
			description: "should not reflect secret because md5sum hash matches annotation",
			dcosSecret: &dcos.SecretsV1Secret{
				Value: "hello world\nhello world\n",
			},
			expectedError: nil,
			kubeSecret: secrettestutil.DummySecretResource("namespace-1", "name-1", func(secret *corev1.Secret) {
				secret.Data["tls.crt"] = []byte("aGVsbG8gd29ybGQK")
				secret.Data["tls.key"] = []byte("aGVsbG8gd29ybGQK")
				secret.Annotations[constants.DklbSecretAnnotationKey] = "cdb22973649bc8bb322a144796b42163"
			}),
		},
		{
			description: "should fail to create DCOS secret",
			dcosSecret: &dcos.SecretsV1Secret{
				Value: "hello world\nhello world\n",
			},
			expectedError: errors.New("fake"),
			dcosSecretsClient: &fakeDCOSSecretsClient{
				Err: errors.New("fake"),
			},
			kubeSecret: defaultTestKubeSecret.DeepCopy(),
		},
		{
			description: "should fail to update kubernetes secret dklb-hash annotation",
			dcosSecret: &dcos.SecretsV1Secret{
				Value: "hello world\nhello world\n",
			},
			expectedError:     kubeerrors.NewNotFound(schema.GroupResource{Group: "", Resource: "secrets"}, defaultTestKubeSecret.Name),
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
			expectedError:     kubeerrors.NewNotFound(schema.GroupResource{Group: "", Resource: "secret"}, defaultTestKubeSecret.Name),
			kubeCache:         dklbcache.NewInformerBackedResourceCache(cachetestutil.NewFakeSharedInformerFactory()),
			kubeSecret:        defaultTestKubeSecret.DeepCopy(),
		},
		{
			description:       "should fail to translate a secret",
			clusterName:       "cluster-1",
			dcosSecretsClient: newFakeDCOSSecretsClient(),
			expectedError:     errors.New("invalid secret: namespace-1/name-1 does not contain tls.crt field"),
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
