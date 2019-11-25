package api

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mesosphere/dklb/pkg/cluster"
	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/util/pointers"
)

func TestNewRandomEdgeLBPoolName(t *testing.T) {
	tests := []struct {
		description            string
		expectedPoolNameRegexp string
		clusterName            string
		prefix                 string
	}{
		{
			description:            "should create an edgelb pool name",
			expectedPoolNameRegexp: "^dev--test",
			clusterName:            "dev/test",
			prefix:                 "",
		},
		{
			description:            "should truncate cluster name",
			expectedPoolNameRegexp: "^one--sixty--three--character--string----",
			clusterName:            "one--sixty--three--character--string--used--for--testing--this",
			prefix:                 "",
		},
		{
			description:            "should truncate cluster name with a prefix",
			expectedPoolNameRegexp: "^cloud--one--sixty--three--character--s",
			clusterName:            "one--sixty--three--character--string--used--for--testing--this",
			prefix:                 "cloud",
		},
		{
			description:            "should trim first forward slash in cluster name",
			expectedPoolNameRegexp: "^foldered--cluster--name--",
			clusterName:            "/foldered/cluster/name",
			prefix:                 "",
		},
	}

	for _, test := range tests {
		t.Logf("test case: %s", test.description)

		cluster.Name = test.clusterName
		poolName := newRandomEdgeLBPoolName(test.prefix)

		assert.Regexp(t, regexp.MustCompile(test.expectedPoolNameRegexp), poolName)
	}
}

func TestGetServiceEdgeLBPoolSpec(t *testing.T) {
	tests := []struct {
		description string
		service     *corev1.Service
		error       error
		edgeLBPool  *ServiceEdgeLBPoolSpec
	}{
		{
			description: "should not fail with empty frontend port",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-service",
					Annotations: map[string]string{
						constants.DklbConfigAnnotationKey: `
name: "dklb"
size: 2
frontends:
- port:
  servicePort: 6379`,
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Port: 6379},
					},
				},
			},
			edgeLBPool: &ServiceEdgeLBPoolSpec{
				BaseEdgeLBPoolSpec: BaseEdgeLBPoolSpec{
					Name:                       pointers.NewString("dklb"),
					Size:                       pointers.NewInt32(2),
					CloudProviderConfiguration: pointers.NewString(""),
					CPUs:                       pointers.NewFloat64(0.1),
					Memory:                     pointers.NewInt32(128),
					Network:                    pointers.NewString(""),
					Role:                       pointers.NewString("slave_public"),
					Strategies: &EdgeLBPoolManagementStrategies{
						Creation: &EdgeLBPoolCreationStrategyIfNotPresent,
					},
				},
				Frontends: []ServiceEdgeLBPoolFrontendSpec{
					{
						Port:        pointers.NewInt32(6379),
						ServicePort: 6379,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Logf("test case: %s", test.description)

		edgeLBPool, err := GetServiceEdgeLBPoolSpec(test.service)

		assert.Equal(t, test.error, err)
		assert.EqualValues(t, test.edgeLBPool, edgeLBPool)
	}
}
