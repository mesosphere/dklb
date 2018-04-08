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

package edgelb

import (
	"sync/atomic"

	"github.com/sirupsen/logrus"

	"github.com/mesosphere/dcos-edge-lb/models"

	"github.com/mesosphere/dklb/pkg/translator"
	"github.com/mesosphere/dklb/pkg/workgroup"
)

// BackendCache holds a set of computed models.V2Backend resources.
type BackendCache interface {
	// Values returns a copy of the contents of the cache.
	// The slice and its contents should be treated as read-only.
	Values() []*models.V2Backend

	// Register registers ch to receive a value when Notify is called.
	Register(chan int, int)

	// TODO Initialize cache from EdgeLB objects, if any.
}

// Manager implements an EdgeLB manager.
type Manager struct {
	wg *workgroup.Group

	backendManager
}

func NewManager(logger logrus.FieldLogger, wg *workgroup.Group, t *translator.Translator /*TODO ,edgeLBClient,*/) *Manager {
	mgr := &Manager{
		wg: wg,
		backendManager: backendManager{
			FieldLogger:  logger.WithField("dklb-edgelb-manager", "backendManager"),
			BackendCache: &t.BackendCache,
		},
	}

	wg.Add(mgr.loop)

	return mgr
}

func (mgr *Manager) loop(stop <-chan struct{}) {
	// TODO load from EdgeLB pools and populate caches
	mgr.Println("started")
	defer mgr.Println("stopped")

	log := mgr.backendManager.WithField("connection", atomic.AddUint64(&mgr.count, 1))
	sync(mgr.backendManager, log, stop)
}

type notifier interface {
	Register(chan int, int)
}

type syncer interface {
	Sync()
}

type backendManager struct {
	logrus.FieldLogger
	BackendCache
	count uint64
}

func sync( /*TODO edgeLBClient,*/ n notifier, log logrus.FieldLogger, stop <-chan struct{}) {
	ch := make(chan int, 1)
	last := 0
	for {
		log.WithField("version", last).Info("waiting for notification")
		n.Register(ch, last)
		select {
		case last = <-ch:
			log.WithField("version", last).Info("notification received")
		case <-stop:
			return
		}
	}
}
