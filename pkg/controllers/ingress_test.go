package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"

	dklbcache "github.com/mesosphere/dklb/pkg/cache"
	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/edgelb/manager"
	cachetestutil "github.com/mesosphere/dklb/test/util/cache"
)

func TestIngressController_enqueueIngressesReferecingService(t *testing.T) {
	dummyIngress := &extsv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.EdgeLBIngressClassAnnotationKey: constants.EdgeLBIngressClassAnnotationValue,
			},
			Namespace: "namespace-1",
			Name:      "ingress-1",
		},
		Spec: extsv1beta1.IngressSpec{
			Backend: &extsv1beta1.IngressBackend{
				ServiceName: "service-1",
				ServicePort: intstr.FromInt(80),
			},
		},
	}
	dummyService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace-1",
			Name:      "service-1",
		},
		Spec: corev1.ServiceSpec{},
	}

	tests := []struct {
		description string
		expected    *extsv1beta1.Ingress
		service     *corev1.Service
		ingress     *extsv1beta1.Ingress
	}{
		{
			description: "should enqueue ingress: contains required annotation",
			expected:    dummyIngress,
			service:     dummyService,
			ingress:     dummyIngress,
		},
		{
			description: "should not enqueue ingress: does not contain required annotation",
			expected:    nil,
			service:     dummyService,
			ingress: &extsv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace-1",
					Name:      "ingress-1",
				},
				Spec: extsv1beta1.IngressSpec{
					Backend: &extsv1beta1.IngressBackend{
						ServiceName: "service-1",
						ServicePort: intstr.FromInt(80),
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Logf("test case: %s", test.description)

		eventRecorder := record.NewFakeRecorder(10)
		fakeEdgeLB := manager.NewFakeEdgeLBManager()
		sharedInformerFactory := cachetestutil.NewFakeSharedInformerFactory(test.ingress)
		ingressInformer := sharedInformerFactory.Extensions().V1beta1().Ingresses()
		serviceInformer := sharedInformerFactory.Core().V1().Services()
		kubeCache := dklbcache.NewInformerBackedResourceCache(sharedInformerFactory)
		kubeClient := fake.NewSimpleClientset(test.service, test.ingress)

		ic := &IngressController{
			kubeClient:    kubeClient,
			er:            eventRecorder,
			kubeCache:     kubeCache,
			edgelbManager: fakeEdgeLB,
		}

		fake := newFakeGenericController()
		ic.base = fake
		ic.initialize(ingressInformer, serviceInformer)

		ic.enqueueIngressesReferencingService(test.service)

		if test.expected == nil {
			assert.Equal(t, len(fake.queue), 0)
		} else {
			assert.Equal(t, test.expected, fake.queue[0])
		}
	}
}
