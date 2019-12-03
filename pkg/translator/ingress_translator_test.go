package translator

import (
	"encoding/json"
	"fmt"
	"testing"

	strfmt "github.com/go-openapi/strfmt"
	"github.com/mesosphere/dcos-edge-lb/pkg/apis/models"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"

	edgelbmodels "github.com/mesosphere/dcos-edge-lb/pkg/apis/models"

	dklbcache "github.com/mesosphere/dklb/pkg/cache"
	"github.com/mesosphere/dklb/pkg/cluster"
	"github.com/mesosphere/dklb/pkg/constants"
	edgelbmanager "github.com/mesosphere/dklb/pkg/edgelb/manager"
	translatorapi "github.com/mesosphere/dklb/pkg/translator/api"
	"github.com/mesosphere/dklb/pkg/util/pointers"
	cachetestutil "github.com/mesosphere/dklb/test/util/cache"
	mockedgelb "github.com/mesosphere/dklb/test/util/edgelb/manager"
	servicetestutil "github.com/mesosphere/dklb/test/util/kubernetes/service"
)

func TestTranslate(t *testing.T) {
	cluster.Name = "test-cluster-test-translate"
	// dummyService represents a dummy Service resource.
	defaultService := servicetestutil.DummyServiceResource("kube-system", "dklb", func(service *corev1.Service) {
		service.Spec.Type = corev1.ServiceTypeLoadBalancer
		service.Spec.Ports = []corev1.ServicePort{
			{
				Port:     80,
				NodePort: 31789,
			},
			{
				Port:     443,
				NodePort: 32789,
			},
		}
	})
	testService := servicetestutil.DummyServiceResource("test-namespace", "test-service", func(service *corev1.Service) {
		service.Spec.Type = corev1.ServiceTypeLoadBalancer
		service.Spec.Ports = []corev1.ServicePort{
			{
				Port:     80,
				NodePort: 31889,
			},
			{
				Port:     443,
				NodePort: 32889,
			},
		}
	})
	tests := []struct {
		description      string
		edgelbManager    func() edgelbmanager.EdgeLBManager
		eventRecorder    record.EventRecorder
		expectedError    error
		expectedLBStatus *corev1.LoadBalancerStatus
		ingress          *extsv1beta1.Ingress
		kubeCache        dklbcache.KubernetesResourceCache
	}{
		// {
		// 	description: "should succeed translating ingress",
		// 	edgelbManager: func() edgelbmanager.EdgeLBManager {
		// 		edgelbManager := new(mockedgelb.MockEdgeLBManager)
		// 		edgelbManager.On("PoolGroup").Return("test-pool-group", nil)
		// 		edgelbManager.On("GetPool", mock.Anything, mock.Anything).Return(nil, nil)
		// 		edgelbManager.On("CreatePool", mock.Anything, mock.Anything).Return(&edgelbmodels.V2Pool{}, nil)
		// 		edgelbManager.On("GetPoolMetadata", mock.Anything, mock.Anything).Return(&edgelbmodels.V2PoolMetadata{}, nil)
		// 		return edgelbManager
		// 	},
		// 	eventRecorder:    record.NewFakeRecorder(10),
		// 	expectedError:    nil,
		// 	expectedLBStatus: &corev1.LoadBalancerStatus{},
		// 	ingress: &extsv1beta1.Ingress{
		// 		ObjectMeta: metav1.ObjectMeta{
		// 			Namespace: "test-namespace",
		// 			Name:      "test-ingress",
		// 		},
		// 		Spec: extsv1beta1.IngressSpec{
		// 			TLS: []extsv1beta1.IngressTLS{
		// 				{SecretName: "test-secret"},
		// 			},
		// 		},
		// 	},
		// 	kubeCache: dklbcache.NewInformerBackedResourceCache(cachetestutil.NewFakeSharedInformerFactory(defaultService)),
		// },
		{
			description: "should succeed adding to frontend backendmap",
			edgelbManager: func() edgelbmanager.EdgeLBManager {
				pool := &models.V2Pool{
					Namespace: pointers.NewString(""),
					Role:      "slave-public",
					Cpus:      0.1,
					Mem:       int32(128),
					VirtualNetworks: []*models.V2PoolVirtualNetworksItems0{
						{Name: "dcos"},
					},
					Count: pointers.NewInt32(1),
					Haproxy: &models.V2Haproxy{
						Backends: []*models.V2Backend{
							{
								Name: "existing-backend",
							},
						},
						Frontends: []*models.V2Frontend{
							{
								BindAddress: "0.0.0.0",
								BindPort:    pointers.NewInt32(443),
								LinkBackend: &models.V2FrontendLinkBackend{
									DefaultBackend: "existing-backend",
									Map: []*models.V2FrontendLinkBackendMapItems0{
										{Backend: "existing-backend"},
									},
								},
								Name:         "frontend",
								Protocol:     "HTTPS",
								Certificates: []string{"$SECRETS/existing-secret"},
							},
						},
						Stats: &models.V2Stats{
							BindPort: pointers.NewInt32(0),
						},
					},
					Secrets: []*models.V2PoolSecretsItems0{
						{
							File:   "existing-secret",
							Secret: "existing-secret",
						},
					},
				}
				edgelbManager := new(mockedgelb.MockEdgeLBManager)
				edgelbManager.On("PoolGroup").Return("test-pool-group", nil)
				edgelbManager.On("GetPool", mock.Anything, mock.Anything).Return(pool, nil)
				edgelbManager.On("GetPoolMetadata", mock.Anything, mock.Anything).Return(&edgelbmodels.V2PoolMetadata{}, nil)
				edgelbManager.On("DeletePool", mock.Anything, mock.Anything).Return(fmt.Errorf("unexpected function call: DeletePool"))
				edgelbManager.On("UpdatePool", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
					pool := args.Get(1).(*models.V2Pool)

					reg := strfmt.NewFormats()
					err := pool.Validate(reg)
					assert.Nil(t, err)

					expectedPool := &models.V2Pool{
						Namespace: pointers.NewString(""),
						Role:      "slave-public",
						Cpus:      0.1,
						Mem:       int32(128),
						VirtualNetworks: []*models.V2PoolVirtualNetworksItems0{
							{Name: "dcos"},
						},
						Count: pointers.NewInt32(1),
						Haproxy: &models.V2Haproxy{
							Backends: []*models.V2Backend{
								{
									Name: "existing-backend",
								},
								{
									Name:     "test-cluster-test-translate:test-namespace:test-ingress-1:test-service:80",
									Protocol: "HTTP",
									Balance:  constants.EdgeLBBackendBalanceLeastConnections,
									RewriteHTTP: &models.V2RewriteHTTP{
										Request: &models.V2RewriteHTTPRequest{
											Forwardfor:                pointers.NewBool(true),
											XForwardedPort:            pointers.NewBool(true),
											XForwardedProtoHTTPSIfTLS: pointers.NewBool(true),
											RewritePath:               pointers.NewBool(false),
											SetHostHeader:             pointers.NewBool(false),
										},
										Response: &models.V2RewriteHTTPResponse{
											RewriteLocation: pointers.NewBool(false),
										},
									},
									Services: []*models.V2Service{
										{
											Endpoint: &models.V2Endpoint{
												Check: &models.V2EndpointCheck{
													Enabled: pointers.NewBool(true),
												},
												MiscStr: "check-ssl ssl verify none",
												Port:    31789,
												Type:    models.V2EndpointTypeCONTAINERIP,
											},
											Marathon: &models.V2ServiceMarathon{},
											Mesos: &models.V2ServiceMesos{
												FrameworkName: "test-cluster",
											},
										},
										{
											Endpoint: &models.V2Endpoint{
												Check: &models.V2EndpointCheck{
													Enabled: pointers.NewBool(true),
												},
												MiscStr: computeEdgeLBBackendMiscStr(constants.EdgeLBBackendBackup),
												Port:    32889,
												Type:    models.V2EndpointTypeCONTAINERIP,
											},
											Marathon: &models.V2ServiceMarathon{
												// We don't want to use any Marathon service as the backend.
											},
											Mesos: &models.V2ServiceMesos{
												FrameworkName:   cluster.Name,
												TaskNamePattern: constants.KubeNodeTaskPattern,
											},
										},
									},
								},
							},
							Frontends: []*models.V2Frontend{
								{
									BindAddress: "0.0.0.0",
									BindPort:    pointers.NewInt32(443),
									LinkBackend: &models.V2FrontendLinkBackend{
										DefaultBackend: "existing-backend",
										Map: []*models.V2FrontendLinkBackendMapItems0{
											{
												Backend: "existing-backend",
											},
											{
												Backend: "test-cluster-test-translate:test-namespace:test-ingress-1:test-service:80",
												HostEq:  "test-host.com",
												PathReg: "^/bar(/.*)?$",
											},
											{
												Backend: "test-cluster-test-translate:test-namespace:test-ingress-1:test-service:80",
												HostEq:  "test-host.com",
												PathReg: "^.*$",
											},
										},
									},
									Name:         "frontend",
									Protocol:     "HTTPS",
									Certificates: []string{"$SECRETS/existing-secret"},
								},
							},
							Stats: &models.V2Stats{
								BindPort: pointers.NewInt32(0),
							},
						},
						Secrets: []*models.V2PoolSecretsItems0{
							{
								File:   "existing-secret",
								Secret: "existing-secret",
							},
							{
								File:   "uid__test-secret-1",
								Secret: "uid__test-secret-1",
							},
						},
					}

					assert.Equal(t, expectedPool, pool)
				}).Return(pool, nil)
				return edgelbManager
			},
			eventRecorder:    record.NewFakeRecorder(10),
			expectedError:    nil,
			expectedLBStatus: &corev1.LoadBalancerStatus{},
			ingress: &extsv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-ingress-1",
					UID:       "uid",
					Annotations: map[string]string{
						constants.EdgeLBIngressClassAnnotationKey: constants.EdgeLBIngressClassAnnotationValue,
						constants.DklbConfigAnnotationKey: `
frontends:
  http:
    mode: disabled
`,
					},
				},
				Spec: extsv1beta1.IngressSpec{
					Backend: &extsv1beta1.IngressBackend{
						ServiceName: testService.Name,
						ServicePort: intstr.FromString("80"),
					},
					Rules: []extsv1beta1.IngressRule{
						{
							Host: "test-host.com",
							IngressRuleValue: extsv1beta1.IngressRuleValue{
								HTTP: &extsv1beta1.HTTPIngressRuleValue{
									Paths: []extsv1beta1.HTTPIngressPath{
										{
											Backend: extsv1beta1.IngressBackend{
												ServiceName: testService.Name,
												ServicePort: intstr.FromInt(80),
											},
										},
										{
											Path: "/bar(/.*)?",
											Backend: extsv1beta1.IngressBackend{
												ServiceName: testService.Name,
												ServicePort: intstr.FromString("80"),
											},
										},
									},
								},
							},
						},
					},
					TLS: []extsv1beta1.IngressTLS{
						{SecretName: "test-secret-1"},
					},
				},
			},
		},
	}
	// Enable logging at the requested level.
	l, _ := log.ParseLevel("trace")
	log.SetLevel(l)

	for _, test := range tests {
		test.kubeCache = dklbcache.NewInformerBackedResourceCache(cachetestutil.NewFakeSharedInformerFactory(defaultService, testService, test.ingress))
		status, err := NewIngressTranslator(test.ingress, test.kubeCache, test.edgelbManager(), test.eventRecorder).Translate()
		assert.Equal(t, test.expectedError, err)
		assert.Equal(t, test.expectedLBStatus, status)
	}
}

