package translator

import (
	"testing"
	"time"

	"github.com/mesosphere/dcos-edge-lb/models"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/mesosphere/dklb/pkg/util/pointers"
	edgelbmanagertestutil "github.com/mesosphere/dklb/test/util/edgelb/manager"
	edgelbpooltestutil "github.com/mesosphere/dklb/test/util/edgelb/pool"
	"github.com/mesosphere/dklb/test/util/kubernetes/service"
)

const (
	// testClusterName is the value used as the name of the Kubernetes cluster in the current file.
	testClusterName = "dev/kubernetes01"
	// testEdgeLBPoolGroup is the value used as the name of the DC/OS service group in which to create EdgeLB pools.
	testEdgeLBPoolGroup = "foo/bar/dcos-edgelb/pools"
)

var (
	// serviceExposingPort80 is a dummy Kubernetes Service resource that exposes port 80.
	serviceExposingPort80 = service.DummyServiceResource("foo", "bar", func(service *v1.Service) {
		service.Spec.Ports = []v1.ServicePort{
			{
				Name:       "http-1",
				Protocol:   v1.ProtocolTCP,
				Port:       80,
				NodePort:   34567,
				TargetPort: intstr.FromInt(8080),
			},
		}
		service.Spec.Type = v1.ServiceTypeLoadBalancer
	})
	// deletedServiceExposingPort80 is a dummy Kubernetes Service resource that exposes port 80 but has been marked for deletion.
	deletedServiceExposingPort80 = service.DummyServiceResource("foo", "bar", func(service *v1.Service) {
		deletionTimestamp := metav1.NewTime(time.Now())
		service.ObjectMeta.DeletionTimestamp = &deletionTimestamp
		service.Spec.Ports = []v1.ServicePort{
			{
				Name:       "http-1",
				Protocol:   v1.ProtocolTCP,
				Port:       80,
				NodePort:   34567,
				TargetPort: intstr.FromInt(8080),
			},
		}
		service.Spec.Type = v1.ServiceTypeLoadBalancer
	})
	// serviceTranslationOptionsForPort80 is a set of translation options that maps a Service's port 80 to EdgeLB frontend bind port 10080.
	serviceTranslationOptionsForPort80 = ServiceTranslationOptions{
		BaseTranslationOptions: BaseTranslationOptions{
			EdgeLBPoolName: "baz",
			EdgeLBPoolRole: "custom_role",
			EdgeLBPoolCpus: resource.MustParse("5010203m"),
			EdgeLBPoolMem:  resource.MustParse("3724Mi"),
			EdgeLBPoolSize: 3,
		},
		EdgeLBPoolPortMap: map[int32]int32{
			80: 10080,
		},
	}
	// backendForServiceExposingPort80 is the computed (expected) backend for port 80 of serviceExposingPort80.
	backendForServiceExposingPort80 = computeBackendForServicePort(testClusterName, serviceExposingPort80, serviceExposingPort80.Spec.Ports[0])
	// frontendForServiceExposingPort80 is the computed (expected) frontend for port 80 of serviceExposingPort80.
	frontendForServiceExposingPort80 = computeFrontendForServicePort(testClusterName, serviceExposingPort80, serviceExposingPort80.Spec.Ports[0], serviceTranslationOptionsForPort80)
	// preExistingBackend1 is used to represent a pre-existing EdgeLB backend.
	preExistingBackend1 = &models.V2Backend{
		Name: "pre-existing-backend-1",
	}
	// preExistingBackend2 is used to represent another pre-existing EdgeLB backend.
	preExistingBackend2 = &models.V2Backend{
		Name: "pre-existing-backend-2",
	}
	// preExistingBackend1 is used to represent yet another pre-existing EdgeLB backend.
	preExistingBackend3 = &models.V2Backend{
		Name: "pre-existing-backend-3",
	}
	// preExistingFrontend1 is used to represent a pre-existing EdgeLB frontend.
	preExistingFrontend1 = &models.V2Frontend{
		Name: "pre-existing-frontend-1",
	}
	// preExistingFrontend2 is used to represent another pre-existing EdgeLB frontend.
	preExistingFrontend2 = &models.V2Frontend{
		Name: "pre-existing-frontend-2",
	}
)

