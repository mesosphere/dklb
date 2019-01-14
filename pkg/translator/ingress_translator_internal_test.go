package translator

import (
	"testing"
	"time"

	"github.com/mesosphere/dcos-edge-lb/models"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/util/pointers"
	cachetestutil "github.com/mesosphere/dklb/test/util/cache"
	edgelbpooltestutil "github.com/mesosphere/dklb/test/util/edgelb/pool"
	ingresstestutil "github.com/mesosphere/dklb/test/util/kubernetes/ingress"
	servicetestutil "github.com/mesosphere/dklb/test/util/kubernetes/service"
)

var (
	// dummyIngress1 is a dummy Kubernetes Ingress resource.
	dummyIngress1 = ingresstestutil.DummyIngressResource("foo", "bar", func(ingress *extsv1beta1.Ingress) {
		ingress.Annotations = map[string]string{
			constants.EdgeLBIngressClassAnnotationKey: constants.EdgeLBIngressClassAnnotationValue,
		}
		ingress.Spec.Backend = &extsv1beta1.IngressBackend{
			ServiceName: dummyIngress1BackendFoo.Name,
			ServicePort: intstr.FromInt(int(dummyIngress1BackendFoo.Spec.Ports[0].Port)),
		}
		ingress.Spec.Rules = []extsv1beta1.IngressRule{
			{
				Host: "foo.bar",
				IngressRuleValue: extsv1beta1.IngressRuleValue{
					HTTP: &extsv1beta1.HTTPIngressRuleValue{
						Paths: []extsv1beta1.HTTPIngressPath{
							{
								Path: "/bar",
								Backend: extsv1beta1.IngressBackend{
									ServiceName: dummyIngress1BackendBar.Name,
									ServicePort: intstr.FromInt(int(dummyIngress1BackendBar.Spec.Ports[0].Port)),
								},
							},
							{
								Path: "/baz",
								Backend: extsv1beta1.IngressBackend{
									ServiceName: dummyIngress1BackendBaz.Name,
									ServicePort: intstr.FromInt(int(dummyIngress1BackendBaz.Spec.Ports[0].Port)),
								},
							},
						},
					},
				},
			},
		}
	})
	// dummyIngress1WithoutDummyIngress1BackendBaz is a dummy Kubernetes Ingress resource built from "dummyIngress1" by removing the "dummyIngress1BackendBaz" backend..
	dummyIngress1WithoutDummyIngress1BackendBaz = ingresstestutil.DummyIngressResource("foo", "bar", func(ingress *extsv1beta1.Ingress) {
		ingress.Annotations = map[string]string{
			constants.EdgeLBIngressClassAnnotationKey: constants.EdgeLBIngressClassAnnotationValue,
		}
		ingress.Spec.Backend = &extsv1beta1.IngressBackend{
			ServiceName: dummyIngress1BackendFoo.Name,
			ServicePort: intstr.FromInt(int(dummyIngress1BackendFoo.Spec.Ports[0].Port)),
		}
		ingress.Spec.Rules = []extsv1beta1.IngressRule{
			{
				Host: "foo.bar",
				IngressRuleValue: extsv1beta1.IngressRuleValue{
					HTTP: &extsv1beta1.HTTPIngressRuleValue{
						Paths: []extsv1beta1.HTTPIngressPath{
							{
								Path: "/bar",
								Backend: extsv1beta1.IngressBackend{
									ServiceName: dummyIngress1BackendBar.Name,
									ServicePort: intstr.FromInt(int(dummyIngress1BackendBar.Spec.Ports[0].Port)),
								},
							},
							{
								Path: "/baz",
								Backend: extsv1beta1.IngressBackend{
									ServiceName: dummyIngress1BackendBar.Name,
									ServicePort: intstr.FromInt(int(dummyIngress1BackendBar.Spec.Ports[0].Port)),
								},
							},
						},
					},
				},
			},
		}
	})
	// deletedDummyIngress1 is a dummy Kubernetes Ingress resource that has been marked for deletion.
	deletedDummyIngress1 = ingresstestutil.DummyIngressResource("foo", "bar", func(ingress *extsv1beta1.Ingress) {
		ingress.Annotations = map[string]string{
			constants.EdgeLBIngressClassAnnotationKey: constants.EdgeLBIngressClassAnnotationValue,
		}
		deletionTimestamp := metav1.NewTime(time.Now())
		ingress.DeletionTimestamp = &deletionTimestamp
		ingress.Spec.Backend = &extsv1beta1.IngressBackend{
			ServiceName: dummyIngress1BackendFoo.Name,
			ServicePort: intstr.FromInt(int(dummyIngress1BackendFoo.Spec.Ports[0].Port)),
		}
		ingress.Spec.Rules = []extsv1beta1.IngressRule{
			{
				Host: "foo.bar",
				IngressRuleValue: extsv1beta1.IngressRuleValue{
					HTTP: &extsv1beta1.HTTPIngressRuleValue{
						Paths: []extsv1beta1.HTTPIngressPath{
							{
								Path: "/bar",
								Backend: extsv1beta1.IngressBackend{
									ServiceName: dummyIngress1BackendBar.Name,
									ServicePort: intstr.FromInt(int(dummyIngress1BackendBar.Spec.Ports[0].Port)),
								},
							},
							{
								Path: "/baz",
								Backend: extsv1beta1.IngressBackend{
									ServiceName: dummyIngress1BackendBaz.Name,
									ServicePort: intstr.FromInt(int(dummyIngress1BackendBaz.Spec.Ports[0].Port)),
								},
							},
						},
					},
				},
			},
		}
	})

	// dummyIngress1BackendFoo is one of the Service resources used as a backend in "dummyIngress1".
	dummyIngress1BackendFoo = servicetestutil.DummyServiceResource("foo", "foo", func(service *corev1.Service) {
		service.Spec.Ports = []corev1.ServicePort{
			{
				NodePort: 32000,
				Port:     8080,
			},
		}
		service.Spec.Type = corev1.ServiceTypeNodePort
	})
	// dummyIngress1BackendBar is one of the Service resources used as a backend in "dummyIngress1".
	dummyIngress1BackendBar = servicetestutil.DummyServiceResource("foo", "bar", func(service *corev1.Service) {
		service.Spec.Ports = []corev1.ServicePort{
			{
				NodePort: 32100,
				Port:     8080,
			},
		}
		service.Spec.Type = corev1.ServiceTypeNodePort
	})
	// dummyIngress1BackendBaz is one of the Service resources used as a backend in "dummyIngress1".
	dummyIngress1BackendBaz = servicetestutil.DummyServiceResource("foo", "baz", func(service *corev1.Service) {
		service.Spec.Ports = []corev1.ServicePort{
			{
				NodePort: 32200,
				Port:     8080,
			},
		}
		service.Spec.Type = corev1.ServiceTypeNodePort
	})

	// dummyIngress1TranslationOptions represents a set of translation options that can be used to translate "dummyIngress1".
	dummyIngress1TranslationOptions = IngressTranslationOptions{
		BaseTranslationOptions: BaseTranslationOptions{
			EdgeLBPoolName: "baz",
			EdgeLBPoolRole: "custom_role",
			EdgeLBPoolCpus: resource.MustParse("5010203m"),
			EdgeLBPoolMem:  resource.MustParse("3724Mi"),
			EdgeLBPoolSize: 3,
		},
		EdgeLBPoolPort: 18080,
	}

	// backendForIngress1Foo is the computed (expected) backend for path "/bar" of "dummyIngress1.
	backendForDummyIngress1Bar = computeEdgeLBBackendForIngressBackend(testClusterName, dummyIngress1, dummyIngress1.Spec.Rules[0].HTTP.Paths[0].Backend, dummyIngress1BackendBar.Spec.Ports[0].NodePort)
	// backendForIngress1Foo is the computed (expected) backend for path "/baz" of "dummyIngress1.
	backendForDummyIngress1Baz = computeEdgeLBBackendForIngressBackend(testClusterName, dummyIngress1, dummyIngress1.Spec.Rules[0].HTTP.Paths[1].Backend, dummyIngress1BackendBaz.Spec.Ports[0].NodePort)
	// defaultBackendForDummyIngress1 is the computed (expected) default backend for "dummyIngress1".
	defaultBackendForDummyIngress1 = computeEdgeLBBackendForIngressBackend(testClusterName, dummyIngress1, *dummyIngress1.Spec.Backend, dummyIngress1BackendFoo.Spec.Ports[0].NodePort)
	// frontendForDummyIngress1 is the computed (expected) frontend for "dummyIngress1".
	frontendForDummyIngress1 = computeEdgeLBFrontendForIngress(testClusterName, dummyIngress1, dummyIngress1TranslationOptions)
)

