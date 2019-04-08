package translator

import (
	"encoding/json"
	"fmt"

	"github.com/mesosphere/dcos-edge-lb/models"
)

// unmarshalCloudProviderObject unmarshals the provided string as a "V2CloudProvider" object.
func (st *ServiceTranslator) unmarshalCloudProviderObject(v string) (*models.V2CloudProvider, error) {
	o := &models.V2CloudProvider{}
	if err := json.Unmarshal([]byte(v), o); err != nil {
		return nil, fmt.Errorf("failed to unmarshal the cloud-provider configuration: %v", err)
	}
	return o, nil
}