func TestTranslate_createEdgeLBPoolObject(t *testing.T) {
	cluster.Name = "test-cluster"
	tests := []struct {
		description    string
		expectedChange bool
		expectedPool   *models.V2Pool
		backendMap     IngressBackendNodePortMap
		it             *IngressTranslator
		spec           *translatorapi.IngressEdgeLBPoolSpec
	}{
		{
			description:    "should create an edgelb pool with default frontend",
			backendMap:     IngressBackendNodePortMap{},
			expectedChange: false,
			expectedPool: &models.V2Pool{
				Namespace: pointers.NewString(""),
				Role:      "slave-public",
				Cpus:      0.1,
				Mem:       int32(128),
				VirtualNetworks: []*models.V2PoolVirtualNetworksItems0{
					{Name: "dcos"},
				},
				Haproxy: &models.V2Haproxy{
					Backends: []*models.V2Backend{},
					Frontends: []*models.V2Frontend{
						{
							BindAddress: "0.0.0.0",
							BindPort:    pointers.NewInt32(80),
							LinkBackend: &models.V2FrontendLinkBackend{
								DefaultBackend: "",
							},
							Name:     "test-cluster:test-namespace:test-ingress:http",
							Protocol: "HTTP",
						},
					},
					Stats: &models.V2Stats{
						BindPort: pointers.NewInt32(0),
					},
				},
			},
			it: &IngressTranslator{
				ingress: &extsv1beta1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-ingress",
						Annotations: map[string]string{
							constants.EdgeLBIngressClassAnnotationKey: constants.EdgeLBIngressClassAnnotationValue,
						},
						UID: types.UID("uid"),
					},
				},
				spec: &translatorapi.IngressEdgeLBPoolSpec{
					BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
						Name:    pointers.NewString(""),
						Role:    pointers.NewString("slave-public"),
						CPUs:    pointers.NewFloat64(0.1),
						Memory:  pointers.NewInt32(128),
						Network: pointers.NewString("dcos"),
					},
					Frontends: &translatorapi.IngressEdgeLBPoolFrontendsSpec{
						HTTP: &translatorapi.IngressEdgeLBPoolHTTPFrontendSpec{
							Mode: pointers.NewString(translatorapi.IngressEdgeLBHTTPModeEnabled),
							Port: pointers.NewInt32(80),
						},
					},
				},
			},
		},
		{
			description:    "should create an edgelb pool with an http and https frontend",
			backendMap:     IngressBackendNodePortMap{},
			expectedChange: false,
			expectedPool: &models.V2Pool{
				Namespace: pointers.NewString(""),
				Role:      "slave-public",
				Cpus:      0.1,
				Mem:       int32(128),
				VirtualNetworks: []*models.V2PoolVirtualNetworksItems0{
					{Name: "dcos"},
				},
				Haproxy: &models.V2Haproxy{
					Backends: []*models.V2Backend{},
					Frontends: []*models.V2Frontend{
						{
							BindAddress: "0.0.0.0",
							BindPort:    pointers.NewInt32(80),
							LinkBackend: &models.V2FrontendLinkBackend{
								DefaultBackend: "",
							},
							Name:     "test-cluster:test-namespace:test-ingress:http",
							Protocol: "HTTP",
						},
						{
							BindAddress: "0.0.0.0",
							BindPort:    pointers.NewInt32(443),
							LinkBackend: &models.V2FrontendLinkBackend{
								DefaultBackend: "",
							},
							Name:         "test-cluster:test-namespace:test-ingress:https",
							Protocol:     "HTTPS",
							Certificates: []string{"$SECRETS/uid__test-secret"},
						},
					},
					Stats: &models.V2Stats{
						BindPort: pointers.NewInt32(0),
					},
				},
				Secrets: []*models.V2PoolSecretsItems0{
					{
						File:   "uid__test-secret",
						Secret: "uid__test-secret",
					},
				},
			},
			it: &IngressTranslator{
				ingress: &extsv1beta1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-ingress",
						Annotations: map[string]string{
							constants.EdgeLBIngressClassAnnotationKey: constants.EdgeLBIngressClassAnnotationValue,
						},
						UID: types.UID("uid"),
					},
					Spec: extsv1beta1.IngressSpec{
						TLS: []extsv1beta1.IngressTLS{
							{SecretName: "test-secret"},
						},
					},
				},
				spec: &translatorapi.IngressEdgeLBPoolSpec{
					BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
						Name:    pointers.NewString(""),
						Role:    pointers.NewString("slave-public"),
						CPUs:    pointers.NewFloat64(0.1),
						Memory:  pointers.NewInt32(128),
						Network: pointers.NewString("dcos"),
					},
					Frontends: &translatorapi.IngressEdgeLBPoolFrontendsSpec{
						HTTP: &translatorapi.IngressEdgeLBPoolHTTPFrontendSpec{
							Mode: pointers.NewString(translatorapi.IngressEdgeLBHTTPModeEnabled),
							Port: pointers.NewInt32(80),
						},
						HTTPS: &translatorapi.IngressEdgeLBPoolHTTPSFrontendSpec{
							Port: pointers.NewInt32(443),
						},
					},
				},
			},
		},
		{
			description:    "should create an edgelb pool with one https frontend and http disabled",
			backendMap:     IngressBackendNodePortMap{},
			expectedChange: false,
			expectedPool: &models.V2Pool{
				Namespace: pointers.NewString(""),
				Role:      "slave-public",
				Cpus:      0.1,
				Mem:       int32(128),
				VirtualNetworks: []*models.V2PoolVirtualNetworksItems0{
					{Name: "dcos"},
				},
				Haproxy: &models.V2Haproxy{
					Backends: []*models.V2Backend{},
					Frontends: []*models.V2Frontend{
						{
							BindAddress: "0.0.0.0",
							BindPort:    pointers.NewInt32(443),
							LinkBackend: &models.V2FrontendLinkBackend{
								DefaultBackend: "",
							},
							Name:         "test-cluster:test-namespace:test-ingress:https",
							Protocol:     "HTTPS",
							Certificates: []string{"$SECRETS/uid__test-secret"},
						},
					},
					Stats: &models.V2Stats{
						BindPort: pointers.NewInt32(0),
					},
				},
				Secrets: []*models.V2PoolSecretsItems0{
					{
						File:   "uid__test-secret",
						Secret: "uid__test-secret",
					},
				},
			},
			it: &IngressTranslator{
				ingress: &extsv1beta1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-ingress",
						Annotations: map[string]string{
							constants.EdgeLBIngressClassAnnotationKey: constants.EdgeLBIngressClassAnnotationValue,
						},
						UID: types.UID("uid"),
					},
					Spec: extsv1beta1.IngressSpec{
						TLS: []extsv1beta1.IngressTLS{
							{SecretName: "test-secret"},
						},
					},
				},
				spec: &translatorapi.IngressEdgeLBPoolSpec{
					BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
						Name:    pointers.NewString(""),
						Role:    pointers.NewString("slave-public"),
						CPUs:    pointers.NewFloat64(0.1),
						Memory:  pointers.NewInt32(128),
						Network: pointers.NewString("dcos"),
					},
					Frontends: &translatorapi.IngressEdgeLBPoolFrontendsSpec{
						HTTP: &translatorapi.IngressEdgeLBPoolHTTPFrontendSpec{
							Mode: pointers.NewString(translatorapi.IngressEdgeLBHTTPModeDisabled),
						},
						HTTPS: &translatorapi.IngressEdgeLBPoolHTTPSFrontendSpec{
							Port: pointers.NewInt32(443),
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		pool := test.it.createEdgeLBPoolObject(test.backendMap)
		assert.Equal(t, test.expectedPool, pool)
	}
}

