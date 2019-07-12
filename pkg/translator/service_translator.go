package translator

import (
	"context"
	"fmt"
	"reflect"

	"github.com/mesosphere/dcos-edge-lb/pkg/apis/models"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"

	dklbcache "github.com/mesosphere/dklb/pkg/cache"
	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/edgelb/manager"
	dklberrors "github.com/mesosphere/dklb/pkg/errors"
	translatorapi "github.com/mesosphere/dklb/pkg/translator/api"
	kubernetesutil "github.com/mesosphere/dklb/pkg/util/kubernetes"
	"github.com/mesosphere/dklb/pkg/util/pointers"
	"github.com/mesosphere/dklb/pkg/util/prettyprint"
)

// ServiceTranslator is the base implementation of ServiceTranslator.
type ServiceTranslator struct {
	// service is the Service resource to be translated.
	service *corev1.Service
	// spec is the EdgeLB pool configuration object to use when performing translation.
	spec *translatorapi.ServiceEdgeLBPoolSpec
	// kubeCache is the instance of the Kubernetes resource cache to use.
	kubeCache dklbcache.KubernetesResourceCache
	// manager is the instance of the EdgeLB manager to use for managing EdgeLB pools.
	manager manager.EdgeLBManager
	// logger is the logger to use when performing translation.
	logger *log.Entry
	// poolGroup is the DC/OS service group in which to create EdgeLB pools.
	poolGroup string
}

// NewServiceTranslator returns a service translator that can be used to translate the specified Service resource into an EdgeLB pool.
func NewServiceTranslator(service *corev1.Service, kubeCache dklbcache.KubernetesResourceCache, manager manager.EdgeLBManager) *ServiceTranslator {
	return &ServiceTranslator{
		service:   service,
		kubeCache: kubeCache,
		manager:   manager,
		logger:    log.WithField("service", kubernetesutil.Key(service)),
		poolGroup: manager.PoolGroup(),
	}
}

