package translator

import (
	"context"
	"fmt"
	"reflect"

	"github.com/mesosphere/dcos-edge-lb/models"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"

	"github.com/mesosphere/dklb/pkg/cluster"
	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/edgelb/manager"
	dklberrors "github.com/mesosphere/dklb/pkg/errors"
	kubernetesutil "github.com/mesosphere/dklb/pkg/util/kubernetes"
	"github.com/mesosphere/dklb/pkg/util/pointers"
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
	// logger is the logger to use when performing translation.
	logger *log.Entry
}

// NewServiceTranslator returns a service translator that can be used to translate the specified Service resource into an EdgeLB pool.
func NewServiceTranslator(service *corev1.Service, options ServiceTranslationOptions, manager manager.EdgeLBManager) *ServiceTranslator {
	return &ServiceTranslator{
		service: service,
		options: options,
		manager: manager,
		logger:  log.WithField("service", kubernetesutil.Key(service)),
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
		return fmt.Errorf("edgelb pool %q targeted by service %q does not exist, but the pool creation strategy is %q", st.options.EdgeLBPoolName, kubernetesutil.Key(st.service), st.options.EdgeLBPoolCreationStrategy)
	}

	// Handle the scenario in which the target EdgeLB pool has once existed (because the service's status contains at least one IP) but has since been deleted.
	// TODO (@bcustodio) Understand what else could/should be done in this scenario.
	if st.options.EdgeLBPoolCreationStrategy == constants.EdgeLBPoolCreationStrategyOnce && len(st.service.Status.LoadBalancer.Ingress) > 0 {
		return fmt.Errorf("edgelb pool %q targeted by service %q has probably been manually deleted, and the pool creation strategy is %q", st.options.EdgeLBPoolName, kubernetesutil.Key(st.service), st.options.EdgeLBPoolCreationStrategy)
	}

	// At this point, we know that we must create the target EdgeLB pool based on the specified options.
	// TODO (@bcustodio) Wait for the pool to be provisioned and check for its IP(s) so that the service's status can be updated.
	pool := st.createEdgeLBPoolObject()
	log.Debugf("computed edgelb pool object for service %q:\n%s", kubernetesutil.Key(st.service), prettyprint.Sprint(pool))
	ctx, fn := context.WithTimeout(context.Background(), defaultEdgeLBManagerTimeout)
	defer fn()
	_, err := st.manager.CreatePool(ctx, pool)
	return err
}

// maybeUpdateEdgeLBPool makes a decision on whether the specified EdgeLB pool should be updated based on the current status of the associated Service resource.
// In case it should, it proceeds to actually updating it.
// TODO (@bcustodio) Decide whether we should also update the pool's role and its CPU/memory/size requests.
func (st *ServiceTranslator) maybeUpdateEdgeLBPool(pool *models.V2Pool) error {
	// Check whether the pool object must be updated.
	mustUpdate, report := st.maybeUpdateEdgeLBPoolObject(pool)
	// Report the status of the pool.
	st.logger.Debugf("inspection report for edgelb pool %q:\n%s", pool.Name, report.String())

	// If the pool doesn't need to be updated, we just return.
	if !mustUpdate {
		st.logger.Debugf("edgelb pool %q is synced", pool.Name)
		return nil
	}

	// Otherwise, we update the pool in the EdgeLB API server.
	st.logger.Debugf("edgelb pool %q must be updated", pool.Name)
	ctx, fn := context.WithTimeout(context.Background(), defaultEdgeLBManagerTimeout)
	defer fn()
	_, err := st.manager.UpdatePool(ctx, pool)
	return err
}

// createEdgeLBPoolObject creates an EdgeLB pool object that satisfies the current Service resource.
func (st *ServiceTranslator) createEdgeLBPoolObject() *models.V2Pool {
	backends := make([]*models.V2Backend, 0, len(st.service.Spec.Ports))
	frontends := make([]*models.V2Frontend, 0, len(st.service.Spec.Ports))

	// Iterate over port definitions and create the corresponding backend and frontend objects.
	for _, port := range st.service.Spec.Ports {
		// Compute the backend and frontend for the current service port.
		bf := st.computeBackendFrontendForServicePort(port)
		// Append the backend to the slice of backends.
		backends = append(backends, bf.Backend)
		// Append the frontend to the slice of frontends.
		frontends = append(frontends, bf.Frontend)
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
			Backends:  backends,
			Frontends: frontends,
		},
	}
	return p
}

