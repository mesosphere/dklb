package translator

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/mesosphere/dcos-edge-lb/pkg/apis/models"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"

	dklbcache "github.com/mesosphere/dklb/pkg/cache"
	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/edgelb/manager"
	dklberrors "github.com/mesosphere/dklb/pkg/errors"
	secretsreflector "github.com/mesosphere/dklb/pkg/secrets_reflector"
	translatorapi "github.com/mesosphere/dklb/pkg/translator/api"
	kubernetesutil "github.com/mesosphere/dklb/pkg/util/kubernetes"
	"github.com/mesosphere/dklb/pkg/util/pointers"
	"github.com/mesosphere/dklb/pkg/util/prettyprint"
)

var (
	// defaultBackendServiceName is the value used internally as ".serviceName" to signal the fact that dklb should be used as the default backend.
	// It will also end up being used as part of the name of an EdgeLB backend whenever an Ingress resource doesn't define a default backend or a referenced Service resource is missing or otherwise invalid.
	defaultBackendServiceName = "default-backend"
	// defaultBackendServicePort is the value used internally as ".servicePort" to signal the fact that dklb should be used as the default backend.
	// It will also end up being used as part of the name of an EdgeLB backend whenever an Ingress resource doesn't define a default backend or a referenced Service resource is missing or otherwise invalid.
	defaultBackendServicePort = intstr.FromInt(0)
)

// IngressTranslator is the base implementation of IngressTranslator.
type IngressTranslator struct {
	// ingress is the Ingress resource to be translated.
	ingress *extsv1beta1.Ingress
	// spec is the EdgeLB pool configuration object to use when performing translation.
	spec *translatorapi.IngressEdgeLBPoolSpec
	// kubeCache is the instance of the Kubernetes resource cache to use.
	kubeCache dklbcache.KubernetesResourceCache
	// manager is the instance of the EdgeLB manager to use for managing EdgeLB pools.
	manager manager.EdgeLBManager
	// logger is the logger to use when performing translation.
	logger *log.Entry
	// recorder is the event recorder used to emit events associated with a given Ingress resource.
	recorder record.EventRecorder
	// poolGroup is the DC/OS service group in which to create EdgeLB pools.
	poolGroup string
}

// NewIngressTranslator returns an ingress translator that can be used to translate the specified Ingress resource into an EdgeLB pool.
func NewIngressTranslator(ingress *extsv1beta1.Ingress, kubeCache dklbcache.KubernetesResourceCache, manager manager.EdgeLBManager, recorder record.EventRecorder) *IngressTranslator {
	return &IngressTranslator{
		// Use a clone of the Ingress resource as we may need to modify it in order to inject the default backend.
		ingress:   ingress.DeepCopy(),
		kubeCache: kubeCache,
		manager:   manager,
		logger:    log.WithField("ingress", kubernetesutil.Key(ingress)),
		recorder:  recorder,
		poolGroup: manager.PoolGroup(),
	}
}

// Translate performs translation of the associated Ingress resource into an EdgeLB pool.
func (it *IngressTranslator) Translate() (*corev1.LoadBalancerStatus, error) {
	// Grab the EdgeLB pool configuration object from the Ingress resource.
	spec, err := translatorapi.GetIngressEdgeLBPoolSpec(it.ingress)
	if err != nil {
		return nil, fmt.Errorf("the edgelb pool configuration object is not valid: %v", err)
	}
	it.spec = spec

	// Dump the EdgeLB pool configuration object for debugging purposes.
	prettyprint.LogfSpew(log.Tracef, spec, "edgelb pool configuration object for %q", kubernetesutil.Key(it.ingress))

	// Attempt to determine the node port at which the default backend is exposed.
	defaultBackendNodePort, err := it.determineDefaultBackendNodePort()
	if err != nil {
		return nil, err
	}

	// Compute the mapping between Ingress backends defined on the current Ingress resource and their target node ports.
	backendMap := it.computeIngressBackendNodePortMap(defaultBackendNodePort)

	// Check whether an EdgeLB pool with the requested name already exists in EdgeLB.
	ctx, fn := context.WithTimeout(context.Background(), defaultEdgeLBManagerTimeout)
	defer fn()
	pool, err := it.manager.GetPool(ctx, *it.spec.Name)
	if err != nil {
		if !dklberrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to check for the existence of the %q edgelb pool: %v", *it.spec.Name, err)
		}
	}
	// If the target EdgeLB pool does not exist, we must try to create it,
	if pool == nil {
		return it.createEdgeLBPool(backendMap)
	}
	// If the target EdgeLB pool already exists, we must check whether it needs to be updated/deleted.
	return it.updateOrDeleteEdgeLBPool(pool, backendMap)
}

