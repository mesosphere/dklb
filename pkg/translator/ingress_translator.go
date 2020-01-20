package translator

import (
	"context"
	"encoding/json"
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
	translatorapi "github.com/mesosphere/dklb/pkg/translator/api"
	kubernetesutil "github.com/mesosphere/dklb/pkg/util/kubernetes"
	"github.com/mesosphere/dklb/pkg/util/pointers"
	"github.com/mesosphere/dklb/pkg/util/prettyprint"
)

type OperationResult string

const (
	OperationResultNone    OperationResult = "unchanged"
	OperationResultUpdated OperationResult = "updated"
	OperationResultDeleted OperationResult = "deleted"
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

	prettyprint.LogfJSON(log.Tracef, spec, "edgelb pool configuration object for %q", kubernetesutil.Key(it.ingress))

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
	log.Printf("searching for backend.servicePort={%+v}", backend.ServicePort)
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
	// Print the compputed EdgeLB pool object in  JSON format.
	prettyprint.LogfJSON(log.Debugf, pool, "computed edgelb pool object for ingress %q", kubernetesutil.Key(it.ingress))
	ctx, fn := context.WithTimeout(context.Background(), defaultEdgeLBManagerTimeout)
	defer fn()
	if _, err := it.manager.CreatePool(ctx, pool); err != nil {
		return nil, err
	}
	// Compute and return the status of the load-balancer.
	return computeLoadBalancerStatus(it.manager, pool.Name, it.ingress, nil), nil
}

