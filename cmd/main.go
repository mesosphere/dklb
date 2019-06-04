package main

import (
	"context"
	"crypto/tls"
	"flag"
	"math/rand"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"

	"github.com/mesosphere/dklb/pkg/admission"
	"github.com/mesosphere/dklb/pkg/backends"
	dklbcache "github.com/mesosphere/dklb/pkg/cache"
	"github.com/mesosphere/dklb/pkg/cluster"
	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/controllers"
	"github.com/mesosphere/dklb/pkg/edgelb/manager"
	"github.com/mesosphere/dklb/pkg/features"
	_ "github.com/mesosphere/dklb/pkg/metrics"
	"github.com/mesosphere/dklb/pkg/signals"
	translatorapi "github.com/mesosphere/dklb/pkg/translator/api"
	"github.com/mesosphere/dklb/pkg/version"
)

const (
	// admissionFailurePolicyFlag name is the name of the flag that specifies the failure policy to use when registering the admission webhook.
	admissionFailurePolicyFlagName = "admission-failure-policy"
	// admissionTLSCaBundleFlagName is the name of the flag that specifies the base64-encoded CA bundle to use for registering the admission webhook.
	admissionTLSCaBundleFlagName = "admission-tls-ca-bundle"
	// admissionTLSCertFileFlagName is the name of the flag that specifies the path to the file containing the certificate to use for serving the admission webhook.
	admissionTLSCertFileFlagName = "admission-tls-cert-file"
	// admissionTLSPrivateKeyFlagName is the name of the flag that specifies the path to the file containing the private key to use for serving the admission webhook.
	admissionTLSPrivateKeyFlagName = "admission-tls-private-key-file"
)

var (
	// admissionFailurePolicy is the failure policy to use when registering the admission webhook.
	admissionFailurePolicy string
	// admissionTLSCaBundle is the base64-encoded CA bundle to use for registering the admission webhook.
	admissionTLSCaBundle string
	// admissionTLSCertFile is the path to the file containing the certificate to use for serving the admission webhook.
	admissionTLSCertFile string
	// admissionTLSPrivateKeyFile is the path to the file containing the private key to use for serving the admission webhook.
	admissionTLSPrivateKeyFile string
	// edgelbOptions is the set of options used to configure the EdgeLB Manager.
	edgelbOptions manager.EdgeLBManagerOptions
	// featureGates is a comma-separated list of "key=value" pairs used to toggle certain features.
	featureGates string
	// featureMap is the mapping between features and their current status.
	featureMap features.FeatureMap
	// kubeconfig is the path to the kubeconfig file to use when running outside a Kubernetes cluster.
	kubeconfig string
	// logLevel is the log level to use.
	logLevel string
	// podNamespace is the name of the namespace in which the current instance of the application is deployed (used to perform leader election).
	podNamespace string
	// podName is the identity of the current instance of the application (used to perform leader election).
	podName string
	// resyncPeriod is the maximum amount of time that may elapse between two consecutive synchronizations of Ingress/Service resources and the status of EdgeLB pools.
	resyncPeriod time.Duration
	// srvWaitGroup is a WaitGroup used to wait for the default backend and admission webhook servers to shutdown.
	srvWaitGroup sync.WaitGroup
)

func init() {
	flag.StringVar(&admissionFailurePolicy, admissionFailurePolicyFlagName, "ignore", "the failure policy to use when registering the admission webhook")
	flag.StringVar(&admissionTLSCaBundle, admissionTLSCaBundleFlagName, "", "the base64-encoded ca bundle to use for registering the admission webhook")
	flag.StringVar(&admissionTLSCertFile, admissionTLSCertFileFlagName, "", "the path to the file containing the certificate to use for serving the admission webhook")
	flag.StringVar(&admissionTLSPrivateKeyFile, admissionTLSPrivateKeyFlagName, "", "the path to the file containing the private key to use for serving the admission webhook")
	flag.StringVar(&edgelbOptions.BearerToken, "edgelb-bearer-token", "", "the (optional) bearer token to use when communicating with the edgelb api server")
	flag.StringVar(&edgelbOptions.Host, "edgelb-host", constants.DefaultEdgeLBHost, "the host at which the edgelb api server can be reached")
	flag.BoolVar(&edgelbOptions.InsecureSkipTLSVerify, "edgelb-insecure-skip-tls-verify", false, "whether to skip verification of the tls certificate presented by the edgelb api server")
	flag.StringVar(&edgelbOptions.Path, "edgelb-path", constants.DefaultEdgeLBPath, "the path at which the edgelb api server can be reached")
	flag.StringVar(&edgelbOptions.PoolGroup, "edgelb-pool-group", constants.DefaultEdgeLBPoolGroup, "the dc/os service group in which to create edgelb pools")
	flag.StringVar(&edgelbOptions.Scheme, "edgelb-scheme", constants.DefaultEdgeLBScheme, "the scheme to use when communicating with the edgelb api server")
	flag.StringVar(&featureGates, "feature-gates", "", "a comma-separated list of \"key=value\" pairs used to toggle certain features")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "the path to the kubeconfig file to use when running outside a kubernetes cluster")
	flag.StringVar(&cluster.Name, "kubernetes-cluster-framework-name", "", "the name of the mesos framework that corresponds to the current kubernetes cluster")
	flag.StringVar(&logLevel, "log-level", log.InfoLevel.String(), "the log level to use")
	flag.StringVar(&podNamespace, "pod-namespace", "", "the name of the namespace in which the current instance of the application is deployed (used to perform leader election)")
	flag.StringVar(&podName, "pod-name", "", "the identity of the current instance of the application (used to perform leader election)")
	flag.DurationVar(&resyncPeriod, "resync-period", constants.DefaultResyncPeriod, "the maximum amount of time that may elapse between two consecutive synchronizations of ingress/service resources and the status of edgelb pools")
}

