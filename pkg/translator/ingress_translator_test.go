package translator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	edgelbmodels "github.com/mesosphere/dcos-edge-lb/pkg/apis/models"

	dklbcache "github.com/mesosphere/dklb/pkg/cache"
	edgelbmanager "github.com/mesosphere/dklb/pkg/edgelb/manager"
	cachetestutil "github.com/mesosphere/dklb/test/util/cache"
	mockedgelb "github.com/mesosphere/dklb/test/util/edgelb/manager"
	servicetestutil "github.com/mesosphere/dklb/test/util/kubernetes/service"
)

func TestTranslate(t *testing.T) {
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
	tests := []struct {
		description      string
		edgelbManager    func() edgelbmanager.EdgeLBManager
		eventRecorder    record.EventRecorder
		expectedError    error
		expectedLBStatus *corev1.LoadBalancerStatus
		ingress          *extsv1beta1.Ingress
		kubeCache        dklbcache.KubernetesResourceCache
	}{
		{
			description: "should succeed translating ingress",
			edgelbManager: func() edgelbmanager.EdgeLBManager {
				edgelbManager := new(mockedgelb.MockEdgeLBManager)
				edgelbManager.On("PoolGroup").Return("test-pool-group", nil)
				edgelbManager.On("GetPool", mock.Anything, mock.Anything).Return(nil, nil)
				edgelbManager.On("CreatePool", mock.Anything, mock.Anything).Return(&edgelbmodels.V2Pool{}, nil)
				edgelbManager.On("GetPoolMetadata", mock.Anything, mock.Anything).Return(&edgelbmodels.V2PoolMetadata{}, nil)
				return edgelbManager
			},
			eventRecorder:    record.NewFakeRecorder(10),
			expectedError:    nil,
			expectedLBStatus: &corev1.LoadBalancerStatus{},
			ingress: &extsv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-ingress",
				},
				Spec: extsv1beta1.IngressSpec{
					TLS: []extsv1beta1.IngressTLS{
						{SecretName: "test-secret"},
					},
				},
			},
			kubeCache: dklbcache.NewInformerBackedResourceCache(cachetestutil.NewFakeSharedInformerFactory(defaultService)),
		},
	}

	for _, test := range tests {
		status, err := NewIngressTranslator(test.ingress, test.kubeCache, test.edgelbManager(), test.eventRecorder).Translate()
		assert.Equal(t, test.expectedError, err)
		assert.Equal(t, test.expectedLBStatus, status)
	}
}