// updateOrDeleteEdgeLBPool makes a decision on whether the specified EdgeLB pool should be updated/deleted based on the current status of the associated Ingress resource.
// In case it should be updated/deleted, it proceeds to actually updating/deleting it.
// TODO (@bcustodio) Decide whether we should also update the EdgeLB pool's role and its CPU/memory/size requests.
func (it *IngressTranslator) updateOrDeleteEdgeLBPool(pool *models.V2Pool, backendMap IngressBackendNodePortMap) (*corev1.LoadBalancerStatus, error) {
	// Check whether the EdgeLB pool object must be updated.
	opResult, desiredFrontends := it.updateEdgeLBPoolObject(pool, backendMap)

	b, _ := json.Marshal(pool)
	log.WithField("pool", string(b)).Infof("computed updated edgelb pool")

	// If the EdgeLB pool doesn't need to be updated, we just compute and return an updated "LoadBalancerStatus" object.
	if opResult == OperationResultNone {
		it.logger.Debugf("edgelb pool %q is synced", pool.Name)
		return computeLoadBalancerStatus(it.manager, pool.Name, it.ingress, desiredFrontends), nil
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
	return computeLoadBalancerStatus(it.manager, pool.Name, it.ingress, desiredFrontends), nil
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
	frontends := computeEdgeLBFrontendForIngress(it.ingress, *it.spec, nil)
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

	// Setup EdgeLB Marathon constraints if applicable.
	if it.spec.Constraints != nil {
		p.Constraints = it.spec.Constraints
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

// checkIfDesired is a helper function that returns true if the list of frontends
// contains a frontend with the same name as desired.
func checkIfDesired(frontends []*models.V2Frontend, desired *models.V2Frontend) *models.V2Frontend {
	for _, f := range frontends {
		if f.Name == desired.Name {
			return f
		}
	}
	return nil
}

func checkIfReferencesBackend(backends []*models.V2Backend, name string) bool {
	for _, b := range backends {
		if b.Name == name {
			return true
		}
	}
	return false
}

func computeEdgeLBBackendForIngress(ingress *extsv1beta1.Ingress, backendMap IngressBackendNodePortMap) []*models.V2Backend {
	desiredBackends := make([]*models.V2Backend, 0)
	for ingressBackend, nodePort := range backendMap {
		desiredBackend := computeEdgeLBBackendForIngressBackend(ingress, ingressBackend, nodePort)
		desiredBackends = append(desiredBackends, desiredBackend)
	}
	return desiredBackends
}

func computeUnmanagedBackendsForIngress(ingress *extsv1beta1.Ingress, pool *models.V2Pool) []*models.V2Backend {
	unmanagedBackends := make([]*models.V2Backend, 0)
	for _, backend := range pool.Haproxy.Backends {
		backendMetadata := computeIngressOwnedEdgeLBObjectMetadata(backend.Name)
		if !backendMetadata.IsOwnedBy(ingress) {
			unmanagedBackends = append(unmanagedBackends, backend)
		}
	}
	return unmanagedBackends
}

func computeUnmanagedFrontendsForIngress(ingress *extsv1beta1.Ingress, pool *models.V2Pool, desiredFrontends []*models.V2Frontend) []*models.V2Frontend {
	unmanagedFrontends := make([]*models.V2Frontend, 0)
	for _, frontend := range pool.Haproxy.Frontends {
		desiredFrontend := checkIfDesired(desiredFrontends, frontend)
		frontendMetadata := computeIngressOwnedEdgeLBObjectMetadata(frontend.Name)

		if desiredFrontend == nil && !frontendMetadata.IsOwnedBy(ingress) {
			unmanagedFrontends = append(unmanagedFrontends, frontend)
		}
	}
	return unmanagedFrontends
}

func computeUnmanagedSecrets(ingress *extsv1beta1.Ingress, pool *models.V2Pool) []*models.V2PoolSecretsItems0 {
	unmanagedSecrets := make([]*models.V2PoolSecretsItems0, 0)

	// Iterate of the pool's secrets and check whether each one is owned by the
	// current ingress
	for _, secret := range pool.Secrets {
		if !strings.HasPrefix(secret.Secret, string(ingress.UID)) {
			unmanagedSecrets = append(unmanagedSecrets, secret)
		}
	}

	return unmanagedSecrets
}

func filterUnreferencedBackends(backends []*models.V2Backend, frontends []*models.V2Frontend) []*models.V2Backend {
	referencedBackends := make([]*models.V2Backend, 0)
	references := make(map[string]string)
	for _, f := range frontends {
		if f.LinkBackend.DefaultBackend != "" {
			references[f.LinkBackend.DefaultBackend] = ""
		}
		for _, b := range f.LinkBackend.Map {
			references[b.Backend] = ""
		}
	}

	for _, b := range backends {
		if _, ok := references[b.Name]; ok {
			referencedBackends = append(referencedBackends, b)
		}
	}

	return referencedBackends
}

func filterReferencedBackends(backends []*models.V2Backend, frontends []*models.V2Frontend) []*models.V2Frontend {
	updatedFrontends := make([]*models.V2Frontend, 0)
	references := make(map[string]string)
	for _, b := range backends {
		log.Debugf("found reference to backend %s", b.Name)
		references[b.Name] = ""
	}

	for _, f := range frontends {
		if _, ok := references[f.LinkBackend.DefaultBackend]; ok {
			f.LinkBackend.DefaultBackend = ""
		}
		linkBackendMap := make([]*models.V2FrontendLinkBackendMapItems0, 0)
		for _, b := range f.LinkBackend.Map {
			if _, ok := references[b.Backend]; ok {
				log.Debugf("removing backend %s from frontend %s", b.Backend, f.Name)
			} else {
				log.Debugf("adding unmanaged backend %s", b.Backend)
				linkBackendMap = append(linkBackendMap, b)
			}
		}
		if len(linkBackendMap) == 0 && f.LinkBackend.DefaultBackend == "" {
			log.Debugf("frontend %s is empty, removing", f.Name)
		} else {
			f.LinkBackend.Map = linkBackendMap
			updatedFrontends = append(updatedFrontends, f)
		}
	}
	return updatedFrontends
}

// updateEdgeLBPoolObject updates the specified EdgeLB pool object in order to reflect the status of the current Ingress resource.
// It modifies the specified EdgeLB pool in-place and returns a value indicating whether the EdgeLB pool object contains changes.
// EdgeLB backends and frontends (the "objects") are added/modified/deleted according to the following rules:
// * If the object is not owned by the current Ingress resource, it is left untouched.
// * If the current Ingress resource has been marked for deletion, it is removed.
// * If the object is an EdgeLB backend owned by the current Ingress resource but the corresponding Ingress backend has disappeared, it is removed.
// * If the object is owned by the current Ingress resource and the corresponding Ingress backend still exists (or the object is an EdgeLB frontend), it is checked for correctness and updated if necessary.
// Furthermore, Ingress backends are iterated over in order to understand which EdgeLB backends must be added to the EdgeLB pool.
func (it *IngressTranslator) updateEdgeLBPoolObject(pool *models.V2Pool, backendMap IngressBackendNodePortMap) (operationResult OperationResult, desiredFrontends []*models.V2Frontend) {
	// ingressDeleted holds whether the Ingress resource has been deleted or its "kubernetes.io/ingress.class" has changed.
	ingressDeleted := it.ingress.DeletionTimestamp != nil || it.ingress.Annotations[constants.EdgeLBIngressClassAnnotationKey] != constants.EdgeLBIngressClassAnnotationValue
	log.Debugf("ingress is being deleted? %v", ingressDeleted)
	operationResult = OperationResultNone

	desiredBackends := computeEdgeLBBackendForIngress(it.ingress, backendMap)
	desiredFrontends = computeEdgeLBFrontendForIngress(it.ingress, *it.spec, pool)
	desiredSecrets := computeEdgeLBSecretsForIngress(it.ingress)

	backends := computeUnmanagedBackendsForIngress(it.ingress, pool)
	frontends := computeUnmanagedFrontendsForIngress(it.ingress, pool, desiredFrontends)
	secrets := computeUnmanagedSecrets(it.ingress, pool)

	if ingressDeleted {
		frontends = filterReferencedBackends(desiredBackends, frontends)
		desiredFrontends = filterReferencedBackends(desiredBackends, desiredFrontends)
		frontends = append(frontends, desiredFrontends...)
	} else {
		backends = append(backends, desiredBackends...)
		frontends = append(frontends, desiredFrontends...)
		secrets = append(secrets, desiredSecrets...)
	}

	backends = filterUnreferencedBackends(backends, frontends)

	prettyprint.LogfSpew(log.Tracef, backends, "computed backends for ingress %q", kubernetesutil.Key(it.ingress))
	prettyprint.LogfSpew(log.Tracef, frontends, "computed frontends for ingress %q", kubernetesutil.Key(it.ingress))

	// sort everything to have predictable objects
	sort.SliceStable(backends, func(i, j int) bool {
		return backends[i].Name < backends[j].Name
	})

	sort.SliceStable(frontends, func(i, j int) bool {
		return frontends[i].Name < frontends[j].Name
	})

	sort.SliceStable(secrets, func(i, j int) bool {
		return secrets[i].File < secrets[j].File
	})

	// check if anything changed
	if !reflect.DeepEqual(pool.Haproxy.Backends, backends) {
		operationResult = OperationResultUpdated
		pool.Haproxy.Backends = backends
	}

	if !reflect.DeepEqual(pool.Haproxy.Frontends, frontends) {
		operationResult = OperationResultUpdated
		pool.Haproxy.Frontends = frontends
	}

	if !reflect.DeepEqual(pool.Secrets, secrets) {
		operationResult = OperationResultUpdated
		pool.Secrets = secrets
	}

	// a pool is deleted if it doesn't have any backends and frontends, in that
	// case we don't need to compute the status of the edgelb so we return now
	log.Tracef("len(frontends)=%d len(backends)=%d", len(pool.Haproxy.Frontends), len(pool.Haproxy.Backends))
	if len(pool.Haproxy.Frontends) == 0 && len(pool.Haproxy.Backends) == 0 {
		return OperationResultDeleted, nil
	}

	// Update the CPU request as necessary.
	desiredCpus := *it.spec.CPUs
	if pool.Cpus != desiredCpus {
		pool.Cpus = desiredCpus
		operationResult = OperationResultUpdated
		log.Debugf("must update the cpu request")
	}
	// Update the memory request as necessary.
	desiredMem := *it.spec.Memory
	if pool.Mem != desiredMem {
		pool.Mem = desiredMem
		operationResult = OperationResultUpdated
		log.Debugf("must update the memory request")
	}
	// Update the size request as necessary.
	desiredSize := *it.spec.Size
	if *pool.Count != desiredSize {
		pool.Count = &desiredSize
		operationResult = OperationResultUpdated
		log.Debugf("must update the size request")
	}

	// Return a value indicating whether the pool was changed and the desired frontend configuration
	return operationResult, desiredFrontends
}