// determineDefaultBackendNodePort attempts to determine the node port at which the default backend is exposed.
func (it *IngressTranslator) determineDefaultBackendNodePort() (int32, error) {
	s, err := it.kubeCache.GetService(constants.KubeSystemNamespaceName, constants.DefaultBackendServiceName)
	if err != nil {
		return 0, fmt.Errorf("failed to read the \"%s/%s\" service: %v", constants.KubeSystemNamespaceName, constants.DefaultBackendServiceName, err)
	}
	if s.Spec.Type != corev1.ServiceTypeNodePort && s.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return 0, fmt.Errorf("service %q is of unexpected type %q", kubernetesutil.Key(s), s.Spec.Type)
	}
	for _, port := range s.Spec.Ports {
		if port.Port == constants.DefaultBackendServicePort && port.NodePort > 0 {
			return port.NodePort, nil
		}
	}
	return 0, fmt.Errorf("no valid node port has been assigned to the default backend")
}

// computeIngressBackendNodePortMap computes the mapping between (unique) Ingress backends defined on the current Ingress resource and their target node ports.
// It starts by compiling a set of all (possibly duplicate) Ingress backends defined on the Ingress resource.
// In case a default backend hasn't been specified, dklb's default backend is injected as the default one.
// Then, it iterates over said set and checks whether the referenced service port exists, adding them to the map or using the default backend's node port instead.
// As the returned object is in fact a map, duplicate Ingress backends are automatically removed.
func (it *IngressTranslator) computeIngressBackendNodePortMap(defaultBackendNodePort int32) IngressBackendNodePortMap {
	// Inject dklb as the default backend in case none is specified.
	if it.ingress.Spec.Backend == nil {
		it.ingress.Spec.Backend = &extsv1beta1.IngressBackend{
			ServiceName: defaultBackendServiceName,
			ServicePort: defaultBackendServicePort,
		}
		it.recorder.Eventf(it.ingress, corev1.EventTypeWarning, constants.ReasonNoDefaultBackendSpecified, "%s will be used as the default backend since none was specified", constants.ComponentName)
	}
	// backends is the slice containing all Ingress backends present in the current Ingress resource.
	backends := make([]extsv1beta1.IngressBackend, 0)
	// Iterate over all Ingress backends, adding them to the slice of results.
	kubernetesutil.ForEachIngresBackend(it.ingress, func(_, _ *string, backend extsv1beta1.IngressBackend) {
		backends = append(backends, backend)
	})
	// Create the map that we will be populating and returning.
	res := make(IngressBackendNodePortMap, len(backends))
	// Iterate over the set of Ingress backends, computing the target node port.
	for _, backend := range backends {
		// If the target service's name corresponds to "defaultBackendServiceName", we use the default backend's node port.
		if backend.ServiceName == defaultBackendServiceName && backend.ServicePort == defaultBackendServicePort {
			res[backend] = defaultBackendNodePort
			continue
		}
		if nodePort, err := it.computeNodePortForIngressBackend(backend); err == nil {
			res[backend] = nodePort
		} else {
			// We've failed to compute the target node port for the current backend.
			// This may be caused by the specified Service resource being absent or not being of NodePort/LoadBalancer type.
			// Hence, we use the default backend's node port and report the error as an event, but do not fail.
			msg := fmt.Sprintf("using the default backend in place of \"%s:%s\": %v", backend.ServiceName, backend.ServicePort.String(), err)
			it.recorder.Eventf(it.ingress, corev1.EventTypeWarning, constants.ReasonInvalidBackendService, msg)
			it.logger.Warn(msg)
			res[backend] = defaultBackendNodePort
		}
	}
	// Return the populated map.
	return res
}

