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

package main

import (
	"flag"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/mesosphere/dklb/pkg/controller"
	"github.com/mesosphere/dklb/pkg/edgelb"
	"github.com/mesosphere/dklb/pkg/translator"
	"github.com/mesosphere/dklb/pkg/version"
	"github.com/mesosphere/dklb/pkg/workgroup"
)

const (
	// This controller default IngressClass.
	defaultIngressClass = "dlkb"

	// Default period between full Kubernetes API sync.
	defaultResyncPeriod = 24 * time.Hour
)

// Do this or logrus will complain
func init() {
	flag.Parse()
}

func main() {
	// Setup the main log, to be passed on to the several control-loops.
	log := logrus.StandardLogger()

	// Setup flags
	app := kingpin.New("dklb", "DC/OS Kubernetes load-balancer manager").Version(version.Version)
	run := app.Command("run", "run")
	ingressClass := run.Flag("ingress-class-name", "The IngressClass name to use").Default(defaultIngressClass).String()
	kubeconfig := run.Flag("kubeconfig", "Apath to a kubeconfig file").String()
	resyncPeriod := run.Flag("resync-period", "TODO").Default(defaultResyncPeriod.String()).Duration()
	kingpin.MustParse(app.Parse(os.Args[1:]))

	// Setup Kubernetes client
	kubeClient, err := newKubernetesClient(kubeconfig)
	if err != nil {
		log.Fatal("There was an error while setting up the Kubernetes client: ", err)
	}

	// TODO setup EdgeLB manager

	// t translates between Kubernetes API events to EdgeLB calls.
	t := translator.NewTranslator(log, *ingressClass /*TODO EdgeLB manager*/)

	var wg workgroup.Group

	edgelb.NewManager(log, &wg, t)

	if controller.NewLoadBalancerController(log, &wg, t, kubeClient, *resyncPeriod); err != nil {
		log.Fatal("There was an error while setting up the controller: ", err)
	}

	// Run all functions registered in the workgroup until one terminates
	// TODO make sure it respects signals
	wg.Run()
}

func newKubernetesClient(kubeconfig *string) (kubernetes.Interface, error) {
	if kubeconfig == nil {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		return kubernetes.NewForConfig(config)
	}

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}
