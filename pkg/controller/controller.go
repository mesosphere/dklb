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

package controller

import (
	"time"

	"github.com/sirupsen/logrus"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	"github.com/mesosphere/dklb/pkg/workgroup"
)

const (
	// Size of Kubernetes API events buffer
	defaultBufferSize = 128

	// The default namespace to watch
	defaultNamespace = v1.NamespaceAll
)

// LoadBalancerController watches the Kubernetes API and adds/removes services
// from the loadbalancer, via loadBalancerConfig.
type LoadBalancerController struct {
	logrus.FieldLogger

	wg           *workgroup.Group
	kubeClient   kubernetes.Interface
	resyncPeriod time.Duration
	// Map of namespace => record.EventRecorder.
	recorders map[string]record.EventRecorder

	translator cache.ResourceEventHandler
}

// NewLoadBalancerController creates a controller for EdgeLB.
func NewLoadBalancerController(logger logrus.FieldLogger, wg *workgroup.Group, translator cache.ResourceEventHandler, kubeClient kubernetes.Interface, resyncPeriod time.Duration) (*LoadBalancerController, error) {
	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(logger.Printf)
	broadcaster.StartRecordingToSink(&corev1.EventSinkImpl{
		Interface: kubeClient.CoreV1().Events(defaultNamespace),
	})
	lbc := &LoadBalancerController{
		FieldLogger:  logger,
		wg:           wg,
		kubeClient:   kubeClient,
		resyncPeriod: resyncPeriod,
		recorders:    map[string]record.EventRecorder{},
		translator:   translator,
	}

	// Prepare Kubernetes API events buffer
	buffer := NewBuffer(wg, translator, logger.WithField("context", "dklb-controller-buffer"), defaultBufferSize)

	// Prepare watchers
	watcherLogger := lbc.WithField("context", "dklb-controller-watch")
	WatchServices(watcherLogger, lbc.wg, lbc.kubeClient, lbc.resyncPeriod, buffer)
	WatchEndpoints(watcherLogger, lbc.wg, lbc.kubeClient, lbc.resyncPeriod, buffer)
	WatchIngress(watcherLogger, lbc.wg, lbc.kubeClient, lbc.resyncPeriod, buffer)
	WatchSecrets(watcherLogger, lbc.wg, lbc.kubeClient, lbc.resyncPeriod, buffer)

	return lbc, nil
}

// Recorder returns the EventRecorder for the desired namespace.
func (lbc *LoadBalancerController) Recorder(ns string) record.EventRecorder {
	if rec, ok := lbc.recorders[ns]; ok {
		return rec
	}

	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(lbc.Printf)
	broadcaster.StartRecordingToSink(&corev1.EventSinkImpl{
		Interface: lbc.kubeClient.CoreV1().Events(ns),
	})
	rec := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "dklb-controller"})
	lbc.recorders[ns] = rec

	return rec
}