func main() {
	// Initialize our source of randomness, which we'll later use to generate random names for EdgeLB pools.
	rand.Seed(time.Now().UnixNano())

	// Parse the provided command-line flags.
	flag.Parse()

	// Enable logging at the requested level.
	l, err := log.ParseLevel(logLevel)
	if err != nil {
		log.Fatalf("%q is not a valid log level", logLevel)
	}
	log.SetLevel(l)

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
	if cluster.Name == "" {
		log.Fatalf("--kubernetes-cluster-framework-name must be set")
	}

	// Build the map of features based on the value of "--feature-gates".
	featureMap, err = features.ParseFeatureMap(featureGates)
	if err != nil {
		log.Errorf("failed to parse feature gates: %v", err)
	}
	// Fallback to the default features in case we couldn't parse the value of "--feature-gates".
	if featureMap == nil {
		featureMap = features.DefaultFeatureMap
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
	// Instruct the Translator API to use the current instance of the EdgeLB Manager whenever access to EdgeLB is required.
	translatorapi.SetEdgeLBManager(edgelbManager)

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

	// Launch the default backend.
	srvWaitGroup.Add(1)
	go func() {
		defer srvWaitGroup.Done()
		// Create and start the default backend.
		if err := backends.NewDefaultBackend().Run(stopCh); err != nil {
			log.Fatalf("failed to serve the default backend: %v", err)
		}
	}()

	// Launch the admission webhook if the "ServeAdmissionWebhook" feature is enabled.
	if featureMap.IsEnabled(features.ServeAdmissionWebhook) {
		if admissionTLSCertFile == "" {
			log.Fatalf("--%s must be set since the %q feature is enabled", admissionTLSCertFileFlagName, features.ServeAdmissionWebhook)
		}
		if admissionTLSPrivateKeyFile == "" {
			log.Fatalf("--%s must be set since the %q feature is enabled", admissionTLSPrivateKeyFlagName, features.ServeAdmissionWebhook)
		}
		srvWaitGroup.Add(1)
		go func() {
			defer srvWaitGroup.Done()
			// Try to load the provided TLS certificate and private key.
			p, err := tls.LoadX509KeyPair(admissionTLSCertFile, admissionTLSPrivateKeyFile)
			if err != nil {
				log.Fatalf("failed to read the tls certificate: %v", err)
			}
			// Create and start the admission webhook.
			if err := admission.NewWebhook(p).Run(stopCh); err != nil {
				log.Fatalf("failed to serve the admission webhook: %v", err)
			}
		}()
	}

	// Register the admission webhook if the "RegisterAdmissionWebhook" feature is enabled.
	if featureMap.IsEnabled(features.RegisterAdmissionWebhook) {
		if admissionTLSCaBundle == "" {
			log.Fatalf("--%s must be set since the %q feature is enabled", admissionTLSCaBundleFlagName, features.RegisterAdmissionWebhook)
		}
		if err := admission.RegisterWebhook(kubeClient, admissionTLSCaBundle, admissionFailurePolicy); err != nil {
			log.Fatalf("failed to register the admission webhook: %v", err)
		}
	}

	// Create an event recorder so we can emit events during leader election and afterwards.
	eb := record.NewBroadcaster()
	eb.StartLogging(log.Debugf)
	eb.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	er := eb.NewRecorder(scheme.Scheme, corev1.EventSource{Component: constants.ComponentName})

	// Setup a resource lock so we can perform leader election.
	rl, _ := resourcelock.New(
		resourcelock.EndpointsResourceLock,
		podNamespace,
		constants.ComponentName,
		kubeClient.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      podName,
			EventRecorder: er,
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
				run(runCtx, kubeClient, er, edgelbManager)
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
	ingressInformer := kubeInformerFactory.Extensions().V1beta1().Ingresses()
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	// we need to setup the secrets informer so that the kubeCache
	// gets populated accordingly
	secretsInformer := kubeInformerFactory.Core().V1().Secrets()
	secretsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{})
	secretsReflector := secretsreflector.New(cluster.Name, dcosClient.Secrets, saConfig.UID, kubeCache, kubeClient)

	// Create an instance of the ingress controller.
	ingressController := controllers.NewIngressController(kubeClient, er, kubeInformerFactory.Extensions().V1beta1().Ingresses(), kubeInformerFactory.Core().V1().Services(), kubeCache, edgelbManager)
	// Create an instance of the service controller.
	serviceController := controllers.NewServiceController(kubeClient, er, serviceInformer, kubeCache, edgelbManager)
	// Start the shared informer factory.
	go kubeInformerFactory.Start(ctx.Done())

	// Wait for the caches to be synced before starting workers.
	log.Debug("waiting for informer caches to be synced")
	if ok := cache.WaitForCacheSync(ctx.Done(), kubeCache.HasSynced, ingressInformer.Informer().HasSynced, serviceInformer.Informer().HasSynced, secretsInformer.Informer().HasSynced); !ok {
		log.Error("failed to wait for informer caches to be synced")
		return
	}
	log.Debug("informer caches are synced")
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
	// Wait for the default backend and admission webhook servers to stop.
	srvWaitGroup.Wait()
	// Confirm successful shutdown.
	log.WithField("version", version.Version).Infof("%s is shutting down", constants.ComponentName)
	// There is a goroutine in the background trying to renew the leader election lock.
	// Hence, we must manually exit now that we know controllers have been shutdown properly.
	os.Exit(0)
}