// computeBackendFrontendForServicePort computes the backend and frontend that correspond to the specified service port.
func (st *ServiceTranslator) computeBackendFrontendForServicePort(sp corev1.ServicePort) servicePortBackendFrontend {
	// Compute the name to give to the backend.
	bn := backendNameForServicePort(st.service, sp)
	// Compute the name to give to the frontend.
	fn := frontendNameForServicePort(st.service, sp)
	// Compute the backend and frontend objects and return them.
	return servicePortBackendFrontend{
		Backend: &models.V2Backend{
			Balance:  constants.EdgeLBBackendBalanceLeastConnections,
			Name:     bn,
			Protocol: models.V2ProtocolTCP,
			Services: []*models.V2Service{
				{
					Endpoint: &models.V2Endpoint{
						Check: &models.V2EndpointCheck{
							Enabled: pointers.NewBool(true),
						},
						Port: sp.NodePort,
						Type: models.V2EndpointTypeCONTAINERIP,
					},
					Marathon: &models.V2ServiceMarathon{
						// We don't want to use any Marathon service as the backend.
					},
					Mesos: &models.V2ServiceMesos{
						FrameworkName:   cluster.KubernetesClusterFrameworkName,
						TaskNamePattern: constants.KubeNodeTaskPattern,
					},
				},
			},
			// Explicitly set "RewriteHTTP" in order to make it easier to compare a computed backend with a V2Backend returned by the EdgeLB API server later on.
			RewriteHTTP: &models.V2RewriteHTTP{
				Request: &models.V2RewriteHTTPRequest{
					Forwardfor:                pointers.NewBool(true),
					RewritePath:               pointers.NewBool(true),
					SetHostHeader:             pointers.NewBool(true),
					XForwardedPort:            pointers.NewBool(true),
					XForwardedProtoHTTPSIfTLS: pointers.NewBool(true),
				},
				Response: &models.V2RewriteHTTPResponse{
					RewriteLocation: pointers.NewBool(true),
				},
			},
		},
		Frontend: &models.V2Frontend{
			BindAddress: constants.EdgeLBFrontendBindAddress,
			Name:        fn,
			Protocol:    models.V2ProtocolTCP,
			BindPort:    pointers.NewInt32(st.options.EdgeLBPoolPortMap[sp.Port]),
			LinkBackend: &models.V2FrontendLinkBackend{
				DefaultBackend: bn,
			},
		},
	}
}

