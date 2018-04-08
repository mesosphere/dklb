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
	"strings"

	"github.com/sirupsen/logrus"

	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/mesosphere/dklb/pkg/annotations"
)

type metadata struct {
	name, namespace string
}

// Translator receives notifications from the Kubernetes API and translates those
// objects into additions and removals entries of EdgeLB objects from a cache.
type Translator struct {
	logrus.FieldLogger

	// The IngressClass the translator is interested in.
	ingressClass string

	// Cache for Kubernetes objects.
	cache translatorCache

	// EdgeLB caches operations
	BackendCache
}

// NewTranslator returns a configured Translator.
func NewTranslator(logger logrus.FieldLogger, ingressClass string) *Translator {
	return &Translator{
		FieldLogger:  logger.WithField("context", "translator"),
		ingressClass: ingressClass,
	}
}

// OnAdd handles added Kubernetes resources.
func (t *Translator) OnAdd(obj interface{}) {
	t.cache.OnAdd(obj)
	switch obj := obj.(type) {
	case *v1.Service:
		t.addService(obj)
	case *v1.Endpoints:
		t.addEndpoints(obj)
	case *v1beta1.Ingress:
		t.addIngress(obj)
	case *v1.Secret:
		t.addSecret(obj)
	default:
		t.Errorf("OnAdd unexpected type %T: %#v", obj, obj)
	}
}

// OnUpdate handles updated Kubernetes resources.
func (t *Translator) OnUpdate(oldObj, newObj interface{}) {
	t.cache.OnUpdate(oldObj, newObj)
	switch newObj := newObj.(type) {
	case *v1.Service:
		oldObj, ok := oldObj.(*v1.Service)
		if !ok {
			t.Errorf("OnUpdate service %#v received invalid oldObj %T: %#v", newObj, oldObj, oldObj)
			return
		}
		t.updateService(oldObj, newObj)
	case *v1.Endpoints:
		oldObj, ok := oldObj.(*v1.Endpoints)
		if !ok {
			t.Errorf("OnUpdate endpoints %#v received invalid oldObj %T; %#v", newObj, oldObj, oldObj)
			return
		}
		t.updateEndpoints(oldObj, newObj)
	case *v1beta1.Ingress:
		oldObj, ok := oldObj.(*v1beta1.Ingress)
		if !ok {
			t.Errorf("OnUpdate endpoints %#v received invalid oldObj %T; %#v", newObj, oldObj, oldObj)
			return
		}
		t.updateIngress(oldObj, newObj)
	case *v1.Secret:
		t.addSecret(newObj)
	default:
		t.Errorf("OnUpdate unexpected type %T: %#v", newObj, newObj)
	}
}

// OnDelete handles deleted Kubernetes resources.
func (t *Translator) OnDelete(obj interface{}) {
	t.cache.OnDelete(obj)
	switch obj := obj.(type) {
	case *v1.Service:
		t.removeService(obj)
	case *v1.Endpoints:
		t.removeEndpoints(obj)
	case *v1beta1.Ingress:
		t.removeIngress(obj)
	case *v1.Secret:
		t.removeSecret(obj)
	case cache.DeletedFinalStateUnknown:
		t.OnDelete(obj.Obj) // recurse into ourselves with the tombstoned value
	default:
		t.Errorf("OnDelete unexpected type %T: %#v", obj, obj)
	}
}

func (t *Translator) addService(svc *v1.Service) {
	t.recomputeService(nil, svc)
}

func (t *Translator) updateService(oldsvc, newsvc *v1.Service) {
	t.recomputeService(oldsvc, newsvc)
}

func (t *Translator) removeService(svc *v1.Service) {
	t.recomputeService(svc, nil)
}

func (t *Translator) addEndpoints(e *v1.Endpoints) {
	//t.recomputeClusterLoadAssignment(nil, e)
}

func (t *Translator) updateEndpoints(oldep, newep *v1.Endpoints) {
	if len(newep.Subsets) < 1 {
		// if there are no endpoints in this object, ignore it
		// to avoid sending a noop notification to watchers.
		return
	}
	//t.recomputeClusterLoadAssignment(oldep, newep)
}

func (t *Translator) removeEndpoints(e *v1.Endpoints) {
	//t.recomputeClusterLoadAssignment(e, nil)
}

func (t *Translator) addIngress(i *v1beta1.Ingress) {
	class, ok := i.Annotations[annotations.KubernetesIngressClass]
	if ok && class != t.ingressClass {
		// if there is an ingress class set, but it is not set to configured
		// or default ingress class, ignore this ingress.
		// TODO skip creating any related backends?
		return
	}

	//t.recomputeListeners(t.cache.ingresses, t.cache.secrets)

	// handle the special case of the default ingress first.
	if i.Spec.Backend != nil {
		// update t.vhosts cache
		//t.recomputevhost("*", t.cache.vhosts["*"])
	}

	for _, rule := range i.Spec.Rules {
		host := rule.Host
		if host == "" {
			// If the host is unspecified, the Ingress routes all traffic based on the specified IngressRuleValue.
			host = "*"
		}
		//t.recomputevhost(host, t.cache.vhosts[host])
	}
}

