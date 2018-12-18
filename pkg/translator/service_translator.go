package translator

import (
	"context"
	"errors"
	"fmt"

	"github.com/mesosphere/dcos-edge-lb/models"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"

	"github.com/mesosphere/dklb/pkg/cluster"
	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/edgelb/manager"
	dklberrors "github.com/mesosphere/dklb/pkg/errors"
	"github.com/mesosphere/dklb/pkg/util/kubernetes"
	"github.com/mesosphere/dklb/pkg/util/prettyprint"
)

// ServiceTranslator is the base implementation of ServiceTranslator.
type ServiceTranslator struct {
	// service is the Service resource to be translated.
	service *corev1.Service
	// options is the set of options used to perform translation.
	options ServiceTranslationOptions
	// manager is the instance of the EdgeLB manager to use for managing EdgeLB pools.
	manager manager.EdgeLBManager
}

// NewServiceTranslator returns a service translator that can be used to translate the specified Service resource into an EdgeLB pool.
func NewServiceTranslator(service *corev1.Service, options ServiceTranslationOptions, manager manager.EdgeLBManager) *ServiceTranslator {
	return &ServiceTranslator{
		service: service,
		options: options,
		manager: manager,
	}
}

// Translate performs translation of the associated Service resource into an EdgeLB pool.
func (st *ServiceTranslator) Translate() error {
	// Check whether a pool with the requested name already exists in EdgeLB.
	ctx, fn := context.WithTimeout(context.Background(), defaultEdgeLBManagerTimeout)
	defer fn()
	pool, err := st.manager.GetPoolByName(ctx, st.options.EdgeLBPoolName)
	if err != nil {
		if !dklberrors.IsNotFound(err) {
			return fmt.Errorf("failed to check for the existence of the %q edgelb pool: %v", st.options.EdgeLBPoolName, err)
		}
	}
	// If the target EdgeLB pool does not exist, we should try to create it (depending on the chosen pool creation strategy).
	if pool == nil {
		return st.maybeCreateEdgeLBPool()
	}
	// If the target EdgeLB pool already exists, we should try to update it.
	return st.maybeUpdateEdgeLBPool(pool)
}

// maybeCreateEdgeLBPool makes a decision on whether an EdgeLB pool should be created for the associated Service resource.
// In case it should, it proceeds to actually creating it.
// TODO (@bcustodio) Implement.
func (st *ServiceTranslator) maybeCreateEdgeLBPool() error {
	// If the pool creation strategy is "Never", the target pool must be provisioned manually.
	// Hence, we should just exit.
	if st.options.EdgeLBPoolCreationStrategy == constants.EdgeLBPoolCreationStrategyNever {
		return fmt.Errorf("edgelb pool %q targeted by service %q does not exist, but the pool creation strategy is %q", st.options.EdgeLBPoolName, kubernetes.Key(st.service), st.options.EdgeLBPoolCreationStrategy)
	}

	// Handle the scenario in which the target EdgeLB pool has once existed (because the service's status contains at least one IP) but has since been deleted.
	// TODO (@bcustodio) Understand what else could/should be done in this scenario.
	if st.options.EdgeLBPoolCreationStrategy == constants.EdgeLBPoolCreationStragegyOnce && len(st.service.Status.LoadBalancer.Ingress) > 0 {
		return fmt.Errorf("edgelb pool %q targeted by service %q has probably been manually deleted, and the pool creation strategy is %q", st.options.EdgeLBPoolName, kubernetes.Key(st.service), st.options.EdgeLBPoolCreationStrategy)
	}

	// At this point, we know that we must create the target EdgeLB pool based on the specified options.
	// TODO (@bcustodio) Wait for the pool to be provisioned and check for its IP(s) so that the service's status can be updated.
	pool := st.createPoolObject()
	log.Debugf("computed pool object:\n\n%s", prettyprint.Sprint(pool))
	ctx, fn := context.WithTimeout(context.Background(), defaultEdgeLBManagerTimeout)
	defer fn()
	_, err := st.manager.CreatePool(ctx, pool)
	return err
}

// maybeUpdateEdgeLBPool makes a decision on whether the specified EdgeLB pool should be updated based on the current status of the associated Service resource.
// In case it should, it proceeds to actually updating it.
// TODO (@bcustodio) Implement.
func (st *ServiceTranslator) maybeUpdateEdgeLBPool(pool *models.V2Pool) error {
	return errors.New("pool updates are not implemented")
}

func (st *ServiceTranslator) createPoolObject() *models.V2Pool {
	var (
		bs []*models.V2Backend
		fs []*models.V2Frontend
	)

	// Iterate over port definitions and create the corresponding backend and frontend objects.
	for _, port := range st.service.Spec.Ports {
		// Create a TCP backend targeting Kubernetes nodes belonging to the current Kubernetes cluster.
		// The backend port is the NodePort for the current service port.
		t := true
		b := &models.V2Backend{
			Name:     backendNameForServicePort(st.service, port),
			Protocol: models.V2ProtocolTCP,
			Services: []*models.V2Service{
				{
					Mesos: &models.V2ServiceMesos{
						FrameworkName:   cluster.KubernetesClusterFrameworkName,
						TaskNamePattern: constants.KubeNodeTaskPattern,
					},
					Endpoint: &models.V2Endpoint{
						Check: &models.V2EndpointCheck{
							Enabled: &t,
						},
						Port: port.NodePort,
						Type: models.V2EndpointTypeCONTAINERIP,
					},
				},
			},
		}
		// Append the backend to the slice of backends.
		bs = append(bs, b)

		// Create a TCP frontend targeting this backend.
		// The frontend port is the port defined in the service or overridden using annotations.
		p := st.options.EdgeLBPoolPortMap[port.Port]
		f := &models.V2Frontend{
			Name:     frontendNameForServicePort(st.service, port),
			Protocol: models.V2ProtocolTCP,
			BindPort: &p,
			LinkBackend: &models.V2FrontendLinkBackend{
				DefaultBackend: b.Name,
			},
		}
		// Append the frontend to the slice of frontends.
		fs = append(fs, f)
	}

	// Create and return the pool object.
	s := int32(st.options.EdgeLBPoolSize)
	p := &models.V2Pool{
		Name:      st.options.EdgeLBPoolName,
		Namespace: &DefaultEdgeLBPoolNamespace,
		Role:      st.options.EdgeLBPoolRole,
		Cpus:      float64(st.options.EdgeLBPoolCpus.MilliValue()) / 1000,
		Mem:       int32(st.options.EdgeLBPoolMem.Value() / (1024 * 1024)),
		Count:     &s,
		Haproxy: &models.V2Haproxy{
			Backends:  bs,
			Frontends: fs,
		},
	}
	return p
}
