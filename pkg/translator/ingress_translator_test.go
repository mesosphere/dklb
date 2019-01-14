package translator_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mesosphere/dcos-edge-lb/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/mesosphere/dklb/pkg/constants"
	dklberrors "github.com/mesosphere/dklb/pkg/errors"
	"github.com/mesosphere/dklb/pkg/translator"
	cachetestutil "github.com/mesosphere/dklb/test/util/cache"
	edgelbmanagertestutil "github.com/mesosphere/dklb/test/util/edgelb/manager"
	ingresstestutil "github.com/mesosphere/dklb/test/util/kubernetes/ingress"
	servicetestutil "github.com/mesosphere/dklb/test/util/kubernetes/service"
)

var (
	// dummyService1 is a dummy Service resource exposing a single port.
	dummyService1 = servicetestutil.DummyServiceResource("foo", "bar", func(service *corev1.Service) {
		service.Spec.Type = corev1.ServiceTypeNodePort
		service.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "http",
				NodePort:   30220,
				Port:       80,
				TargetPort: intstr.FromInt(8080),
			},
		}
	})
)

func TestIngressTranslator_Translate(t *testing.T) {
	tests := []struct {
		description    string
		resources      []runtime.Object
		ingress        *extsv1beta1.Ingress
		poolName       string
		mockCustomizer func(manager *edgelbmanagertestutil.MockEdgeLBManager)
		options        translator.IngressTranslationOptions
		expectedError  error
	}{
		// Tests that a pool is created whenever it doesn't exist and the pool creation strategy is set to "IfNotPresent".
		{
			description: "pool is created whenever it doesn't exist and the pool creation strategy is set to \"IfNotPresent\"",
			resources:   []runtime.Object{},
			ingress: ingresstestutil.DummyIngressResource("foo", "bar", func(ingress *extsv1beta1.Ingress) {
				ingress.Annotations = map[string]string{
					constants.EdgeLBIngressClassAnnotationKey: constants.EdgeLBIngressClassAnnotationValue,
					constants.EdgeLBPoolNameAnnotationKey:     "foo-bar",
				}
			}),
			poolName: "foo-bar",
			mockCustomizer: func(manager *edgelbmanagertestutil.MockEdgeLBManager) {
				manager.On("GetPoolByName", mock.Anything, "foo-bar").Return(nil, dklberrors.NotFound(errors.New("not found")))
				manager.On("CreatePool", mock.Anything, mock.Anything).Return(&models.V2Pool{}, nil)
			},
			options: translator.IngressTranslationOptions{
				BaseTranslationOptions: translator.BaseTranslationOptions{
					EdgeLBPoolName:             "foo-bar",
					EdgeLBPoolCreationStrategy: constants.EdgeLBPoolCreationStrategyIfNotPresent,
				},
			},
			expectedError: nil,
		},
		// Tests that a pool is not created when it doesn't exist and the pool creation strategy is set to "Never".
		{
			description: "pool is not created when it doesn't exist and the pool creation strategy is set to \"Never\"",
			resources:   []runtime.Object{},
			ingress: ingresstestutil.DummyIngressResource("foo", "bar", func(ingress *extsv1beta1.Ingress) {
				ingress.Annotations = map[string]string{
					constants.EdgeLBIngressClassAnnotationKey: constants.EdgeLBIngressClassAnnotationValue,
					constants.EdgeLBPoolNameAnnotationKey:     "foo-bar",
				}
			}),
			poolName: "foo-bar",
			mockCustomizer: func(manager *edgelbmanagertestutil.MockEdgeLBManager) {
				manager.On("GetPoolByName", mock.Anything, "foo-bar").Return(nil, dklberrors.NotFound(errors.New("not found")))
			},
			options: translator.IngressTranslationOptions{
				BaseTranslationOptions: translator.BaseTranslationOptions{
					EdgeLBPoolName:             "foo-bar",
					EdgeLBPoolCreationStrategy: constants.EdgeLBPoolCreationStrategyNever,
				},
			},
			expectedError: fmt.Errorf("edgelb pool %q targeted by ingress %q does not exist, but the pool creation strategy is %q", "foo-bar", "foo/bar", constants.EdgeLBPoolCreationStrategyNever),
		},
		// Tests that a pool is not created when it doesn't exist, the target Ingress resource has a non-empty status field and the pool creation strategy is set to "Once".
		{
			description: "pool is not created when it doesn't exist, the target Ingress resource has a non-empty status field and the pool creation strategy is set to \"Once\"",
			resources:   []runtime.Object{},
			ingress: ingresstestutil.DummyIngressResource("foo", "bar", func(ingress *extsv1beta1.Ingress) {
				ingress.Annotations = map[string]string{
					constants.EdgeLBIngressClassAnnotationKey: constants.EdgeLBIngressClassAnnotationValue,
					constants.EdgeLBPoolNameAnnotationKey:     "foo-bar",
				}
				ingress.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
					{
						IP: "1.2.3.4",
					},
				}
			}),
			poolName: "foo-bar",
			mockCustomizer: func(manager *edgelbmanagertestutil.MockEdgeLBManager) {
				manager.On("GetPoolByName", mock.Anything, "foo-bar").Return(nil, dklberrors.NotFound(errors.New("not found")))
			},
			options: translator.IngressTranslationOptions{
				BaseTranslationOptions: translator.BaseTranslationOptions{
					EdgeLBPoolName:             "foo-bar",
					EdgeLBPoolCreationStrategy: constants.EdgeLBPoolCreationStrategyOnce,
				},
			},
			expectedError: fmt.Errorf("edgelb pool %q targeted by ingress %q has probably been manually deleted, and the pool creation strategy is %q", "foo-bar", "foo/bar", constants.EdgeLBPoolCreationStrategyOnce),
		},
		// Tests that a pool is updated whenever it exists but is not in sync with the target Ingress resource.
		{
			description: "pool is updated whenever it exists but is not in sync with the target Ingress resource",
			resources: []runtime.Object{
				dummyService1,
			},
			ingress: ingresstestutil.DummyIngressResource("foo", "bar", func(ingress *extsv1beta1.Ingress) {
				ingress.Annotations = map[string]string{
					constants.EdgeLBIngressClassAnnotationKey: constants.EdgeLBIngressClassAnnotationValue,
					constants.EdgeLBPoolNameAnnotationKey:     "foo-bar",
				}
				ingress.Spec.Rules = []extsv1beta1.IngressRule{
					{
						Host: "foo.bar",
						IngressRuleValue: extsv1beta1.IngressRuleValue{
							HTTP: &extsv1beta1.HTTPIngressRuleValue{
								Paths: []extsv1beta1.HTTPIngressPath{
									{
										Path: "/baz",
										Backend: extsv1beta1.IngressBackend{
											ServiceName: dummyService1.Name,
											ServicePort: intstr.FromString(dummyService1.Spec.Ports[0].Name),
										},
									},
								},
							},
						},
					},
				}
			}),
			poolName: "foo-bar",
			mockCustomizer: func(manager *edgelbmanagertestutil.MockEdgeLBManager) {
				manager.On("GetPoolByName", mock.Anything, "foo-bar").Return(&models.V2Pool{
					Name:    "foo-bar",
					Haproxy: &models.V2Haproxy{},
				}, nil)
				manager.On("UpdatePool", mock.Anything, mock.Anything).Return(&models.V2Pool{}, nil)
			},
			options: translator.IngressTranslationOptions{
				BaseTranslationOptions: translator.BaseTranslationOptions{
					EdgeLBPoolName: "foo-bar",
				},
			},
			expectedError: nil,
		},
	}
	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		// Create a mock KubernetesResourceCache.
		k := cachetestutil.NewFakeKubernetesResourceCache(test.resources...)
		// Create and customize a mock EdgeLB manager.
		m := new(edgelbmanagertestutil.MockEdgeLBManager)
		test.mockCustomizer(m)
		// Perform translation of the Ingress resource.
		err := translator.NewIngressTranslator(testClusterName, test.ingress, test.options, k, m).Translate()
		if test.expectedError != nil {
			// Make sure we've got the expected error.
			assert.Equal(t, test.expectedError, err)
		} else {
			// Make sure that we haven't got any errors, and that all expected method calls were performed.
			assert.NoError(t, err)
			m.AssertExpectations(t)
		}
	}
}
