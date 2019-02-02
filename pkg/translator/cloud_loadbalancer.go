package translator

import (
	"encoding/json"
	"fmt"

	"github.com/mesosphere/dcos-edge-lb/models"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/util/kubernetes"
)

// computeCloudLoadBalancerObject computes a "V2CloudProvider" object from the contents of the referenced configmap.
func (st *ServiceTranslator) computeCloudLoadBalancerObject() (*models.V2CloudProvider, error) {
	// Read the configmap with the specified name.
	m, err := st.kubeCache.GetConfigMap(st.service.Namespace, *st.options.CloudLoadBalancerConfigMapName)
	if err != nil {
		return nil, fmt.Errorf("failed to read configmap \"%s/%s\": %v", st.service.Namespace, *st.options.CloudLoadBalancerConfigMapName, err)
	}
	// Read the value of the "spec" key, failing if it doesn't exist.
	v, exists := m.Data[constants.CloudLoadBalancerSpecKey]
	if !exists {
		return nil, fmt.Errorf("required key %q not found in configmap %q", constants.CloudLoadBalancerSpecKey, kubernetes.Key(m))
	}
	// Attempt to parse the provided value as a "V2CloudProvider" object.
	o := &models.V2CloudProvider{}
	if err := json.Unmarshal([]byte(v), o); err != nil {
		return nil, fmt.Errorf("failed to parse the value of the %q key: %v", constants.CloudLoadBalancerSpecKey, err)
	}
	return o, nil
}
