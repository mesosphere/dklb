package translator

import (
	"context"
	"fmt"
	"reflect"

	"github.com/mesosphere/dcos-edge-lb/models"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/edgelb/manager"
	dklberrors "github.com/mesosphere/dklb/pkg/errors"
	kubernetesutil "github.com/mesosphere/dklb/pkg/util/kubernetes"
	"github.com/mesosphere/dklb/pkg/util/prettyprint"
)

// ServiceTranslator is the base implementation of ServiceTranslator.
type ServiceTranslator struct {
	// clusterName is the name of the Mesos framework that corresponds to the current Kubernetes cluster.
	clusterName string
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
func NewServiceTranslator(clusterName string, service *corev1.Service, options ServiceTranslationOptions, manager manager.EdgeLBManager) *ServiceTranslator {
	return &ServiceTranslator{
		clusterName: clusterName,
		service:     service,
		options:     options,
		manager:     manager,
		logger:      log.WithField("service", kubernetesutil.Key(service)),
	}
}

// Translate performs translation of the associated Service resource into an EdgeLB pool.
func (st *ServiceTranslator) Translate() error {
	// Return immediately if pool translation is paused.
	if st.options.EdgeLBPoolTranslationPaused {
		st.logger.Warnf("skipping translation of %q as translation is paused for the resource", kubernetesutil.Key(st.service))
		return nil
	}

	// Check whether a pool with the requested name already exists in EdgeLB.
	ctx, fn := context.WithTimeout(context.Background(), defaultEdgeLBManagerTimeout)
	defer fn()
	pool, err := st.manager.GetPoolByName(ctx, st.options.EdgeLBPoolName)
	if err != nil {
		if !dklberrors.IsNotFound(err) {
			return fmt.Errorf("failed to check for the existence of the %q edgelb pool: %v", st.options.EdgeLBPoolName, err)
		}
	}
	// If the target EdgeLB pool does not exist, we must try to create it,
	if pool == nil {
		return st.createEdgeLBPool()
	}
	// If the target EdgeLB pool already exists, we must check whether it needs to be updated/deleted.
	return st.updateOrDeleteEdgeLBPool(pool)
}

// createEdgeLBPool makes a decision on whether an EdgeLB pool should be created for the associated Service resource.
// This decision is based on the pool creation strategy specified for the Service resource.
// In case it should be created, it proceeds to actually creating it.
func (st *ServiceTranslator) createEdgeLBPool() error {
	// If the pool creation strategy is "Never", the target pool must be provisioned manually.
	// Hence, we should just exit.
	if st.options.EdgeLBPoolCreationStrategy == constants.EdgeLBPoolCreationStrategyNever {
		return fmt.Errorf("edgelb pool %q targeted by service %q does not exist, but the pool creation strategy is %q", st.options.EdgeLBPoolName, kubernetesutil.Key(st.service), st.options.EdgeLBPoolCreationStrategy)
	}

	// If the Service resource's ".status" field contains at least one IP/host, that means a pool has once existed, but has been deleted manually.
	// Hence, and if the pool creation strategy is "Once", we should also just exit.
	if len(st.service.Status.LoadBalancer.Ingress) > 0 && st.options.EdgeLBPoolCreationStrategy == constants.EdgeLBPoolCreationStrategyOnce {
		return fmt.Errorf("edgelb pool %q targeted by service %q has probably been manually deleted, and the pool creation strategy is %q", st.options.EdgeLBPoolName, kubernetesutil.Key(st.service), st.options.EdgeLBPoolCreationStrategy)
	}

	// At this point, we know that we must create the target EdgeLB pool based on the specified options.
	// TODO (@bcustodio) Wait for the pool to be provisioned and check for its IP(s) so that the service resource's ".status" field can be updated.
	pool := st.createEdgeLBPoolObject()
	prettyprint.Logf(log.Debugf, pool, "computed edgelb pool object for service %q", kubernetesutil.Key(st.service))
	ctx, fn := context.WithTimeout(context.Background(), defaultEdgeLBManagerTimeout)
	defer fn()
	_, err := st.manager.CreatePool(ctx, pool)
	return err
}

// updateOrDeleteEdgeLBPool makes a decision on whether the specified EdgeLB pool should be updated/deleted based on the current status of the associated Service resource.
// In case it should be updated/deleted, it proceeds to actually updating/deleting it.
// TODO (@bcustodio) Decide whether we should also update the pool's role and its CPU/memory/size requests when updating a pool.
func (st *ServiceTranslator) updateOrDeleteEdgeLBPool(pool *models.V2Pool) error {
	// Check whether the pool object must be updated.
	wasChanged, report := st.updateEdgeLBPoolObject(pool)
	// Report the status of the pool.
	prettyprint.Logf(log.Debugf, report, "inspection report for edgelb pool %q", pool.Name)

	// If the pool doesn't need to be updated, we just return.
	if !wasChanged {
		st.logger.Debugf("edgelb pool %q is synced", pool.Name)
		return nil
	}

	// At this point we know that the pool must be either updated or deleted.
	// If the pool is empty (i.e. it has no frontends or backends) we proceed to deleting it.
	// Otherwise, we proceed to updating it.

	var (
		err error
		ctx context.Context
		fn  func()
	)

	ctx, fn = context.WithTimeout(context.Background(), defaultEdgeLBManagerTimeout)
	defer fn()

	if len(pool.Haproxy.Frontends) == 0 && len(pool.Haproxy.Backends) == 0 {
		// The pool is empty, so we delete it.
		st.logger.Debugf("edgelb pool %q is empty and must be deleted", pool.Name)
		err = st.manager.DeletePool(ctx, pool.Name)
	} else {
		// The pool is not empty, so we update it.
		st.logger.Debugf("edgelb pool %q must be updated", pool.Name)
		_, err = st.manager.UpdatePool(ctx, pool)
	}

	return err
}

// createEdgeLBPoolObject creates an EdgeLB pool object that satisfies the current Service resource.
func (st *ServiceTranslator) createEdgeLBPoolObject() *models.V2Pool {
	backends := make([]*models.V2Backend, 0, len(st.service.Spec.Ports))
	frontends := make([]*models.V2Frontend, 0, len(st.service.Spec.Ports))

	// Iterate over port definitions and create the corresponding backend and frontend objects.
	for _, port := range st.service.Spec.Ports {
		// Compute the backend and frontend for the current service port.
		backend, frontend := computeBackendForServicePort(st.clusterName, st.service, port), computeFrontendForServicePort(st.clusterName, st.service, port, st.options)
		// Append the backend to the slice of backends.
		backends = append(backends, backend)
		// Append the frontend to the slice of frontends.
		frontends = append(frontends, frontend)
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
	// Request for the pool to join the requested DC/OS virtual network if applicable.
	if st.options.EdgeLBPoolNetwork != "" {
		p.VirtualNetworks = []*models.V2PoolVirtualNetworksItems0{
			{
				Name: st.options.EdgeLBPoolNetwork,
			},
		}
	}
	return p
}

// updateEdgeLBPoolObject updates the specified pool object in order to reflect the status of the current Service resource.
// It modifies the specified pool in-place and returns a value indicating whether the pool object contains changes.
// Backends and frontends (the "objects") are added/modified/deleted according to the following rules:
// * If the object is not owned by the current Service resource, it is left untouched.
// * If the current Service resource has been marked for deletion or is not of type "LoadBalancer" anymore, it is removed.
// * If the object is owned by the current Service resource but the corresponding service port has disappeared, it is removed.
// * If the object is owned by the current Service resource and the corresponding service port still exists, it is checked for correctness and updated if necessary.
// Furthermore, service ports are iterated over in order to understand which objects must be added to the pool.
func (st *ServiceTranslator) updateEdgeLBPoolObject(pool *models.V2Pool) (wasChanged bool, report poolInspectionReport) {
	// serviceDeleted holds whether the Service resource has been deleted (or changed to a different type, which must produce a similar effect).
	serviceDeleted := st.service.DeletionTimestamp != nil || st.service.Spec.Type != corev1.ServiceTypeLoadBalancer

	// If the service has not been deleted, we iterate over ports defined on the service and re-compute the corresponding backend and frontend objects.
	// These will be later compared with the backend and frontend objects reported by the EdgeLB API server (i.e. those in "pool").
	desiredBackendFrontends := make(map[int32]servicePortBackendFrontend, len(st.service.Spec.Ports))
	if !serviceDeleted {
		for _, port := range st.service.Spec.Ports {
			desiredBackendFrontends[port.Port] = servicePortBackendFrontend{
				Backend:  computeBackendForServicePort(st.clusterName, st.service, port),
				Frontend: computeFrontendForServicePort(st.clusterName, st.service, port, st.options),
			}
		}
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
		if err != nil || !backendMetadata.IsOwnedBy(st.clusterName, st.service) {
			updatedBackends = append(updatedBackends, backend)
			report.Report("no changes required for backend %q (not owned by %s)", backend.Name, kubernetesutil.Key(st.service))
			continue
		}
		// At this point we know the current backend is owned by the current service.
		// Check whether the service has been deleted, and skip (i.e. remove) the backend if it has.
		if serviceDeleted {
			wasChanged = true
			report.Report("must delete backend %q as %q was deleted or its type has changed", backend.Name, kubernetesutil.Key(st.service))
			continue
		}
		// Check whether the target service port is still present in the service and skip (i.e. remove) the backend if it doesn't.
		if _, exists := desiredBackendFrontends[backendMetadata.ServicePort]; !exists {
			wasChanged = true
			report.Report("must delete backend %q as port %d is missing from %s", backend.Name, backendMetadata.ServicePort, kubernetesutil.Key(st.service))
			continue
		}
		// At this point we know the service port corresponding to the current backend still exists.
		// Mark the current backend/service port as having been visited.
		visitedBackends[backendMetadata.ServicePort] = true
		// Check whether the existing (current) and computed (desired) backends differ.
		// In case differences are detected, we replace the existing backend by the computed one.
		if !reflect.DeepEqual(backend, desiredBackendFrontends[backendMetadata.ServicePort].Backend) {
			wasChanged = true
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
		if err != nil || !frontendMetadata.IsOwnedBy(st.clusterName, st.service) {
			updatedFrontends = append(updatedFrontends, frontend)
			report.Report("no changes required for frontend %q (not owned by %s)", frontend.Name, kubernetesutil.Key(st.service))
			continue
		}
		// At this point we know the current frontend is owned by the current service.
		// Check whether the service has been deleted, and skip (i.e. remove) the frontend if it has.
		if serviceDeleted {
			wasChanged = true
			report.Report("must delete frontend %q as %q was deleted or its type has changed", frontend.Name, kubernetesutil.Key(st.service))
			continue
		}
		// Check whether the target service port is still present in the service and skip (i.e. remove) the frontend if it doesn't.
		if _, exists := desiredBackendFrontends[frontendMetadata.ServicePort]; !exists {
			wasChanged = true
			report.Report("must delete backend %q as port %d is missing from %s", frontend.Name, frontendMetadata.ServicePort, kubernetesutil.Key(st.service))
			continue
		}
		// At this point we know the service port corresponding to the current frontend still exists.
		// Mark the current frontend/service port as having been visited.
		visitedFrontends[frontendMetadata.ServicePort] = true
		// Check whether the existing (current) and computed (desired) frontends differ.
		// In case differences are detected, we replace the existing frontend by the computed one.
		if !reflect.DeepEqual(frontend, desiredBackendFrontends[frontendMetadata.ServicePort].Frontend) {
			wasChanged = true
			updatedFrontends = append(updatedFrontends, desiredBackendFrontends[frontendMetadata.ServicePort].Frontend)
			report.Report("must modify frontend %q", frontend.Name)
		} else {
			updatedFrontends = append(updatedFrontends, frontend)
			report.Report("no changes required for frontend %q", frontend.Name)
		}
	}

	// Replace the pool's backends and frontends with the (possibly empty) updated lists.
	pool.Haproxy.Backends, pool.Haproxy.Frontends = updatedBackends, updatedFrontends

	// If the current Service resource was deleted, there is nothing else to compute.
	if serviceDeleted {
		return wasChanged, report
	}

	// Iterate over all the service ports, in order to understand whether there are new service port definitions.
	// For every service port, if the corresponding backend or frontend isn't present, add it.
	for port, dbf := range desiredBackendFrontends {
		if _, visited := visitedBackends[port]; !visited {
			// The current service port doesn't have a matching backend.
			// Hence, we add it to the set of updated backends and mark the pool as requiring an update.
			wasChanged = true
			pool.Haproxy.Backends = append(pool.Haproxy.Backends, dbf.Backend)
			report.Report("must create backend %q", dbf.Backend.Name)
		}
		if _, visited := visitedFrontends[port]; !visited {
			// The current service port doesn't have a matching frontend.
			// Hence, we add it to the set of updated frontends and mark the pool as requiring an update.
			wasChanged = true
			pool.Haproxy.Frontends = append(pool.Haproxy.Frontends, dbf.Frontend)
			report.Report("must create frontend %q", dbf.Frontend.Name)
		}
	}

	// Return a value indicating whether the pool was changed, and the pool inspection report.
	return wasChanged, report
}
