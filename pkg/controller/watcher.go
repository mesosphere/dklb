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
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/mesosphere/dklb/pkg/workgroup"
)

// WatchServices creates a SharedInformer for v1.Services and registers it with g.
func WatchServices(log logrus.FieldLogger, wg *workgroup.Group, kubeClient kubernetes.Interface, resyncPeriod time.Duration, rs ...cache.ResourceEventHandler) {
	watch(log, wg, kubeClient.CoreV1().RESTClient(), "services", new(v1.Service), resyncPeriod, rs...)
}

// WatchEndpoints creates a SharedInformer for v1.Endpoints and registers it with g.
func WatchEndpoints(log logrus.FieldLogger, wg *workgroup.Group, kubeClient kubernetes.Interface, resyncPeriod time.Duration, rs ...cache.ResourceEventHandler) {
	watch(log, wg, kubeClient.CoreV1().RESTClient(), "endpoints", new(v1.Endpoints), resyncPeriod, rs...)
}

// WatchIngress creates a SharedInformer for v1beta1.Ingress and registers it with g.
func WatchIngress(log logrus.FieldLogger, wg *workgroup.Group, kubeClient kubernetes.Interface, resyncPeriod time.Duration, rs ...cache.ResourceEventHandler) {
	watch(log, wg, kubeClient.ExtensionsV1beta1().RESTClient(), "ingresses", new(v1beta1.Ingress), resyncPeriod, rs...)
}

// WatchSecrets creates a SharedInformer for v1.Secrets and registers it with g.
func WatchSecrets(log logrus.FieldLogger, wg *workgroup.Group, kubeClient kubernetes.Interface, resyncPeriod time.Duration, rs ...cache.ResourceEventHandler) {
	watch(log, wg, kubeClient.CoreV1().RESTClient(), "secrets", new(v1.Secret), resyncPeriod, rs...)
}

func watch(log logrus.FieldLogger, wg *workgroup.Group, c cache.Getter, resource string, objType runtime.Object, resyncPeriod time.Duration, rs ...cache.ResourceEventHandler) {
	lw := cache.NewListWatchFromClient(c, resource, defaultNamespace, fields.Everything())
	sw := cache.NewSharedInformer(lw, objType, resyncPeriod)
	for _, r := range rs {
		sw.AddEventHandler(r)
	}
	wg.Add(func(stop <-chan struct{}) {
		log := log.WithField("resource", resource)
		log.Println("started")
		defer log.Println("stopped")
		sw.Run(stop)
	})
}