func TestCreateEdgeLBPoolObject(t *testing.T) {
	tests := []struct {
		description       string
		service           *v1.Service
		options           ServiceTranslationOptions
		expectedName      string
		expectedRole      string
		expectedCpus      float64
		expectedMem       int32
		expectedSize      int
		expectedBackends  []*models.V2Backend
		expectedFrontends []*models.V2Frontend
	}{
		{
			description:  "create an edgelb pool based on valid translattion options",
			service:      serviceExposingPort80,
			options:      serviceTranslationOptionsForPort80,
			expectedName: serviceTranslationOptionsForPort80.EdgeLBPoolName,
			expectedRole: serviceTranslationOptionsForPort80.EdgeLBPoolRole,
			expectedCpus: 5010.203,
			expectedMem:  3724,
			expectedSize: serviceTranslationOptionsForPort80.EdgeLBPoolSize,
			expectedBackends: []*models.V2Backend{
				backendForServiceExposingPort80,
			},
			expectedFrontends: []*models.V2Frontend{
				frontendForServiceExposingPort80,
			},
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		// Create and customize a mock EdgeLB manager.
		manager := new(edgelbmanagertestutil.MockEdgeLBManager)
		manager.On("PoolGroup").Return(testEdgeLBPoolGroup)
		pool := NewServiceTranslator(testClusterName, test.service, test.options, manager).createEdgeLBPoolObject()
		assert.Equal(t, testEdgeLBPoolGroup, *pool.Namespace)
		assert.Equal(t, test.expectedName, pool.Name)
		assert.Equal(t, test.expectedRole, pool.Role)
		assert.Equal(t, test.expectedCpus, pool.Cpus)
		assert.Equal(t, test.expectedMem, pool.Mem)
		assert.Equal(t, pointers.NewInt32(int32(test.expectedSize)), pool.Count)
		assert.Equal(t, test.expectedBackends, pool.Haproxy.Backends)
		assert.Equal(t, test.expectedFrontends, pool.Haproxy.Frontends)
	}
}

// TestUpdateEdgeLBPoolObject tests the "updateEdgeLBPoolObject" function.
func TestUpdateEdgeLBPoolObject(t *testing.T) {
	tests := []struct {
		description        string
		service            *v1.Service
		options            ServiceTranslationOptions
		pool               *models.V2Pool
		expectedWasChanged bool
		expectedBackends   []*models.V2Backend
		expectedFrontends  []*models.V2Frontend
	}{
		{
			// Test that a pool that is in "in sync" with the Service resource's spec is detected as not requiring an update.
			description: "pool that is \"in sync\" with the Service resource's spec is detected as not requiring an update",
			service:     serviceExposingPort80,
			options:     serviceTranslationOptionsForPort80,
			pool: edgelbpooltestutil.DummyEdgeLBPool("baz", func(p *models.V2Pool) {
				p.Haproxy.Backends = []*models.V2Backend{
					backendForServiceExposingPort80,
				}
				p.Haproxy.Frontends = []*models.V2Frontend{
					frontendForServiceExposingPort80,
				}
			}),
			expectedWasChanged: false,
			expectedBackends: []*models.V2Backend{
				backendForServiceExposingPort80,
			},
			expectedFrontends: []*models.V2Frontend{
				frontendForServiceExposingPort80,
			},
		},
		{
			// Test that a pool that was "in sync" with a deleted Service resource is detected as requiring an update.
			description: "pool that was \"in sync\" with a deleted Service resource is detected as requiring an update",
			service:     deletedServiceExposingPort80,
			options:     serviceTranslationOptionsForPort80,
			pool: edgelbpooltestutil.DummyEdgeLBPool("baz", func(p *models.V2Pool) {
				p.Haproxy.Backends = []*models.V2Backend{
					backendForServiceExposingPort80,
				}
				p.Haproxy.Frontends = []*models.V2Frontend{
					frontendForServiceExposingPort80,
				}
			}),
			expectedWasChanged: true,
			expectedBackends:   []*models.V2Backend{},
			expectedFrontends:  []*models.V2Frontend{},
		},
		{
			// Test that a pool for which a backend was manually changed is detected as requiring an update.
			description: "pool for which a backend was manually changed is detected as requiring an update",
			service:     serviceExposingPort80,
			options:     serviceTranslationOptionsForPort80,
			pool: edgelbpooltestutil.DummyEdgeLBPool("baz", func(p *models.V2Pool) {
				// Change the target port for the backend.
				backend := computeBackendForServicePort(testClusterName, serviceExposingPort80, serviceExposingPort80.Spec.Ports[0])
				backend.Services[0].Endpoint.Port = 10101
				p.Haproxy.Backends = []*models.V2Backend{
					// Will have to be replaced by "backendForServiceExposingPort80".
					backend,
				}
				p.Haproxy.Frontends = []*models.V2Frontend{
					frontendForServiceExposingPort80,
				}
			}),
			expectedWasChanged: true,
			expectedBackends: []*models.V2Backend{
				backendForServiceExposingPort80,
			},
			expectedFrontends: []*models.V2Frontend{
				frontendForServiceExposingPort80,
			},
		},
		{
			// Test that a pool for which a frontend was manually changed is detected as requiring an update.
			description: "pool for which a frontend was manually changed is detected as requiring an update",
			service:     serviceExposingPort80,
			options:     serviceTranslationOptionsForPort80,
			pool: edgelbpooltestutil.DummyEdgeLBPool("baz", func(p *models.V2Pool) {
				p.Haproxy.Backends = []*models.V2Backend{
					backendForServiceExposingPort80,
				}
				// Change the bind port for the frontend.
				frontend := computeFrontendForServicePort(testClusterName, serviceExposingPort80, serviceExposingPort80.Spec.Ports[0], serviceTranslationOptionsForPort80)
				frontend.BindPort = pointers.NewInt32(10101)
				p.Haproxy.Frontends = []*models.V2Frontend{
					// Will have to be replaced by "frontendForServiceExposingPort80".
					frontend,
				}
			}),
			expectedWasChanged: true,
			expectedBackends: []*models.V2Backend{
				backendForServiceExposingPort80,
			},
			expectedFrontends: []*models.V2Frontend{
				frontendForServiceExposingPort80,
			},
		},
		{
			// Test that a pool with existing backends/frontends but no backends/frontends for the current Service resource is detected as requiring an update, and has all expected backends and frontends after being updated in-place.
			description: "pool with existing backends/frontends but no backends/frontends for the current Service resource is detected as requiring an update",
			service:     serviceExposingPort80,
			options:     serviceTranslationOptionsForPort80,
			pool: edgelbpooltestutil.DummyEdgeLBPool("baz", func(p *models.V2Pool) {
				p.Haproxy.Backends = []*models.V2Backend{
					// Will have to exist (and come before "backendForServiceExposingPort80") in the end.
					preExistingBackend1,
					preExistingBackend2,
					preExistingBackend3,
				}
				p.Haproxy.Frontends = []*models.V2Frontend{
					// Will have to exist (and come before "frontendForServiceExposingPort80") in the end.
					preExistingFrontend1,
					preExistingFrontend2,
				}
			}),
			expectedWasChanged: true,
			expectedBackends: []*models.V2Backend{
				preExistingBackend1,
				preExistingBackend2,
				preExistingBackend3,
				backendForServiceExposingPort80,
			},
			expectedFrontends: []*models.V2Frontend{
				preExistingFrontend1,
				preExistingFrontend2,
				frontendForServiceExposingPort80,
			},
		},
		{
			// Test that a pool with existing backends/frontends is detected as requiring an update after a port is removed from the Service resource.
			description: "pool with existing backends/frontends is detected as requiring an update after a port is removed from the Service resource",
			service:     serviceExposingPort80,
			options:     serviceTranslationOptionsForPort80,
			pool: edgelbpooltestutil.DummyEdgeLBPool("baz", func(p *models.V2Pool) {
				p.Haproxy.Backends = []*models.V2Backend{
					preExistingBackend1,
					preExistingBackend2,
					preExistingBackend3,
					backendForServiceExposingPort80,
					// Add an extra backend owned by "serviceExposingPort80" to simulate removal of a port.
					computeBackendForServicePort(testClusterName, serviceExposingPort80, v1.ServicePort{
						NodePort:   34568,
						Port:       90,
						Protocol:   v1.ProtocolTCP,
						TargetPort: intstr.FromInt(8080),
					}),
				}
				p.Haproxy.Frontends = []*models.V2Frontend{
					preExistingFrontend1,
					preExistingFrontend2,
					frontendForServiceExposingPort80,
					// Add an extra frontend owned by "serviceExposingPort80" to simulate removal of a port.
					computeFrontendForServicePort(testClusterName, serviceExposingPort80, v1.ServicePort{
						NodePort:   34568,
						Port:       90,
						Protocol:   v1.ProtocolTCP,
						TargetPort: intstr.FromInt(8080),
					}, ServiceTranslationOptions{}),
				}
			}),
			expectedWasChanged: true,
			expectedBackends: []*models.V2Backend{
				preExistingBackend1,
				preExistingBackend2,
				preExistingBackend3,
				backendForServiceExposingPort80,
			},
			expectedFrontends: []*models.V2Frontend{
				preExistingFrontend1,
				preExistingFrontend2,
				frontendForServiceExposingPort80,
			},
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		// Create and customize a mock EdgeLB manager.
		manager := new(edgelbmanagertestutil.MockEdgeLBManager)
		manager.On("PoolGroup").Return(testEdgeLBPoolGroup)
		// Update the EdgeLB pool object in-place.
		mustUpdate, _ := NewServiceTranslator(testClusterName, test.service, test.options, manager).updateEdgeLBPoolObject(test.pool)
		// Check that the need for a pool update was adequately detected.
		assert.Equal(t, test.expectedWasChanged, mustUpdate)
		// Check that all expected backends are present.
		assert.Equal(t, test.expectedBackends, test.pool.Haproxy.Backends)
		// Check that all expected frontends are present.
		assert.Equal(t, test.expectedFrontends, test.pool.Haproxy.Frontends)
	}
}
