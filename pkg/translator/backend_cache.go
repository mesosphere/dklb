/*
 * Copyright (c) 2018 Mesosphere, Inc
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package translator

import (
	"sort"
	"strconv"
	"sync"

	"k8s.io/api/core/v1"

	"github.com/mesosphere/dcos-edge-lb/models"
)

// backendCache is a thread safe, atomic, copy on write cache of *models.V2Backend objects.
type backendCache struct {
	sync.Mutex
	values []*models.V2Backend
}

// Values returns a copy of the contents of the cache.
func (bc *backendCache) Values() []*models.V2Backend {
	bc.Lock()
	r := append([]*models.V2Backend{}, bc.values...)
	bc.Unlock()
	return r
}

// Add adds an entry to the cache. If a V2Backend with the same
// name exists, it is replaced.
func (bc *backendCache) Add(backends ...*models.V2Backend) {
	if len(backends) == 0 {
		return
	}
	bc.Lock()
	sort.Sort(backendByName(bc.values))
	for _, backend := range backends {
		bc.add(backend)
	}
	bc.Unlock()
}

// add adds backend to the cache. If backend is already present, the cached
// value of backend is overwritten.
// invariant: bc.values should be sorted on entry.
func (bc *backendCache) add(backend *models.V2Backend) {
	i := sort.Search(len(bc.values), func(i int) bool { return bc.values[i].Name >= backend.Name })
	if i < len(bc.values) && bc.values[i].Name == backend.Name {
		// backend is already present, replace
		bc.values[i] = backend
	} else {
		// backend is not present, append
		bc.values = append(bc.values, backend)
		// re-sort to convert append into insert
		sort.Sort(backendByName(bc.values))
	}
}

// Remove removes the named entry from the cache. If the entry
// is not present in the cache, the operation is a no-op.
func (bc *backendCache) Remove(names ...string) {
	if len(names) == 0 {
		return
	}
	bc.Lock()
	sort.Sort(backendByName(bc.values))
	for _, n := range names {
		bc.remove(n)
	}
	bc.Unlock()
}

// remove removes the named entry from the cache.
// invariant: bc.values should be sorted on entry.
func (bc *backendCache) remove(name string) {
	i := sort.Search(len(bc.values), func(i int) bool { return bc.values[i].Name >= name })
	if i < len(bc.values) && bc.values[i].Name == name {
		// c is present, remove
		bc.values = append(bc.values[:i], bc.values[i+1:]...)
	}
}

type backendByName []*models.V2Backend

func (c backendByName) Len() int           { return len(c) }
func (c backendByName) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c backendByName) Less(i, j int) bool { return c[i].Name < c[j].Name }

// BackendCache manage the contents of the EdgeLB backend cache.
type BackendCache struct {
	backendCache
	Cond
}

// TODO update
// recomputeService recomputes V2Backend cache entries, adding, updating, or
// removing entries as required.
// If oldsvc is nil, entries in newsvc are unconditionally added to the cache.
// If oldsvc differs to newsvc, then the entries present only in oldsvc will
// be removed from the  cache, present in newsvc will be added.
// If newsvc is nil, entries in oldsvc  will be unconditionally deleted.
func (bc *BackendCache) recomputeService(oldsvc, newsvc *v1.Service) {
	if oldsvc == newsvc {
		// skip if oldsvc & newsvc == nil, or are the same object.
		return
	}

	defer bc.Notify()

	if oldsvc == nil {
		// if oldsvc is nil, replace it with a blank spec so entries
		// are unconditionally added.
		oldsvc = &v1.Service{
			ObjectMeta: newsvc.ObjectMeta,
		}
	}

	if newsvc == nil {
		// if newsvc is nil, replace it with a blank spec so entries
		// are unconditionally deleted.
		newsvc = &v1.Service{
			ObjectMeta: oldsvc.ObjectMeta,
		}
	}

	// TODO parse any Service annotations we're interested in

	// iterate over all ports in newsvc adding or updating their records.
	// also, record ports in named and unnamed, respectively for processing later.
	named := make(map[string]v1.ServicePort)
	unnamed := make(map[int32]v1.ServicePort)
	for _, p := range newsvc.Spec.Ports {
		switch p.Protocol {
		case "TCP":
			svcName := servicename(newsvc.ObjectMeta, p.Name)
			var backend *models.V2Backend
			if p.Name == "" {
				portString := strconv.Itoa(int(p.Port))
				backend = v2backend(newsvc, svcName, portString)
				unnamed[p.Port] = p
			} else {
				// service port is named, so we must generate both a cluster for the port name
				// and a cluster for the port number.
				backend = v2backend(newsvc, svcName, p.Name)
				named[p.Name] = p
			}
			bc.Add(backend)
		default:
			// ignore UDP and other port types.
		}
	}

	// iterate over all the ports in oldsvc, if they are not found in named or
	// unnamed then remove them from the cache.
	for _, p := range oldsvc.Spec.Ports {
		switch p.Protocol {
		case "TCP":
			if _, found := named[p.Name]; !found {
				bc.Remove(servicename(newsvc.ObjectMeta, p.Name))
			}
			if _, found := unnamed[p.Port]; !found {
				bc.Remove(servicename(newsvc.ObjectMeta, strconv.Itoa(int(p.Port))))
			}
		default:
			// ignore UDP and other port types.
		}
	}
}

func v2backend(svc *v1.Service, computedServiceName, portString string) *models.V2Backend {
	backend := &models.V2Backend{
		Name: svc.GetName(),
	}

	return backend
}
