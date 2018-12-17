package main

import (
	"context"
	"flag"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"

	"github.com/mesosphere/dklb/pkg/constants"
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
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "the path to the kubeconfig file to use when running outside a Kubernetes cluster")
	flag.StringVar(&podNamespace, "pod-namespace", "", "the name of the namespace in which the current instance of the application is deployed (used to perform leader election)")
	flag.StringVar(&podName, "pod-name", "", "the identity of the current instance of the application (used to perform leader election)")
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
				run(runCtx)
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

// TODO (@bcustodio) Implement (start the controllers and wait for them to stop).
func run(ctx context.Context) {
	// Wait for the context to be cancelled.
	<-ctx.Done()
	// Confirm successful shutdown.
	log.WithField("version", version.Version).Infof("%s is shutting down", constants.ComponentName)
	// There is a goroutine in the background trying to renew the leader election lock.
	// Hence, we must manually exit now that we know controllers have been shutdown properly.
	os.Exit(0)
}
