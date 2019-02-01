package cache_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"

	dklbcache "github.com/mesosphere/dklb/pkg/cache"
	cachetestutil "github.com/mesosphere/dklb/test/util/cache"
	ingresstestutil "github.com/mesosphere/dklb/test/util/kubernetes/ingress"
	servicetestutil "github.com/mesosphere/dklb/test/util/kubernetes/service"
)

var (
	// dummyConfigMap1 represents a dummy ConfigMap resource.
	dummyConfigMap1 = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace-1",
			Name:      "name-1",
		},
		Data: map[string]string{
			"foo": "bar",
		},
	}
	// dummyIngress1 represents a dummy Ingress resource.
	dummyIngress1 = ingresstestutil.DummyIngressResource("namespace-1", "name-1", func(ingress *extsv1beta1.Ingress) {
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
)

// TestHasSynced tests the "HasSynced" function.
func TestHasSynced(t *testing.T) {
	cache := dklbcache.NewKubernetesResourceCache(cachetestutil.NewFakeSharedInformerFactory(dummyConfigMap1, dummyIngress1, dummyService1))
	// "fakeSharedInformerFactory" already waits for caches to be synced, so this should be trivially true.
	assert.True(t, cache.HasSynced())
}

// TestGetConfigMap tests the "GetConfigMap" function.
func TestGetConfigMap(t *testing.T) {
	cache := dklbcache.NewKubernetesResourceCache(cachetestutil.NewFakeSharedInformerFactory(dummyConfigMap1))
	tests := []struct {
		description    string
		namespace      string
		name           string
		expectedResult *corev1.ConfigMap
		expectedError  error
	}{
		{
			description:    "get an existing configmap resource",
			namespace:      dummyConfigMap1.Namespace,
			name:           dummyConfigMap1.Name,
			expectedResult: dummyConfigMap1,
			expectedError:  nil,
		},
		{
			description:    "get an inexistent configmap resource",
			namespace:      "foo",
			name:           "bar",
			expectedResult: nil,
			expectedError:  kubeerrors.NewNotFound(schema.GroupResource{Group: "", Resource: "configmap"}, "bar"),
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		res, err := cache.GetConfigMap(test.namespace, test.name)
		if test.expectedError != nil {
			assert.Equal(t, test.expectedError, err)
		} else {
			assert.Equal(t, test.expectedResult, res)
		}
	}
}

// TestGetIngress tests the "GetIngress" function.
func TestGetIngress(t *testing.T) {
	cache := dklbcache.NewKubernetesResourceCache(cachetestutil.NewFakeSharedInformerFactory(dummyIngress1))
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
	cache := dklbcache.NewKubernetesResourceCache(cachetestutil.NewFakeSharedInformerFactory(dummyService1))
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