// Given an IngressTranslator with a spec it should update the pool to match
// expectedPool.
func TestTranslate_updateEdgeLBPoolObject(t *testing.T) {

	cluster.Name = "test-cluster"

	// returns an IngressTranslator, the mutator function can be used to
	// customize the object
	newIngressTranslator := func(mutator func(*IngressTranslator)) *IngressTranslator {
		it := &IngressTranslator{
			ingress: &extsv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-ingress",
					Annotations: map[string]string{
						constants.EdgeLBIngressClassAnnotationKey: constants.EdgeLBIngressClassAnnotationValue,
					},
					UID: types.UID("uid"),
				},
				Spec: extsv1beta1.IngressSpec{
					TLS: []extsv1beta1.IngressTLS{
						{SecretName: "test-secret"},
					},
				},
			},
			spec: &translatorapi.IngressEdgeLBPoolSpec{
				BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
					Name:    pointers.NewString(""),
					Role:    pointers.NewString("slave-public"),
					CPUs:    pointers.NewFloat64(0.1),
					Memory:  pointers.NewInt32(128),
					Network: pointers.NewString("dcos"),
					Size:    pointers.NewInt32(1),
				},
				Frontends: &translatorapi.IngressEdgeLBPoolFrontendsSpec{
					HTTP: &translatorapi.IngressEdgeLBPoolHTTPFrontendSpec{
						Mode: pointers.NewString(translatorapi.IngressEdgeLBHTTPModeDisabled),
					},
					HTTPS: &translatorapi.IngressEdgeLBPoolHTTPSFrontendSpec{
						Port: pointers.NewInt32(443),
					},
				},
			},
		}
		mutator(it)
		return it
	}

	// return a new Pool object, the mutator function can be used to customize
	// the object
	newPool := func(mutator func(*models.V2Pool)) *models.V2Pool {
		pool := &models.V2Pool{
			Namespace: pointers.NewString(""),
			Role:      "slave-public",
			Cpus:      0.1,
			Mem:       int32(128),
			VirtualNetworks: []*models.V2PoolVirtualNetworksItems0{
				{Name: "dcos"},
			},
			Count: pointers.NewInt32(1),
			Haproxy: &models.V2Haproxy{
				Backends: []*models.V2Backend{
					{Name: "backend"},
				},
				Frontends: []*models.V2Frontend{
					{
						BindAddress: "0.0.0.0",
						BindPort:    pointers.NewInt32(443),
						LinkBackend: &models.V2FrontendLinkBackend{
							DefaultBackend: "",
							Map: []*models.V2FrontendLinkBackendMapItems0{
								{Backend: "backend"},
							},
						},
						Name:         "test-cluster:test-namespace:test-ingress:https",
						Protocol:     "HTTPS",
						Certificates: []string{"$SECRETS/uid__test-secret"},
					},
				},
				Stats: &models.V2Stats{
					BindPort: pointers.NewInt32(0),
				},
			},
			Secrets: []*models.V2PoolSecretsItems0{
				{
					File:   "uid__test-secret",
					Secret: "uid__test-secret",
				},
			},
		}
		mutator(pool)
		return pool
	}

	tests := []struct {
		description    string
		backendMap     IngressBackendNodePortMap
		expectedChange bool
		it             *IngressTranslator
		// pool is the initial state of the edgelb pool
		pool *models.V2Pool
		// what we expect the edgelb pool to look like after running the test
		expectedPool *models.V2Pool
		beforeEach   func()
		afterEach    func()
	}{
		{
			description:    "should not update pool",
			backendMap:     IngressBackendNodePortMap{},
			expectedChange: false,
			it: newIngressTranslator(func(it *IngressTranslator) {
			}),
			pool: newPool(func(pool *models.V2Pool) {
			}),
			expectedPool: newPool(func(expectedPool *models.V2Pool) {
			}),
		},
		{
			description:    "should update frontend secrets",
			backendMap:     IngressBackendNodePortMap{},
			expectedChange: true,
			it: newIngressTranslator(func(it *IngressTranslator) {
				it.ingress.Spec.TLS = []extsv1beta1.IngressTLS{
					// in this test we update the name of the secret
					{SecretName: "should-update-test-secret-1"},
				}
			}),
			pool: newPool(func(*models.V2Pool) {
			}),
			expectedPool: newPool(func(expectedPool *models.V2Pool) {
				expectedPool.Haproxy.Frontends[0].Certificates = []string{
					"$SECRETS/uid__should-update-test-secret-1",
				}
				expectedPool.Secrets = []*models.V2PoolSecretsItems0{
					{
						File:   "uid__should-update-test-secret-1",
						Secret: "uid__should-update-test-secret-1",
					},
				}
			}),
		},
		{
			description:    "should not remove unmanaged pool secret",
			backendMap:     IngressBackendNodePortMap{},
			expectedChange: true,
			it: newIngressTranslator(func(*IngressTranslator) {
			}),
			pool: newPool(func(pool *models.V2Pool) {
				pool.Secrets = []*models.V2PoolSecretsItems0{
					{
						File:   "foobar",
						Secret: "foobar",
					},
				}
			}),
			expectedPool: newPool(func(expectedPool *models.V2Pool) {
				expectedPool.Secrets = []*models.V2PoolSecretsItems0{
					{
						File:   "uid__test-secret",
						Secret: "uid__test-secret",
					},
					{
						File:   "foobar",
						Secret: "foobar",
					},
				}
			}),
		},
		{
			description:    "should enable HTTP frontend",
			backendMap:     IngressBackendNodePortMap{},
			expectedChange: true,
			it: newIngressTranslator(func(it *IngressTranslator) {
				it.spec.Frontends.HTTP.Mode = pointers.NewString(translatorapi.IngressEdgeLBHTTPModeEnabled)
				it.spec.Frontends.HTTP.Port = pointers.NewInt32(80)
			}),
			pool: newPool(func(pool *models.V2Pool) {
			}),
			expectedPool: newPool(func(expectedPool *models.V2Pool) {
				expectedPool.Haproxy.Frontends = []*models.V2Frontend{
					{
						BindAddress: "0.0.0.0",
						BindPort:    pointers.NewInt32(80),
						LinkBackend: &models.V2FrontendLinkBackend{
							DefaultBackend: "",
						},
						Name:     "test-cluster:test-namespace:test-ingress:http",
						Protocol: "HTTP",
					},
					{
						BindAddress: "0.0.0.0",
						BindPort:    pointers.NewInt32(443),
						LinkBackend: &models.V2FrontendLinkBackend{
							DefaultBackend: "",
							Map: []*models.V2FrontendLinkBackendMapItems0{
								{Backend: "backend"},
							},
						},
						Name:         "test-cluster:test-namespace:test-ingress:https",
						Protocol:     "HTTPS",
						Certificates: []string{"$SECRETS/uid__test-secret"},
					},
				}
			}),
		},
		{
			description:    "should disable HTTP frontend",
			backendMap:     IngressBackendNodePortMap{},
			expectedChange: true,
			it: newIngressTranslator(func(*IngressTranslator) {
			}),
			pool: newPool(func(pool *models.V2Pool) {
				pool.Haproxy.Frontends = []*models.V2Frontend{
					{
						BindAddress: "0.0.0.0",
						BindPort:    pointers.NewInt32(80),
						LinkBackend: &models.V2FrontendLinkBackend{
							DefaultBackend: "",
						},
						Name: "test-cluster:test-namespace:test-ingress:http",
					},
					{
						BindAddress: "0.0.0.0",
						BindPort:    pointers.NewInt32(443),
						LinkBackend: &models.V2FrontendLinkBackend{
							DefaultBackend: "",
							Map: []*models.V2FrontendLinkBackendMapItems0{
								{Backend: "backend"},
							},
						},
						Name:         "test-cluster:test-namespace:test-ingress:https",
						Protocol:     "HTTPS",
						Certificates: []string{"$SECRETS/uid__test-secret"},
					},
				}
			}),
			expectedPool: newPool(func(*models.V2Pool) {
			}),
		},
		{
			description:    "should not remove unmanaged frontend and add http frontend",
			backendMap:     IngressBackendNodePortMap{},
			expectedChange: true,
			it: newIngressTranslator(func(it *IngressTranslator) {
				it.spec.Frontends.HTTP.Mode = pointers.NewString(translatorapi.IngressEdgeLBHTTPModeEnabled)
				it.spec.Frontends.HTTP.Port = pointers.NewInt32(80)
			}),
			pool: newPool(func(pool *models.V2Pool) {
				pool.Haproxy.Frontends = []*models.V2Frontend{
					{
						BindAddress: "0.0.0.0",
						BindPort:    pointers.NewInt32(443),
						LinkBackend: &models.V2FrontendLinkBackend{
							DefaultBackend: "",
						},
						Name:         "test-cluster:test-namespace:test-ingress:https",
						Protocol:     "HTTPS",
						Certificates: []string{"$SECRETS/uid__test-secret"},
					},
					{
						BindAddress: "0.0.0.0",
						BindPort:    pointers.NewInt32(444),
						LinkBackend: &models.V2FrontendLinkBackend{
							DefaultBackend: "",
						},
						Name:     "unmanaged",
						Protocol: "HTTP",
					},
				}
			}),
			expectedPool: newPool(func(expectedPool *models.V2Pool) {
				expectedPool.Haproxy.Frontends = []*models.V2Frontend{
					{
						BindAddress: "0.0.0.0",
						BindPort:    pointers.NewInt32(444),
						LinkBackend: &models.V2FrontendLinkBackend{
							DefaultBackend: "",
						},
						Name:     "unmanaged",
						Protocol: "HTTP",
					},
					{
						BindAddress: "0.0.0.0",
						BindPort:    pointers.NewInt32(80),
						LinkBackend: &models.V2FrontendLinkBackend{
							DefaultBackend: "",
						},
						Name:     "test-cluster:test-namespace:test-ingress:http",
						Protocol: "HTTP",
					},
					{
						BindAddress: "0.0.0.0",
						BindPort:    pointers.NewInt32(443),
						LinkBackend: &models.V2FrontendLinkBackend{
							DefaultBackend: "",
						},
						Name:         "test-cluster:test-namespace:test-ingress:https",
						Protocol:     "HTTPS",
						Certificates: []string{"$SECRETS/uid__test-secret"},
					},
				}
			}),
		},
		{
			description:    "should update https frontend port",
			backendMap:     IngressBackendNodePortMap{},
			expectedChange: true,
			it: newIngressTranslator(func(it *IngressTranslator) {
				it.spec.Frontends.HTTPS.Port = pointers.NewInt32(4430)
			}),
			pool: newPool(func(*models.V2Pool) {
			}),
			expectedPool: newPool(func(expectedPool *models.V2Pool) {
				expectedPool.Haproxy.Frontends[0].BindPort = pointers.NewInt32(4430)
			}),
			beforeEach: func() {
				// Enable logging at the requested level.
				l, _ := log.ParseLevel("trace")
				log.SetLevel(l)
			},
			afterEach: func() {
				l, _ := log.ParseLevel("error")
				log.SetLevel(l)
			},
		},
		{
			description:    "should update pools cpu requirements",
			backendMap:     IngressBackendNodePortMap{},
			expectedChange: true,
			it: newIngressTranslator(func(it *IngressTranslator) {
				it.spec.BaseEdgeLBPoolSpec.CPUs = pointers.NewFloat64(1.0)
			}),
			pool: newPool(func(*models.V2Pool) {
			}),
			expectedPool: newPool(func(expectedPool *models.V2Pool) {
				expectedPool.Cpus = 1.0
			}),
		},
		{
			description:    "should update pools memory requirements",
			backendMap:     IngressBackendNodePortMap{},
			expectedChange: true,
			it: newIngressTranslator(func(it *IngressTranslator) {
				it.spec.BaseEdgeLBPoolSpec.Memory = pointers.NewInt32(256)
			}),
			pool: newPool(func(*models.V2Pool) {
			}),
			expectedPool: newPool(func(expectedPool *models.V2Pool) {
				expectedPool.Mem = 256
			}),
		},
		{
			description:    "should update pools size",
			backendMap:     IngressBackendNodePortMap{},
			expectedChange: true,
			it: newIngressTranslator(func(it *IngressTranslator) {
				it.spec.BaseEdgeLBPoolSpec.Size = pointers.NewInt32(2)
			}),
			pool: newPool(func(*models.V2Pool) {
			}),
			expectedPool: newPool(func(expectedPool *models.V2Pool) {
				expectedPool.Count = pointers.NewInt32(2)
			}),
		},
		{
			description:    "should delete ingress",
			backendMap:     IngressBackendNodePortMap{},
			expectedChange: false,
			it: newIngressTranslator(func(it *IngressTranslator) {
				it.ingress.ObjectMeta.Annotations = map[string]string{}
			}),
			pool: newPool(func(pool *models.V2Pool) {
				pool.Haproxy.Backends = nil
			}),
			expectedPool: newPool(func(expectedPool *models.V2Pool) {
				expectedPool.Haproxy.Backends = make([]*edgelbmodels.V2Backend, 0)
				expectedPool.Haproxy.Frontends = make([]*edgelbmodels.V2Frontend, 0)
				expectedPool.Secrets = make([]*edgelbmodels.V2PoolSecretsItems0, 0)
			}),
		},
		{
			description:    "should delete ingress frontends and secrets",
			backendMap:     IngressBackendNodePortMap{},
			expectedChange: true,
			it: newIngressTranslator(func(it *IngressTranslator) {
				it.ingress.ObjectMeta.Annotations = map[string]string{}
			}),
			pool: newPool(func(pool *models.V2Pool) {
				pool.Haproxy.Backends = nil
				pool.Haproxy.Frontends = []*models.V2Frontend{
					{
						BindAddress: "0.0.0.0",
						BindPort:    pointers.NewInt32(80),
						LinkBackend: &models.V2FrontendLinkBackend{
							DefaultBackend: "",
						},
						Name:     "unmanaged",
						Protocol: "HTTP",
					},
				}
				pool.Secrets = []*models.V2PoolSecretsItems0{
					{
						File:   "foobar",
						Secret: "foobar",
					},
				}
			}),
			expectedPool: newPool(func(expectedPool *models.V2Pool) {
				expectedPool.Haproxy.Backends = make([]*edgelbmodels.V2Backend, 0)
				expectedPool.Haproxy.Frontends = []*models.V2Frontend{
					{
						BindAddress: "0.0.0.0",
						BindPort:    pointers.NewInt32(80),
						LinkBackend: &models.V2FrontendLinkBackend{
							DefaultBackend: "",
						},
						Name:     "unmanaged",
						Protocol: "HTTP",
					},
				}
				expectedPool.Secrets = []*models.V2PoolSecretsItems0{
					{
						File:   "foobar",
						Secret: "foobar",
					},
				}
			}),
		},
	}

	for _, test := range tests {
		t.Logf("test case: %s", test.description)
		if test.beforeEach != nil {
			test.beforeEach()
		}
		changed, _ := test.it.updateEdgeLBPoolObject(test.pool, test.backendMap)
		assert.Equal(t, test.expectedChange, changed)
		if test.description == "should not remove unmanaged frontend and add http frontend" {
			b, _ := json.MarshalIndent(test.pool, "", "  ")
			fmt.Printf("test.pool=%s\n", string(b))
		}
		assert.Equal(t, test.expectedPool, test.pool)
		reg := strfmt.NewFormats()
		err := test.pool.Validate(reg)
		assert.Nil(t, err)
		if test.afterEach != nil {
			test.afterEach()
		}
	}
}