// maybeUpdateEdgeLBPoolObject updates the specified pool object in order to reflect the status of the current Service resource.
// It modifies the specified pool in-place and returns a value indicating whether the pool must be updated in the EdgeLB API server.
// Backends and frontends (the "objects") are added/modified/deleted according to the following rules:
// * If the object is not owned by the current Service resource, it is left untouched.
// * If the object is owned by the current Service resource but the corresponding service port has disappeared, it is removed.
// * If the object is owned by the current Service resource and the corresponding service port still exists, it is checked for correctness and updated if necessary.
// Furthermore, service ports are iterated over in order to understand which objects must be added to the pool.
func (st *ServiceTranslator) maybeUpdateEdgeLBPoolObject(pool *models.V2Pool) (mustUpdate bool, report poolInspectionReport) {
	// Iterate over ports defined on the service and re-compute the corresponding backend and frontend objects.
	// These will be compared with the backend and frontend objects reported by the EdgeLB API server (i.e. those in "pool").
	desiredBackendFrontends := make(map[int32]servicePortBackendFrontend, len(st.service.Spec.Ports))
	for _, port := range st.service.Spec.Ports {
		desiredBackendFrontends[port.Port] = st.computeBackendFrontendForServicePort(port)
	}

	// visitedBackends holds the set of service ports corresponding to visited (existing) backends.
	// It is used to understand which service ports currently have backend objects in the pool, and which don't.
	visitedBackends := make(map[int32]bool, len(pool.Haproxy.Backends))
	// updatedBackends holds the set of updated backend objects.
	// It is used as the final set of backends for the pool if we find out we need to update it.
	updatedBackends := make([]*models.V2Backend, 0, len(pool.Haproxy.Backends))

	// Iterate over the pool's backends and check whether each backend is owned by the current service.
	// In case a backend isn't owned by the current service, it is left unchanged and added to the set of "updated" backends.
	// Otherwise, it is checked for correctness and, if necessary, replaced with the computed backend for the target service port.
	for _, backend := range pool.Haproxy.Backends {
		// Parse the name of the backend in order to determine if the current service owns it.
		// If the current backend isn't owned by the current service, it is left unchanged.
		backendMetadata, err := computeServiceOwnedEdgeLBObjectMetadata(backend.Name)
		if err != nil || !backendMetadata.IsOwnedBy(st.service) {
			updatedBackends = append(updatedBackends, backend)
			report.Report("no changes required for backend %q (not owned by %s)", backend.Name, kubernetesutil.Key(st.service))
			continue
		}
		// At this point we know the current backend is owned by the current service.
		// Check whether the target service port is still present in the service and skip (i.e. remove) the backend if it doesn't.
		if _, exists := desiredBackendFrontends[backendMetadata.ServicePort]; !exists {
			mustUpdate = true
			report.Report("must delete backend %q as port %d is missing from %s", backend.Name, backendMetadata.ServicePort, kubernetesutil.Key(st.service))
			continue
		}
		// At this point we know the service port corresponding to the current backend still exists.
		// Mark the current backend/service port as having been visited.
		visitedBackends[backendMetadata.ServicePort] = true
		// Check whether the existing (current) and computed (desired) backends differ.
		// In case differences are detected, we replace the existing backend by the computed one.
		// TODO (@bcustodio) Investigate if "reflect.DeepEqual" is a good option or if we need to get "smarter" on detecting differences.
		if !reflect.DeepEqual(backend, desiredBackendFrontends[backendMetadata.ServicePort].Backend) {
			mustUpdate = true
			updatedBackends = append(updatedBackends, desiredBackendFrontends[backendMetadata.ServicePort].Backend)
			report.Report("must modify backend %q", backend.Name)
		} else {
			updatedBackends = append(updatedBackends, backend)
			report.Report("no changes required for backend %q", backend.Name)
		}
	}

	// visitedFrontends holds the set of service ports corresponding to visited (existing) frontends.
	// It is used to understand which service ports currently have frontend objects in the pool, and which don't.
	visitedFrontends := make(map[int32]bool, len(pool.Haproxy.Frontends))
	// updatedFrontends holds the set of updated frontend objects.
	// It is used as the final set of frontends for the pool if we find out we need to update it.
	updatedFrontends := make([]*models.V2Frontend, 0, len(pool.Haproxy.Frontends))

	// Iterate over the pool's frontends and check whether each frontend is owned by the current service.
	// In case a frontend isn't owned by the current service, it is left unchanged and added to the set of "updated" frontends.
	// Otherwise, it is checked for correctness and, if necessary, replaced with the computed frontends for the target service port.
	for _, frontend := range pool.Haproxy.Frontends {
		// Parse the name of the frontend in order to determine if the current service owns it.
		// If the current frontend isn't owned by the current service, it is left unchanged.
		frontendMetadata, err := computeServiceOwnedEdgeLBObjectMetadata(frontend.Name)
		if err != nil || !frontendMetadata.IsOwnedBy(st.service) {
			updatedFrontends = append(updatedFrontends, frontend)
			report.Report("no changes required for frontend %q (not owned by %s)", frontend.Name, kubernetesutil.Key(st.service))
			continue
		}
		// At this point we know the current frontend is owned by the current service.
		// Check whether the target service port is still present in the service and skip (i.e. remove) the frontend if it doesn't.
		if _, exists := desiredBackendFrontends[frontendMetadata.ServicePort]; !exists {
			mustUpdate = true
			report.Report("must delete frontend %q as port %d is missing from %s", frontend.Name, frontendMetadata.ServicePort, kubernetesutil.Key(st.service))
			continue
		}
		// At this point we know the service port corresponding to the current frontend still exists.
		// Mark the current frontend/service port as having been visited.
		visitedFrontends[frontendMetadata.ServicePort] = true
		// Check whether the existing (current) and computed (desired) frontends differ.
		// In case differences are detected, we replace the existing frontend by the computed one.
		// TODO (@bcustodio) Investigate if "reflect.DeepEqual" is a good option or if we need to get "smarter" on detecting differences.
		if !reflect.DeepEqual(frontend, desiredBackendFrontends[frontendMetadata.ServicePort].Frontend) {
			mustUpdate = true
			updatedFrontends = append(updatedFrontends, desiredBackendFrontends[frontendMetadata.ServicePort].Frontend)
			report.Report("must modify frontend %q", frontend.Name)
		} else {
			updatedFrontends = append(updatedFrontends, frontend)
			report.Report("no changes required for frontend %q", frontend.Name)
		}
	}

	// Iterate over all the service ports, in order to understand whether there are new service port definitions.
	// For every service port, if the corresponding backend or frontend isn't present, add it.
	for port, dbf := range desiredBackendFrontends {
		if _, visited := visitedBackends[port]; !visited {
			// The current service port doesn't have a matching backend.
			// Hence, we add it to the set of updated backends and mark the pool as requiring an update.
			mustUpdate = true
			updatedBackends = append(updatedBackends, dbf.Backend)
			report.Report("must create backend %q", dbf.Backend.Name)
		}
		if _, visited := visitedFrontends[port]; !visited {
			// The current service port doesn't have a matching frontend.
			// Hence, we add it to the set of updated frontends and mark the pool as requiring an update.
			mustUpdate = true
			updatedFrontends = append(updatedFrontends, dbf.Frontend)
			report.Report("must create frontend %q", dbf.Frontend.Name)
		}
	}

	// Replace the pool's backends and frontends, and return a value indicating whether the pool must be updated.
	pool.Haproxy.Backends, pool.Haproxy.Frontends = updatedBackends, updatedFrontends
	return mustUpdate, report
}
