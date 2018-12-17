package main

import (
	"context"
	"flag"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/controllers"
	"github.com/mesosphere/dklb/pkg/signals"
	"github.com/mesosphere/dklb/pkg/version"
)

var (
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
	flag.StringVar(&kubeconfig, "kubeconfig", "", "the path to the kubeconfig file to use when running outside a Kubernetes cluster")
	flag.StringVar(&podNamespace, "pod-namespace", "", "the name of the namespace in which the current instance of the application is deployed (used to perform leader election)")
	flag.StringVar(&podName, "pod-name", "", "the identity of the current instance of the application (used to perform leader election)")
	flag.DurationVar(&resyncPeriod, "resync-period", constants.DefaultResyncPeriod, "the maximum amount of time that may elapse between two consecutive synchronizations of ingress/service resources and the status of EdgeLB pools")
}

func main() {
	// Parse the provided command-line flags.
	flag.Parse()

	// Make sure that all necessary flags have been set.
	if podNamespace == "" {
		log.Fatalf("--pod-namespace must be set")
	}
	if podName == "" {
		log.Fatalf("--pod-name must be set")
	}

	// Setup a signal handler so we can gracefully shutdown when requested to.
	stopCh := signals.SetupSignalHandler()
	// Birth cry.
	log.WithField("version", version.Version).Infof("%s is starting", constants.ComponentName)

	// Create a Kubernetes configuration object.
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("error building kubeconfig: %v", err)
	}
	// Create a client for the core Kubernetes APIs.
	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		log.Fatalf("error building kubernetes clientset: %v", err)
	}

	// Setup a resource lock so we can perform leader election.
	rl, err := resourcelock.New(
		resourcelock.EndpointsResourceLock,
		podNamespace,
		constants.ComponentName,
		kubeClient.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      podName,
			EventRecorder: createRecorder(kubeClient, podNamespace),
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
				run(runCtx, kubeClient)
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

// createRecorder creates a recorder for Kubernetes events.
func createRecorder(kubeClient kubernetes.Interface, podNamespace string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Infof)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: corev1client.New(kubeClient.CoreV1().RESTClient()).Events(podNamespace)})
	return eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: constants.ComponentName})
}

// run starts the controllers and blocks until they stop.
func run(ctx context.Context, kubeClient kubernetes.Interface) {
	// Create a shared informer factory for the base API types.
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, resyncPeriod)
	// Create an instance of the ingress controller that uses an ingress informer for watching Ingress resources.
	ingressController := controllers.NewIngressController(kubeInformerFactory.Extensions().V1beta1().Ingresses())
	// Create an instance of the service controller that uses a service informer for watching Service resources.
	serviceController := controllers.NewServiceController(kubeInformerFactory.Core().V1().Services())
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