func (t *Translator) updateIngress(oldIng, newIng *v1beta1.Ingress) {
	t.removeIngress(oldIng)
	t.addIngress(newIng)
}

func (t *Translator) removeIngress(i *v1beta1.Ingress) {
	class, ok := i.Annotations[annotations.KubernetesIngressClass]
	if ok && class != t.ingressClass {
		// if there is an ingress class set, but it is not set to configured
		// or default ingress class, ignore this ingress.
		// TODO skip creating any related backends?
		return
	}

	//t.recomputeListeners(t.cache.ingresses, t.cache.secrets)

	if i.Spec.Backend != nil {
		//t.recomputevhost("*", nil)
	}

	for _, rule := range i.Spec.Rules {
		host := rule.Host
		if host == "" {
			// If the host is unspecified, the Ingress routes all traffic based on the specified IngressRuleValue.
			host = "*"
		}
		//t.recomputevhost(rule.Host, t.cache.vhosts[host])
	}
}

func (t *Translator) addSecret(s *v1.Secret) {
	//t.recomputeTLSListener(t.cache.ingresses, t.cache.secrets)
}

func (t *Translator) removeSecret(s *v1.Secret) {
	//t.recomputeTLSListener(t.cache.ingresses, t.cache.secrets)
}

type translatorCache struct {
	ingresses map[metadata]*v1beta1.Ingress
	endpoints map[metadata]*v1.Endpoints
	services  map[metadata]*v1.Service

	// secrets stores tls secrets
	secrets map[metadata]*v1.Secret

	// vhosts stores a slice of vhosts with the ingress objects that
	// went into creating them.
	vhosts map[string]map[metadata]*v1beta1.Ingress
}

func (t *translatorCache) OnAdd(obj interface{}) {
	switch obj := obj.(type) {
	case *v1.Service:
		if t.services == nil {
			t.services = make(map[metadata]*v1.Service)
		}
		t.services[metadata{name: obj.Name, namespace: obj.Namespace}] = obj
	case *v1.Endpoints:
		if t.endpoints == nil {
			t.endpoints = make(map[metadata]*v1.Endpoints)
		}
		t.endpoints[metadata{name: obj.Name, namespace: obj.Namespace}] = obj
	case *v1beta1.Ingress:
		if t.ingresses == nil {
			t.ingresses = make(map[metadata]*v1beta1.Ingress)
		}
		md := metadata{name: obj.Name, namespace: obj.Namespace}
		t.ingresses[md] = obj
		if t.vhosts == nil {
			t.vhosts = make(map[string]map[metadata]*v1beta1.Ingress)
		}
		if obj.Spec.Backend != nil {
			if _, ok := t.vhosts["*"]; !ok {
				t.vhosts["*"] = make(map[metadata]*v1beta1.Ingress)
			}
			t.vhosts["*"][md] = obj
		}
		for _, rule := range obj.Spec.Rules {
			host := rule.Host
			if host == "" {
				host = "*"
			}
			if _, ok := t.vhosts[host]; !ok {
				t.vhosts[host] = make(map[metadata]*v1beta1.Ingress)
			}
			t.vhosts[host][md] = obj
		}
	case *v1.Secret:
		if t.secrets == nil {
			t.secrets = make(map[metadata]*v1.Secret)
		}
		t.secrets[metadata{name: obj.Name, namespace: obj.Namespace}] = obj
	default:
		// ignore
	}
}

func (t *translatorCache) OnUpdate(oldObj, newObj interface{}) {
	switch oldObj := oldObj.(type) {
	case *v1beta1.Ingress:
		// ingress objects are special because their contents can change
		// which affects the t.vhost cache. The simplest way is to model
		// update as delete, then add.
		t.OnDelete(oldObj)
	}
	t.OnAdd(newObj)
}

func (t *translatorCache) OnDelete(obj interface{}) {
	switch obj := obj.(type) {
	case *v1.Service:
		delete(t.services, metadata{name: obj.Name, namespace: obj.Namespace})
	case *v1.Endpoints:
		delete(t.endpoints, metadata{name: obj.Name, namespace: obj.Namespace})
	case *v1beta1.Ingress:
		md := metadata{name: obj.Name, namespace: obj.Namespace}
		delete(t.ingresses, md)
		delete(t.vhosts["*"], md)
		for _, rule := range obj.Spec.Rules {
			host := rule.Host
			if host == "" {
				host = "*"
			}
			delete(t.vhosts[host], md)
			if len(t.vhosts[host]) == 0 {
				delete(t.vhosts, host)
			}
		}
		if len(t.vhosts["*"]) == 0 {
			delete(t.vhosts, "*")
		}
	case *v1.Secret:
		delete(t.secrets, metadata{name: obj.Name, namespace: obj.Namespace})
	case cache.DeletedFinalStateUnknown:
		t.OnDelete(obj.Obj) // recurse into ourselves with the tombstoned value
	default:
		// ignore
	}
}

// servicename returns a fixed name for this service and portname
func servicename(meta metav1.ObjectMeta, portname string) string {
	sn := []string{
		meta.Namespace,
		meta.Name,
		"",
	}[:2]
	if portname != "" {
		sn = append(sn, portname)
	}
	return strings.Join(sn, "/")
}
