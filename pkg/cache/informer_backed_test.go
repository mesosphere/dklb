package cache_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"

	dklbcache "github.com/mesosphere/dklb/pkg/cache"
	cachetestutil "github.com/mesosphere/dklb/test/util/cache"
	ingresstestutil "github.com/mesosphere/dklb/test/util/kubernetes/ingress"
	secrettestutil "github.com/mesosphere/dklb/test/util/kubernetes/secret"
	servicetestutil "github.com/mesosphere/dklb/test/util/kubernetes/service"
)

var (
	// dummyIngress1 represents a dummy Ingress resource.
	dummyIngress1 = ingresstestutil.DummyEdgeLBIngressResource("namespace-1", "name-1", func(ingress *extsv1beta1.Ingress) {
		ingress.Spec.Backend = &extsv1beta1.IngressBackend{
			ServiceName: "foo",
			ServicePort: intstr.FromInt(80),
		}
	})
	// dummyService1 represents a dummy Service resource.
	dummyService1 = servicetestutil.DummyServiceResource("namespace-1", "name-1", func(service *corev1.Service) {
		service.Spec.Ports = []corev1.ServicePort{
			{
				Port: 80,
			},
		}
	})
	// dummySecret1 represents a dummy Secret resource. the base64 data is 'hello world'
	dummySecret1 = secrettestutil.DummySecretResource("namespace-1", "name-1", func(secret *corev1.Secret) {
		secret.Data["tls.crt"] = []byte("aGVsbG8gd29ybGQK")
		secret.Data["tls.key"] = []byte("aGVsbG8gd29ybGQK")
	})
)

// TestHasSynced tests the "HasSynced" function.
func TestHasSynced(t *testing.T) {
	cache := dklbcache.NewInformerBackedResourceCache(cachetestutil.NewFakeSharedInformerFactory(dummyIngress1, dummyService1))
	// "fakeSharedInformerFactory" already waits for caches to be synced, so this should be trivially true.
	assert.True(t, cache.HasSynced())
}

// TestGetIngress tests the "GetIngress" function.
func TestGetIngress(t *testing.T) {
	cache := dklbcache.NewInformerBackedResourceCache(cachetestutil.NewFakeSharedInformerFactory(dummyIngress1))
	tests := []struct {
		description    string
		namespace      string
		name           string
		expectedResult *extsv1beta1.Ingress
		expectedError  error
	}{
		{
			description:    "get an existing ingress resource",
			namespace:      dummyIngress1.Namespace,
			name:           dummyIngress1.Name,
			expectedResult: dummyIngress1,
			expectedError:  nil,
		},
		{
			description:    "get an inexistent ingress resource",
			namespace:      "foo",
			name:           "bar",
			expectedResult: nil,
			expectedError:  kubeerrors.NewNotFound(schema.GroupResource{Group: "extensions", Resource: "ingress"}, "bar"),
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		res, err := cache.GetIngress(test.namespace, test.name)
		if test.expectedError != nil {
			assert.Equal(t, test.expectedError, err)
		} else {
			assert.Equal(t, test.expectedResult, res)
		}
	}
}

// TestGetService tests the "GetService" function.
func TestGetService(t *testing.T) {
	cache := dklbcache.NewInformerBackedResourceCache(cachetestutil.NewFakeSharedInformerFactory(dummyService1))
	tests := []struct {
		description    string
		namespace      string
		name           string
		expectedResult *corev1.Service
		expectedError  error
	}{
		{
			description:    "get an existing service resource",
			namespace:      dummyService1.Namespace,
			name:           dummyService1.Name,
			expectedResult: dummyService1,
			expectedError:  nil,
		},
		{
			description:    "get an inexistent service resource",
			namespace:      "foo",
			name:           "bar",
			expectedResult: nil,
			expectedError:  kubeerrors.NewNotFound(schema.GroupResource{Group: "", Resource: "service"}, "bar"),
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		res, err := cache.GetService(test.namespace, test.name)
		if test.expectedError != nil {
			assert.Equal(t, test.expectedError, err)
		} else {
			assert.Equal(t, test.expectedResult, res)
		}
	}
}

func TestGetSecret(t *testing.T) {
	cache := dklbcache.NewInformerBackedResourceCache(cachetestutil.NewFakeSharedInformerFactory(dummySecret1))
	tests := []struct {
		description    string
		namespace      string
		name           string
		expectedResult *corev1.Secret
		expectedError  error
	}{
		{
			description:    "get an existing secret resource",
			namespace:      dummySecret1.Namespace,
			name:           dummySecret1.Name,
			expectedResult: dummySecret1,
			expectedError:  nil,
		},
		{
			description:    "get an non-existent secret resource",
			namespace:      "foo",
			name:           "bar",
			expectedResult: nil,
			expectedError:  kubeerrors.NewNotFound(schema.GroupResource{Group: "", Resource: "secret"}, "bar"),
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		res, err := cache.GetSecret(test.namespace, test.name)
		if test.expectedError != nil {
			assert.Equal(t, test.expectedError, err)
		} else {
			assert.Equal(t, test.expectedResult, res)
		}
	}
}