// computeNodePortForIngressBackend computes the node port targeted by the specified Ingress backend.
func (it *IngressTranslator) computeNodePortForIngressBackend(backend extsv1beta1.IngressBackend) (int32, error) {
	// Check whether the referenced Service resource exists.
	s, err := it.kubeCache.GetService(it.ingress.Namespace, backend.ServiceName)
	if err != nil {
		return 0, fmt.Errorf("failed to read service %q referenced by ingress %q: %v", backend.ServiceName, kubernetesutil.Key(it.ingress), err)
	}
	// Check whether the referenced Service resource is of type "NodePort" or "LoadBalancer".
	if s.Spec.Type != corev1.ServiceTypeNodePort && s.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return 0, fmt.Errorf("service %q referenced by ingress %q is of unexpected type %q", backend.ServiceName, kubernetesutil.Key(it.ingress), s.Spec.Type)
	}
	// Lookup the referenced service port.
	var servicePort *corev1.ServicePort
	for _, port := range s.Spec.Ports {
		// Pin "port" so we can take its address.
		port := port
		if port.Port == backend.ServicePort.IntVal || port.Name == backend.ServicePort.StrVal {
			servicePort = &port
		}
	}
	// Check whether the referenced service port has been found.
	if servicePort == nil {
		return 0, fmt.Errorf("port %q of service %q referenced by ingress %q not found", backend.ServicePort.String(), backend.ServiceName, kubernetesutil.Key(it.ingress))
	}
	return servicePort.NodePort, nil
}

// createEdgeLBPool makes a decision on whether an EdgeLB pool should be created for the associated Ingress resource.
// This decision is based on the EdgeLB pool creation strategy specified for the Ingress resource.
// In case it should be created, it proceeds to actually creating it.
func (it *IngressTranslator) createEdgeLBPool(backendMap IngressBackendNodePortMap) (*corev1.LoadBalancerStatus, error) {
	// If the pool creation strategy is "Never", the target EdgeLB pool must be provisioned manually.
	// Hence, we should just exit.
	if *it.spec.Strategies.Creation == translatorapi.EdgeLBPoolCreationStrategyNever {
		return nil, fmt.Errorf("edgelb pool %q targeted by ingress %q does not exist, but the pool creation strategy is %q", *it.spec.Name, kubernetesutil.Key(it.ingress), *it.spec.Strategies.Creation)
	}

	// If the Ingress resource's ".status" field contains at least one IP/host, that means an EdgeLB pool has once existed, but has been deleted manually.
	// Hence, and if the EdgeLB pool creation strategy is "Once", we should also just exit.
	if len(it.ingress.Status.LoadBalancer.Ingress) > 0 && *it.spec.Strategies.Creation == translatorapi.EdgeLBPoolCreationStrategyOnce {
		return nil, fmt.Errorf("edgelb pool %q targeted by ingress %q has probably been manually deleted, and the pool creation strategy is %q", *it.spec.Name, kubernetesutil.Key(it.ingress), *it.spec.Strategies.Creation)
	}

	// At this point, we know that we must create the target EdgeLB pool based on the specified options and Ingress backend map.
	pool := it.createEdgeLBPoolObject(backendMap)
	// Print the compputed EdgeLB pool object in "spew" and JSON formats.
	prettyprint.LogfSpew(log.Tracef, pool, "computed edgelb pool object for ingress %q", kubernetesutil.Key(it.ingress))
	prettyprint.LogfJSON(log.Debugf, pool, "computed edgelb pool object for ingress %q", kubernetesutil.Key(it.ingress))
	ctx, fn := context.WithTimeout(context.Background(), defaultEdgeLBManagerTimeout)
	defer fn()
	if _, err := it.manager.CreatePool(ctx, pool); err != nil {
		return nil, err
	}
	// Compute and return the status of the load-balancer.
	return computeLoadBalancerStatus(it.manager, pool.Name, it.ingress), nil
}

