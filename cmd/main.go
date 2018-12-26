package main

import (
	"context"
	"flag"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"github.com/mesosphere/dklb/pkg/cluster"
	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/controllers"
	"github.com/mesosphere/dklb/pkg/edgelb/manager"
	"github.com/mesosphere/dklb/pkg/signals"
	kubernetesutil "github.com/mesosphere/dklb/pkg/util/kubernetes"
	"github.com/mesosphere/dklb/pkg/version"
)

var (
	// debug indicates whether to enable debug logging.
	debug bool
	// edgelbOptions is the set of options used to configure the EdgeLB Manager.
	edgelbOptions manager.EdgeLBManagerOptions
	// kubeconfig is the path to the kubeconfig file to use when running outside a Kubernetes cluster.
	kubeconfig string
	// podNamespace is the name of the namespace in which the current instance of the application is deployed (used to perform leader election).
	podNamespace string
	// podName is the identity of the current instance of the application (used to perform leader election).
	podName string
	// resyncPeriod is the maximum amount of time that may elapse between two consecutive synchronizations of Ingress/Service resources and the status of EdgeLB pools.
	resyncPeriod time.Duration
)

func init() {
	flag.BoolVar(&debug, "debug", false, "whether to enable debug logging")
	flag.StringVar(&edgelbOptions.BearerToken, "edgelb-bearer-token", "", "the (optional) bearer token to use when communicating with the edgelb api server")
	flag.StringVar(&edgelbOptions.Host, "edgelb-host", constants.DefaultEdgeLBHost, "the host at which the edgelb api server can be reached")
	flag.BoolVar(&edgelbOptions.InsecureSkipTLSVerify, "edgelb-insecure-skip-tls-verify", false, "whether to skip verification of the tls certificate presented by the edgelb api server")
	flag.StringVar(&edgelbOptions.Path, "edgelb-path", constants.DefaultEdgeLBPath, "the path at which the edgelb api server can be reached")
	flag.StringVar(&edgelbOptions.Scheme, "edgelb-scheme", constants.DefaultEdgeLBScheme, "the scheme to use when communicating with the edgelb api server")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "the path to the kubeconfig file to use when running outside a kubernetes cluster")
	flag.StringVar(&cluster.KubernetesClusterFrameworkName, "kubernetes-cluster-framework-name", "", "the name of the mesos framework that corresponds to the current kubernetes cluster")
	flag.StringVar(&podNamespace, "pod-namespace", "", "the name of the namespace in which the current instance of the application is deployed (used to perform leader election)")
	flag.StringVar(&podName, "pod-name", "", "the identity of the current instance of the application (used to perform leader election)")
	flag.DurationVar(&resyncPeriod, "resync-period", constants.DefaultResyncPeriod, "the maximum amount of time that may elapse between two consecutive synchronizations of ingress/service resources and the status of edgelb pools")
}

func main() {
	// Parse the provided command-line flags.
	flag.Parse()

	// Enable debug logging if requested.
	if debug {
		log.SetLevel(log.DebugLevel)
	}

	// Make sure that all necessary flags have been set and have adequate values.
	if podNamespace == "" {
		log.Fatalf("--pod-namespace must be set")
	}
	if podNamespace != constants.KubeSystemNamespaceName {
		log.Fatalf("%s must run on the %q namespace", constants.ComponentName, constants.KubeSystemNamespaceName)
	}
	if podName == "" {
		log.Fatalf("--pod-name must be set")
	}
	if cluster.KubernetesClusterFrameworkName == "" {
		log.Fatalf("--kubernetes-cluster-framework-name must be set")
	}

	// Setup a signal handler so we can gracefully shutdown when requested to.
	stopCh := signals.SetupSignalHandler()
	// Birth cry.
	log.WithField("version", version.Version).Infof("%s is starting", constants.ComponentName)

	// Create a new instance of the EdgeLB Manager.
	edgelbManager, err := manager.NewEdgeLBManager(edgelbOptions)
	if err != nil {
		log.Fatalf("failed to build edgelb manager: %v", err)
	}

	// Check the version of the EdgeLB API server that is currently installed, and issue a warning in case it could not be detected within a couple seconds.
	ctx, fn := context.WithTimeout(context.Background(), 2*time.Second)
	defer fn()
	if v, err := edgelbManager.GetVersion(ctx); err == nil {
		log.Infof("detected edgelb version: %s", v)
	} else {
		log.Warnf("failed to detect the version of edgelb currently installed: %v", err)
	}

	// Create a Kubernetes configuration object.
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("failed to build kubeconfig: %v", err)
	}
	// Create a client for the core Kubernetes APIs.
	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		log.Fatalf("failed to build kubernetes client: %v", err)
	}

	// Setup a resource lock so we can perform leader election.
	rl, _ := resourcelock.New(
		resourcelock.EndpointsResourceLock,
		podNamespace,
		constants.ComponentName,
		kubeClient.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      podName,
			EventRecorder: kubernetesutil.NewEventRecorderForNamespace(kubeClient, podNamespace),
		},
	)

	// Perform leader election so that at most a single instance of the application is active at any given moment.
	leaderelection.RunOrDie(context.Background(), leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				// We've started leading, so we can start our controllers.
				// The controllers will run under the specified context, and will stop whenever said context is canceled.
				// However, we must also make sure that they stop whenever we receive a shutdown signal.
				// Hence, we must create a new child context and wait in a separate goroutine for "stopCh" to be notified of said shutdown signal.
				runCtx, runCancel := context.WithCancel(ctx)
				defer runCancel()
				go func() {
					<-stopCh
					runCancel()
				}()
				run(runCtx, kubeClient, edgelbManager)
			},
			OnStoppedLeading: func() {
				// We've stopped leading, so we should exit immediately.
				log.Fatalf("leader election lost")
			},
			OnNewLeader: func(identity string) {
				// Report who the current leader is for debugging purposes.
				log.Infof("current leader: %s", identity)
			},
		},
	})
}

// run starts the controllers and blocks until they stop.
func run(ctx context.Context, kubeClient kubernetes.Interface, edgelbManager manager.EdgeLBManager) {
	// Create a shared informer factory for the base API types.
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, resyncPeriod)
	// Create an instance of the ingress controller that uses an ingress informer for watching Ingress resources.
	ingressController := controllers.NewIngressController(kubeClient, kubeInformerFactory.Extensions().V1beta1().Ingresses(), edgelbManager)
	// Create an instance of the service controller that uses a service informer for watching Service resources.
	serviceController := controllers.NewServiceController(kubeClient, kubeInformerFactory.Core().V1().Services(), edgelbManager)
	// Start the shared informer factory.
	go kubeInformerFactory.Start(ctx.Done())

	// Start the ingress and service controllers.
	var wg sync.WaitGroup
	for _, c := range []controllers.Controller{ingressController, serviceController} {
		wg.Add(1)
		go func(c controllers.Controller) {
			defer wg.Done()
			if err := c.Run(ctx); err != nil {
				log.Error(err)
			}
		}(c)
	}

	// Wait for the controllers to stop.
	wg.Wait()
	// Confirm successful shutdown.
	log.WithField("version", version.Version).Infof("%s is shutting down", constants.ComponentName)
	// There is a goroutine in the background trying to renew the leader election lock.
	// Hence, we must manually exit now that we know controllers have been shutdown properly.
	os.Exit(0)
}