// Translate performs translation of the associated Service resource into an EdgeLB pool.
func (st *ServiceTranslator) Translate() (*corev1.LoadBalancerStatus, error) {
	// Grab the EdgeLB pool configuration object from the Service resource.
	spec, err := translatorapi.GetServiceEdgeLBPoolSpec(st.service)
	if err != nil {
		return nil, fmt.Errorf("the edgelb pool configuration object is not valid: %v", err)
	}
	st.spec = spec

	// Dump the EdgeLB pool configuration object for debugging purposes.
	prettyprint.LogfSpew(log.Tracef, spec, "edgelb pool configuration object for %q", kubernetesutil.Key(st.service))

	// Check whether a pool with the requested name already exists in EdgeLB.
	ctx, fn := context.WithTimeout(context.Background(), defaultEdgeLBManagerTimeout)
	defer fn()
	pool, err := st.manager.GetPool(ctx, *st.spec.Name)
	if err != nil {
		if !dklberrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to check for the existence of the %q edgelb pool: %v", *st.spec.Name, err)
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
func (st *ServiceTranslator) createEdgeLBPool() (*corev1.LoadBalancerStatus, error) {
	// If the pool creation strategy is "Never", the target pool must be provisioned manually.
	// Hence, we should just exit.
	if *st.spec.Strategies.Creation == translatorapi.EdgeLBPoolCreationStrategyNever {
		return nil, fmt.Errorf("edgelb pool %q targeted by service %q does not exist, but the pool creation strategy is %q", *st.spec.Name, kubernetesutil.Key(st.service), *st.spec.Strategies.Creation)
	}

	// If the Service resource's ".status" field contains at least one IP/host, that means a pool has once existed, but has been deleted manually.
	// Hence, and if the pool creation strategy is "Once", we should also just exit.
	if len(st.service.Status.LoadBalancer.Ingress) > 0 && *st.spec.Strategies.Creation == translatorapi.EdgeLBPoolCreationStrategyOnce {
		return nil, fmt.Errorf("edgelb pool %q targeted by service %q has probably been manually deleted, and the pool creation strategy is %q", *st.spec.Name, kubernetesutil.Key(st.service), *st.spec.Strategies.Creation)
	}

	// At this point, we know that we must create the target EdgeLB pool based on the specified options.
	pool, err := st.createEdgeLBPoolObject()
	if err != nil {
		return nil, err
	}
	// Print the computed EdgeLB pool object in "spew" and JSON formats.
	prettyprint.LogfSpew(log.Tracef, pool, "computed edgelb pool object for service %q", kubernetesutil.Key(st.service))
	prettyprint.LogfJSON(log.Debugf, pool, "computed edgelb pool object for service %q", kubernetesutil.Key(st.service))
	ctx, fn := context.WithTimeout(context.Background(), defaultEdgeLBManagerTimeout)
	defer fn()
	if _, err := st.manager.CreatePool(ctx, pool); err != nil {
		return nil, err
	}
	// Compute and return the status of the load-balancer.
	return computeLoadBalancerStatus(st.manager, pool.Name, st.service), nil
}

// updateOrDeleteEdgeLBPool makes a decision on whether the specified EdgeLB pool should be updated/deleted based on the current status of the associated Service resource.
// In case it should be updated/deleted, it proceeds to actually updating/deleting it.
// TODO (@bcustodio) Decide whether we should also update the pool's role and its CPU/memory/size requests when updating a pool.
func (st *ServiceTranslator) updateOrDeleteEdgeLBPool(pool *models.V2Pool) (*corev1.LoadBalancerStatus, error) {
	// Check whether the pool object must be updated.
	wasChanged, report, err := st.updateEdgeLBPoolObject(pool)
	if err != nil {
		return nil, err
	}
	// Report the status of the pool.
	prettyprint.LogfSpew(log.Tracef, report, "inspection report for edgelb pool %q", pool.Name)
	// Print the compputed EdgeLB pool object in "spew" and JSON formats.
	prettyprint.LogfSpew(log.Tracef, pool, "computed edgelb pool object for service %q", kubernetesutil.Key(st.service))
	prettyprint.LogfJSON(log.Debugf, pool, "computed edgelb pool object for service %q", kubernetesutil.Key(st.service))

	// If the pool doesn't need to be updated, we just compute and return an updated "LoadBalancerStatus" object.
	if !wasChanged {
		st.logger.Debugf("edgelb pool %q is synced", pool.Name)
		return computeLoadBalancerStatus(st.manager, pool.Name, st.service), nil
	}

	// At this point we know that the pool must be either updated or deleted.

	ctx, fn := context.WithTimeout(context.Background(), defaultEdgeLBManagerTimeout)
	defer fn()

	// If the pool is empty (i.e. it has no frontends or backends) we proceed to deleting it and reporting an empty status.
	if len(pool.Haproxy.Frontends) == 0 && len(pool.Haproxy.Backends) == 0 {
		// The pool is empty, so we must delete it.
		st.logger.Debugf("edgelb pool %q is empty and must be deleted", pool.Name)
		if err := st.manager.DeletePool(ctx, pool.Name); err != nil {
			return nil, err
		}
		return &corev1.LoadBalancerStatus{}, nil
	}

	// The pool is not empty, so we proceed to actually updating it and reporting its status.
	st.logger.Debugf("edgelb pool %q must be updated", pool.Name)
	if _, err := st.manager.UpdatePool(ctx, pool); err != nil {
		return nil, err
	}
	return computeLoadBalancerStatus(st.manager, pool.Name, st.service), nil
}

// createEdgeLBPoolObject creates an EdgeLB pool object that satisfies the current Service resource.
func (st *ServiceTranslator) createEdgeLBPoolObject() (*models.V2Pool, error) {
	backends := make([]*models.V2Backend, 0, len(st.service.Spec.Ports))
	frontends := make([]*models.V2Frontend, 0, len(st.service.Spec.Ports))

	// Iterate over port definitions and create the corresponding backend and frontend objects.
	for _, port := range st.service.Spec.Ports {
		// Compute the backend and frontend for the current service port.
		backend, frontend := computeBackendForServicePort(st.service, port), computeFrontendForServicePort(st.service, *st.spec, port)
		// Append the backend to the slice of backends.
		backends = append(backends, backend)
		// Append the frontend to the slice of frontends.
		frontends = append(frontends, frontend)
	}

	// Create and return the pool object.
	p := &models.V2Pool{
		Name:      *st.spec.Name,
		Namespace: &st.poolGroup,
		Role:      *st.spec.Role,
		Cpus:      *st.spec.CPUs,
		Mem:       *st.spec.Memory,
		Count:     st.spec.Size,
		Haproxy: &models.V2Haproxy{
			Backends:  backends,
			Frontends: frontends,
			Stats: &models.V2Stats{
				BindPort: pointers.NewInt32(0),
			},
		},
	}

	// Setup EdgeLB Marathon constraints if applicable.
	if st.spec.Constraints != nil {
		p.Constraints = st.spec.Constraints
	}

	// Request for a cloud load-balancer to be configured if applicable.
	if *st.spec.CloudProviderConfiguration != "" {
		o, err := st.unmarshalCloudProviderObject(*st.spec.CloudProviderConfiguration)
		if err != nil {
			return nil, err
		}
		p.CloudProvider = o
	}

	// Request for the pool to join the requested DC/OS virtual network if applicable.
	if *st.spec.Network != constants.EdgeLBHostNetwork {
		p.VirtualNetworks = []*models.V2PoolVirtualNetworksItems0{
			{
				Name: *st.spec.Network,
			},
		}
	}
	return p, nil
}

// updateEdgeLBPoolObject updates the specified pool object in order to reflect the status of the current Service resource.
// It modifies the specified pool in-place and returns a value indicating whether the pool object contains changes.
// Backends and frontends (the "objects") are added/modified/deleted according to the following rules:
// * If the object is not owned by the current Service resource, it is left untouched.
// * If the current Service resource has been marked for deletion or is not of type "LoadBalancer" anymore, it is removed.
// * If the object is owned by the current Service resource but the corresponding service port has disappeared, it is removed.
// * If the object is owned by the current Service resource and the corresponding service port still exists, it is checked for correctness and updated if necessary.
// Furthermore, service ports are iterated over in order to understand which objects must be added to the pool.
func (st *ServiceTranslator) updateEdgeLBPoolObject(pool *models.V2Pool) (wasChanged bool, report poolInspectionReport, err error) {
	// serviceDeleted holds whether the Service resource has been deleted (or changed to a different type, which must produce a similar effect).
	serviceDeleted := st.service.DeletionTimestamp != nil || st.service.Spec.Type != corev1.ServiceTypeLoadBalancer

	// If the service has not been deleted, we iterate over ports defined on the service and re-compute the corresponding backend and frontend objects.
	// These will be later compared with the backend and frontend objects reported by the EdgeLB API server (i.e. those in "pool").
	desiredBackendFrontends := make(map[int32]servicePortBackendFrontend, len(st.service.Spec.Ports))
	if !serviceDeleted {
		for _, port := range st.service.Spec.Ports {
			desiredBackendFrontends[port.Port] = servicePortBackendFrontend{
				Backend:  computeBackendForServicePort(st.service, port),
				Frontend: computeFrontendForServicePort(st.service, *st.spec, port),
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
		if err != nil || !backendMetadata.IsOwnedBy(st.service) {
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
		if err != nil || !frontendMetadata.IsOwnedBy(st.service) {
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
		return wasChanged, report, err
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

	// Update the CPU request as necessary.
	desiredCpus := *st.spec.CPUs
	if pool.Cpus != desiredCpus {
		pool.Cpus = desiredCpus
		wasChanged = true
		report.Report("must update the cpu request")
	}
	// Update the memory request as necessary.
	desiredMem := *st.spec.Memory
	if pool.Mem != desiredMem {
		pool.Mem = desiredMem
		wasChanged = true
		report.Report("must update the memory request")
	}
	// Update the size request as necessary.
	desiredSize := *st.spec.Size
	if *pool.Count != desiredSize {
		pool.Count = &desiredSize
		wasChanged = true
		report.Report("must update the size request")
	}

	// Update the cloud-provider configuration as required.
	if *st.spec.CloudProviderConfiguration != "" {
		// Grab the current value of the ".cloudProvider" field.
		currentCloudLoadBalancerObject := pool.CloudProvider
		// Compute the desired value for the ".cloudProvider field.
		desiredCloudLoadBalancerObject, err := st.unmarshalCloudProviderObject(*st.spec.CloudProviderConfiguration)
		if err != nil {
			return false, report, err
		}
		// Update the pool with the desired configuration if and only if the current and desired configurations differ.
		if reflect.DeepEqual(currentCloudLoadBalancerObject, desiredCloudLoadBalancerObject) {
			report.Report("no changes to the cloud-provider configuration are required")
		} else {
			pool.CloudProvider = desiredCloudLoadBalancerObject
			wasChanged = true
			report.Report("must update the cloud-provider configuration")
		}
	}

	// Return a value indicating whether the pool was changed, and the pool inspection report.
	return wasChanged, report, err
}