// updateOrDeleteEdgeLBPool makes a decision on whether the specified EdgeLB pool should be updated/deleted based on the current status of the associated Ingress resource.
// In case it should be updated/deleted, it proceeds to actually updating/deleting it.
// TODO (@bcustodio) Decide whether we should also update the EdgeLB pool's role and its CPU/memory/size requests.
func (it *IngressTranslator) updateOrDeleteEdgeLBPool(pool *models.V2Pool, backendMap IngressBackendNodePortMap) (*corev1.LoadBalancerStatus, error) {
	// Check whether the EdgeLB pool object must be updated.
	wasChanged, report := it.updateEdgeLBPoolObject(pool, backendMap)
	// Report the status of the EdgeLB pool.
	prettyprint.LogfSpew(log.Tracef, report, "inspection report for edgelb pool %q", pool.Name)
	// Print the compputed EdgeLB pool object in "spew" and JSON formats.
	prettyprint.LogfSpew(log.Tracef, pool, "computed edgelb pool object for ingress %q", kubernetesutil.Key(it.ingress))
	prettyprint.LogfJSON(log.Debugf, pool, "computed edgelb pool object for ingress %q", kubernetesutil.Key(it.ingress))

	// If the EdgeLB pool doesn't need to be updated, we just compute and return an updated "LoadBalancerStatus" object.
	if !wasChanged {
		it.logger.Debugf("edgelb pool %q is synced", pool.Name)
		return computeLoadBalancerStatus(it.manager, pool.Name, it.ingress), nil
	}

	// At this point we know that the EdgeLB pool must be either updated or deleted.

	ctx, fn := context.WithTimeout(context.Background(), defaultEdgeLBManagerTimeout)
	defer fn()

	// If the EdgeLB pool is empty (i.e. it has no EdgeLB frontends or EdgeLB backends) we proceed to deleting it and reporting an empty status.
	if len(pool.Haproxy.Frontends) == 0 && len(pool.Haproxy.Backends) == 0 {
		// The EdgeLB pool is empty, so we delete it.
		it.logger.Debugf("edgelb pool %q is empty and must be deleted", pool.Name)
		if err := it.manager.DeletePool(ctx, pool.Name); err != nil {
			return nil, err
		}
		return &corev1.LoadBalancerStatus{}, nil
	}

	// The pool is not empty, so we proceed to actually updating it and reporting its status.
	it.logger.Debugf("edgelb pool %q must be updated", pool.Name)
	if _, err := it.manager.UpdatePool(ctx, pool); err != nil {
		return nil, err
	}
	return computeLoadBalancerStatus(it.manager, pool.Name, it.ingress), nil
}

// createEdgeLBPoolObject creates an EdgeLB pool object that satisfies the current Ingress resource.
func (it *IngressTranslator) createEdgeLBPoolObject(backendMap IngressBackendNodePortMap) *models.V2Pool {
	// Iterate over Ingress backends and their target node ports, and create the corresponding EdgeLB backend objects.
	backends := make([]*models.V2Backend, 0, len(backendMap))
	for backend, nodePort := range backendMap {
		backends = append(backends, computeEdgeLBBackendForIngressBackend(it.ingress, backend, nodePort))
	}
	// Sort backends alphabetically in order to get a predictable output, as ranging over a map can produce different results every time.
	sort.SliceStable(backends, func(i, j int) bool {
		return backends[i].Name < backends[j].Name
	})
	// Create the EdgeLB frontend object.
	frontends := computeEdgeLBFrontendForIngress(it.ingress, *it.spec)
	secrets := computeEdgeLBSecretsForIngress(it.ingress)
	// Create the base EdgeLB pool object.
	p := &models.V2Pool{
		Name:      *it.spec.Name,
		Namespace: &it.poolGroup,
		Role:      *it.spec.Role,
		Cpus:      *it.spec.CPUs,
		Mem:       *it.spec.Memory,
		Count:     it.spec.Size,
		Haproxy: &models.V2Haproxy{
			Backends:  backends,
			Frontends: frontends,
			Stats: &models.V2Stats{
				BindPort: pointers.NewInt32(0),
			},
		},
		Secrets: secrets,
	}
	// Request for the EdgeLB pool to join the requested DC/OS virtual network if applicable.
	if *it.spec.Network != constants.EdgeLBHostNetwork {
		p.VirtualNetworks = []*models.V2PoolVirtualNetworksItems0{
			{
				Name: *it.spec.Network,
			},
		}
	}
	return p
}