func TestCreateEdgeLBPoolObjectForIngress(t *testing.T) {
	tests := []struct {
		description       string
		resources         []runtime.Object
		ingress           *extsv1beta1.Ingress
		options           IngressTranslationOptions
		expectedName      string
		expectedRole      string
		expectedCpus      float64
		expectedMem       int32
		expectedSize      int
		expectedBackends  []*models.V2Backend
		expectedFrontends []*models.V2Frontend
	}{
		{
			description: "create an edgelb pool based on valid translation options",
			resources: []runtime.Object{
				dummyIngress1BackendFoo,
				dummyIngress1BackendBar,
				dummyIngress1BackendBaz,
			},
			ingress:      dummyIngress1,
			options:      dummyIngress1TranslationOptions,
			expectedName: "baz",
			expectedRole: "custom_role",
			expectedCpus: 5010.203,
			expectedMem:  3724,
			expectedSize: 3,
			expectedBackends: []*models.V2Backend{
				backendForDummyIngress1Bar,
				backendForDummyIngress1Baz,
				defaultBackendForDummyIngress1,
			},
			expectedFrontends: []*models.V2Frontend{
				frontendForDummyIngress1,
			},
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		// Create a mock KubernetesResourceCache.
		kubeCache := cachetestutil.NewFakeKubernetesResourceCache(test.resources...)
		// Create a new instance of the Ingress translator.
		translator := NewIngressTranslator(testClusterName, test.ingress, test.options, kubeCache, nil)
		// Compute the mapping between Ingress backends and Service node ports.
		m, err := translator.computeIngressBackendNodePortMap()
		// Make sure no error occurred.
		assert.NoError(t, err)
		// Create the target EdgeLB pool object.
		pool := translator.createEdgeLBPoolObject(m)
		// Make sure the resulting EdgeLB pool object meets our expectations.
		assert.Equal(t, test.expectedName, pool.Name)
		assert.Equal(t, test.expectedRole, pool.Role)
		assert.Equal(t, test.expectedCpus, pool.Cpus)
		assert.Equal(t, test.expectedMem, pool.Mem)
		assert.Equal(t, pointers.NewInt32(int32(test.expectedSize)), pool.Count)
		assert.Equal(t, test.expectedBackends, pool.Haproxy.Backends)
		assert.Equal(t, test.expectedFrontends, pool.Haproxy.Frontends)
	}
}

// TestUpdateEdgeLBPoolObjectForIngress tests the "updateEdgeLBPoolObject" function.
func TestUpdateEdgeLBPoolObjectForIngress(t *testing.T) {
	tests := []struct {
		description        string
		resources          []runtime.Object
		ingress            *extsv1beta1.Ingress
		options            IngressTranslationOptions
		pool               *models.V2Pool
		expectedWasChanged bool
		expectedBackends   []*models.V2Backend
		expectedFrontends  []*models.V2Frontend
	}{
		{
			// Test that a pool that is in "in sync" with the Ingress resource's spec is detected as not requiring an update.
			description: "pool that is \"in sync\" with the Ingress resource's spec is detected as not requiring an update",
			resources: []runtime.Object{
				dummyIngress1BackendFoo,
				dummyIngress1BackendBar,
				dummyIngress1BackendBaz,
			},
			ingress: dummyIngress1,
			options: dummyIngress1TranslationOptions,
			pool: edgelbpooltestutil.DummyEdgeLBPool("baz", func(p *models.V2Pool) {
				p.Haproxy.Backends = []*models.V2Backend{
					backendForDummyIngress1Bar,
					backendForDummyIngress1Baz,
					defaultBackendForDummyIngress1,
				}
				p.Haproxy.Frontends = []*models.V2Frontend{
					frontendForDummyIngress1,
				}
			}),
			expectedWasChanged: false,
			expectedBackends: []*models.V2Backend{
				backendForDummyIngress1Bar,
				backendForDummyIngress1Baz,
				defaultBackendForDummyIngress1,
			},
			expectedFrontends: []*models.V2Frontend{
				frontendForDummyIngress1,
			},
		},
		{
			// Test that a pool that is in "in sync" with a deleted Ingress resource's spec is detected as requiring an update.
			description: "pool that is \"in sync\" with a deleted Ingress resource's spec is detected as requiring an update",
			resources: []runtime.Object{
				dummyIngress1BackendFoo,
				dummyIngress1BackendBar,
				dummyIngress1BackendBaz,
			},
			ingress: deletedDummyIngress1,
			options: dummyIngress1TranslationOptions,
			pool: edgelbpooltestutil.DummyEdgeLBPool("baz", func(p *models.V2Pool) {
				p.Haproxy.Backends = []*models.V2Backend{
					backendForDummyIngress1Bar,
					backendForDummyIngress1Baz,
					defaultBackendForDummyIngress1,
				}
				p.Haproxy.Frontends = []*models.V2Frontend{
					frontendForDummyIngress1,
				}
			}),
			expectedWasChanged: true,
			expectedBackends:   []*models.V2Backend{},
			expectedFrontends:  []*models.V2Frontend{},
		},
		{
			// Test that a pool for which a backend was manually changed is detected as requiring an update.
			description: "pool for which a backend was manually changed is detected as requiring an update",
			resources: []runtime.Object{
				dummyIngress1BackendFoo,
				dummyIngress1BackendBar,
				dummyIngress1BackendBaz,
			},
			ingress: dummyIngress1,
			options: dummyIngress1TranslationOptions,
			pool: edgelbpooltestutil.DummyEdgeLBPool("baz", func(p *models.V2Pool) {
				// Replicate "backendForDummyIngress1Bar".
				copyOfBackendForDummyIngress1Bar := computeEdgeLBBackendForIngressBackend(testClusterName, dummyIngress1, dummyIngress1.Spec.Rules[0].HTTP.Paths[0].Backend, dummyIngress1BackendBar.Spec.Ports[0].NodePort)
				// Change the target port.
				copyOfBackendForDummyIngress1Bar.Services[0].Endpoint.Port = 10101
				p.Haproxy.Backends = []*models.V2Backend{
					// Will have to be replaced by "backendForDummyIngress1Bar".
					copyOfBackendForDummyIngress1Bar,
					backendForDummyIngress1Baz,
					defaultBackendForDummyIngress1,
				}
				p.Haproxy.Frontends = []*models.V2Frontend{
					frontendForDummyIngress1,
				}
			}),
			expectedWasChanged: true,
			expectedBackends: []*models.V2Backend{
				backendForDummyIngress1Bar,
				backendForDummyIngress1Baz,
				defaultBackendForDummyIngress1,
			},
			expectedFrontends: []*models.V2Frontend{
				frontendForDummyIngress1,
			},
		},
		{
			// Test that a pool for which a frontend was manually changed is detected as requiring an update.
			description: "pool for which a frontend was manually changed is detected as requiring an update",
			resources: []runtime.Object{
				dummyIngress1BackendFoo,
				dummyIngress1BackendBar,
				dummyIngress1BackendBaz,
			},
			ingress: dummyIngress1,
			options: dummyIngress1TranslationOptions,
			pool: edgelbpooltestutil.DummyEdgeLBPool("baz", func(p *models.V2Pool) {
				// Replicate "frontendForDummyIngress1".
				copyOfFrontendForDummyIngress1 := computeEdgeLBFrontendForIngress(testClusterName, dummyIngress1, dummyIngress1TranslationOptions)
				// Change the bind port.
				copyOfFrontendForDummyIngress1.BindPort = pointers.NewInt32(90)
				p.Haproxy.Backends = []*models.V2Backend{
					backendForDummyIngress1Bar,
					backendForDummyIngress1Baz,
					defaultBackendForDummyIngress1,
				}
				p.Haproxy.Frontends = []*models.V2Frontend{
					// Will have to be replaced by "frontendForDummyIngress1".
					copyOfFrontendForDummyIngress1,
				}
			}),
			expectedWasChanged: true,
			expectedBackends: []*models.V2Backend{
				backendForDummyIngress1Bar,
				backendForDummyIngress1Baz,
				defaultBackendForDummyIngress1,
			},
			expectedFrontends: []*models.V2Frontend{
				frontendForDummyIngress1,
			},
		},
		{
			// Test that a pool with existing backends/frontends but no backends/frontends for the current Ingress resource is detected as requiring an update, and has all expected backends and frontends after being updated in-place.
			description: "pool with existing backends/frontends but no backends/frontends for the current Ingress resource is detected as requiring an update",
			resources: []runtime.Object{
				dummyIngress1BackendFoo,
				dummyIngress1BackendBar,
				dummyIngress1BackendBaz,
			},
			ingress: dummyIngress1,
			options: dummyIngress1TranslationOptions,
			pool: edgelbpooltestutil.DummyEdgeLBPool("baz", func(p *models.V2Pool) {
				p.Haproxy.Backends = []*models.V2Backend{
					preExistingBackend1,
					preExistingBackend2,
					preExistingBackend3,
				}
				p.Haproxy.Frontends = []*models.V2Frontend{
					preExistingFrontend1,
					preExistingFrontend2,
				}
			}),
			expectedWasChanged: true,
			expectedBackends: []*models.V2Backend{
				preExistingBackend1,
				preExistingBackend2,
				preExistingBackend3,
				backendForDummyIngress1Bar,
				backendForDummyIngress1Baz,
				defaultBackendForDummyIngress1,
			},
			expectedFrontends: []*models.V2Frontend{
				preExistingFrontend1,
				preExistingFrontend2,
				frontendForDummyIngress1,
			},
		},
		{
			// Test that a pool that was "in sync" with an Ingress resource for which a backend was removed is detected as requiring an update.
			description: "pool that was \"in sync\" with an Ingress resource for which a backend was removed is detected as requiring an update",
			resources: []runtime.Object{
				dummyIngress1BackendFoo,
				dummyIngress1BackendBar,
				dummyIngress1BackendBaz,
			},
			ingress: dummyIngress1WithoutDummyIngress1BackendBaz,
			options: dummyIngress1TranslationOptions,
			pool: edgelbpooltestutil.DummyEdgeLBPool("baz", func(p *models.V2Pool) {
				p.Haproxy.Backends = []*models.V2Backend{
					preExistingBackend1,
					preExistingBackend2,
					preExistingBackend3,
					backendForDummyIngress1Bar,
					// Must not be present in the final list of EdgeLB backends.
					backendForDummyIngress1Baz,
					defaultBackendForDummyIngress1,
				}
				p.Haproxy.Frontends = []*models.V2Frontend{
					preExistingFrontend2,
					preExistingFrontend1,
					frontendForDummyIngress1,
				}
			}),
			expectedWasChanged: true,
			expectedBackends: []*models.V2Backend{
				preExistingBackend1,
				preExistingBackend2,
				preExistingBackend3,
				backendForDummyIngress1Bar,
				defaultBackendForDummyIngress1,
			},
			expectedFrontends: []*models.V2Frontend{
				preExistingFrontend2,
				preExistingFrontend1,
				computeEdgeLBFrontendForIngress(testClusterName, dummyIngress1WithoutDummyIngress1BackendBaz, dummyIngress1TranslationOptions),
			},
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		// Create a mock KubernetesResourceCache.
		kubeCache := cachetestutil.NewFakeKubernetesResourceCache(test.resources...)
		// Create a new instance of the Ingress translator.
		translator := NewIngressTranslator(testClusterName, test.ingress, test.options, kubeCache, nil)
		// Compute the mapping between Ingress backends and Service node ports.
		m, err := translator.computeIngressBackendNodePortMap()
		// Make sure no error occurred.
		assert.NoError(t, err)
		// Update the EdgeLB pool object in-place.
		wasChanged, _ := translator.updateEdgeLBPoolObject(test.pool, m)
		// Check that the need for a pool update was adequately detected.
		assert.Equal(t, test.expectedWasChanged, wasChanged)
		// Check that all expected backends are present.
		assert.Equal(t, test.expectedBackends, test.pool.Haproxy.Backends)
		// Check that all expected frontends are present.
		assert.Equal(t, test.expectedFrontends, test.pool.Haproxy.Frontends)
	}
}