// updateEdgeLBPoolObject updates the specified EdgeLB pool object in order to reflect the status of the current Ingress resource.
// It modifies the specified EdgeLB pool in-place and returns a value indicating whether the EdgeLB pool object contains changes.
// EdgeLB backends and frontends (the "objects") are added/modified/deleted according to the following rules:
// * If the object is not owned by the current Ingress resource, it is left untouched.
// * If the current Ingress resource has been marked for deletion, it is removed.
// * If the object is an EdgeLB backend owned by the current Ingress resource but the corresponding Ingress backend has disappeared, it is removed.
// * If the object is owned by the current Ingress resource and the corresponding Ingress backend still exists (or the object is an EdgeLB frontend), it is checked for correctness and updated if necessary.
// Furthermore, Ingress backends are iterated over in order to understand which EdgeLB backends must be added to the EdgeLB pool.
func (it *IngressTranslator) updateEdgeLBPoolObject(pool *models.V2Pool, backendMap IngressBackendNodePortMap) (wasChanged bool, report poolInspectionReport) {
	// ingressDeleted holds whether the Ingress resource has been deleted or its "kubernetes.io/ingress.class" has changed.
	ingressDeleted := it.ingress.DeletionTimestamp != nil || it.ingress.Annotations[constants.EdgeLBIngressClassAnnotationKey] != constants.EdgeLBIngressClassAnnotationValue

	// visitedBackends holds the set of IngressBackend objects that have been visited (i.e. that exist in "pool").
	// It is used to understand which Ingress backends currently have EdgeLB backends in the EdgeLB pool, and which don't.
	visitedIngressBackends := make(map[extsv1beta1.IngressBackend]bool, len(pool.Haproxy.Backends))
	// updatedBackends holds the set of updated EdgeLB backends.
	// It is used as the final set of EdgeLB backends for the EdgeLB pool if we find out we need to update it.
	updatedBackends := make([]*models.V2Backend, 0, len(pool.Haproxy.Backends))

	// Iterate over the EdgeLB pool's EdgeLB backends and check whether each one is owned by the current Ingress.
	// In case an EdgeLB backend isn't owned by the current Ingress, it is left unchanged and added to the set of "updated" EdgeLB backends.
	// Otherwise, it is checked for correctness and, if necessary, replaced with the computed EdgeLB backend for the target Ingress backend.
	for _, backend := range pool.Haproxy.Backends {
		// Parse the name of the EdgeLB backend in order to determine if the current Ingress owns it.
		// If the current EdgeLB backend isn't owned by the current Ingress, it is left unchanged.
		backendMetadata, err := computeIngressOwnedEdgeLBObjectMetadata(backend.Name)
		if err != nil || !backendMetadata.IsOwnedBy(it.ingress) {
			updatedBackends = append(updatedBackends, backend)
			report.Report("no changes required for backend %q (not owned by %s)", backend.Name, kubernetesutil.Key(it.ingress))
			continue
		}
		// At this point we know the current EdgeLB backend is owned by the current Ingress.
		// Check whether the Ingress resource has been deleted, and skip (i.e. remove) the EdgeLB backend if it has.
		if ingressDeleted {
			wasChanged = true
			report.Report("must delete backend %q as %q was deleted", backend.Name, kubernetesutil.Key(it.ingress))
			continue
		}
		// Check whether the Ingress backend that corresponds to the current EdgeLB backend is still present in the Ingress resource.
		// If it doesn't, skip (i.e. remove) the EdgeLB backend.
		currentIngressBackend := *backendMetadata.IngressBackend
		if _, exists := backendMap[currentIngressBackend]; !exists {
			wasChanged = true
			report.Report("must delete edgelb backend %q as the corresponding ingress backend is missing from %q", backend.Name, kubernetesutil.Key(it.ingress))
			continue
		}
		// At this point we know the Ingress backend corresponding to the current EdgeLB backend still exists.
		// Mark the current Ingress backend as having been visited.
		visitedIngressBackends[currentIngressBackend] = true
		// Compute the desired state for the current EdgeLB backend.
		// In case differences are detected, we replace  the existing EdgeLB backend with the computed one.
		desiredBackend := computeEdgeLBBackendForIngressBackend(it.ingress, currentIngressBackend, backendMap[currentIngressBackend])
		if !reflect.DeepEqual(backend, desiredBackend) {
			wasChanged = true
			updatedBackends = append(updatedBackends, desiredBackend)
			report.Report("must modify backend %q", backend.Name)
		} else {
			updatedBackends = append(updatedBackends, backend)
			report.Report("no changes required for backend %q", backend.Name)
		}
	}

	// Check if pool's frontends match the desired list. If a frontend is not
	// managed by dklb it's added to the list of desiredFrontends.
	desiredFrontends := computeEdgeLBFrontendForIngress(it.ingress, *it.spec)
	updatedFrontends := make([]*models.V2Frontend, 0)
	if !reflect.DeepEqual(desiredFrontends, pool.Haproxy.Frontends) {
		wasChanged = true
		report.Report("frontend list requires an update")
		for _, frontend := range pool.Haproxy.Frontends {
			frontendMetadata, err := computeIngressOwnedEdgeLBObjectMetadata(frontend.Name)
			if err != nil || !frontendMetadata.IsOwnedBy(it.ingress) {
				wasChanged = true
				updatedFrontends = append(updatedFrontends, frontend)
				report.Report("added unmanaged frontend %q", frontend.Name)
			}
		}
	}

	// Iterate of the pool's secrets and check whether each one is owned by the current ingress
	desiredSecrets := computeEdgeLBSecretsForIngress(it.ingress)
	updatedSecrets := make([]*models.V2PoolSecretsItems0, 0)
	// Compute the dcos secret name using the empty string. This
	// will return the prefix used when creating the secret. We'll
	// use it check if the secret is managed by dklb or not.
	dcosSecretNamePrefix := secretsreflector.ComputeDCOSSecretName(it.ingress.Namespace, "")
	for _, secret := range pool.Secrets {
		if !strings.HasPrefix(secret.Secret, dcosSecretNamePrefix) {
			wasChanged = true
			report.Report("added unmanaged secret %q", secret.File)
			updatedSecrets = append(updatedSecrets, secret)
		}
	}

	// If the ingress wasn't deleted we concatenate the desired list of
	// frontends and secrets with the list of the unmanaged ones.
	if !ingressDeleted {
		updatedFrontends = append(desiredFrontends, updatedFrontends...)
		updatedSecrets = append(desiredSecrets, updatedSecrets...)
	}

	// Replace the EdgeLB pool's backends, frontends and secrets with the (possibly empty) updated lists.
	pool.Haproxy.Backends = updatedBackends
	pool.Haproxy.Frontends = updatedFrontends
	pool.Secrets = updatedSecrets

	// If the current Ingress resource was deleted, there is nothing else to do.
	if ingressDeleted {
		return wasChanged, report
	}

	// Iterate over all desired Ingress backends in order to understand whether there are new ones.
	// For every Ingress backend, if the corresponding EdgeLB backend is not present in the EdgeLB pool, we add it.
	newBackends := make([]*models.V2Backend, 0)
	for ingressBackend, nodePort := range backendMap {
		if _, visited := visitedIngressBackends[ingressBackend]; !visited {
			wasChanged = true
			desiredBackend := computeEdgeLBBackendForIngressBackend(it.ingress, ingressBackend, nodePort)
			newBackends = append(newBackends, desiredBackend)
			report.Report("must create backend %q", desiredBackend.Name)
		}
	}
	// Sort new EdgeLB backends by their name before adding them to the EdgeLB pool in order to guarantee a predictable order.
	sort.SliceStable(newBackends, func(i, j int) bool {
		return newBackends[i].Name < newBackends[j].Name
	})
	pool.Haproxy.Backends = append(pool.Haproxy.Backends, newBackends...)

	// Update the CPU request as necessary.
	desiredCpus := *it.spec.CPUs
	if pool.Cpus != desiredCpus {
		pool.Cpus = desiredCpus
		wasChanged = true
		report.Report("must update the cpu request")
	}
	// Update the memory request as necessary.
	desiredMem := *it.spec.Memory
	if pool.Mem != desiredMem {
		pool.Mem = desiredMem
		wasChanged = true
		report.Report("must update the memory request")
	}
	// Update the size request as necessary.
	desiredSize := *it.spec.Size
	if *pool.Count != desiredSize {
		pool.Count = &desiredSize
		wasChanged = true
		report.Report("must update the size request")
	}

	// Return a value indicating whether the pool was changed, and the EdgeLB pool inspection report.
	return wasChanged, report
}
