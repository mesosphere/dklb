package framework

import (
	"context"

	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	edgelbmanager "github.com/mesosphere/dklb/pkg/edgelb/manager"
	kubernetesutil "github.com/mesosphere/dklb/pkg/util/kubernetes"
)

const (
	// mkeClusterInfoConfigMapNamespace is the namespace where the configmap holding metadata for an MKE cluster lives.
	mkeClusterInfoConfigMapNamespace = "kube-system"
	// mkeClusterInfoConfigMapName is the name of the configmap holding metadata for an MKE cluster.
	mkeClusterInfoConfigMapName = "mke-cluster-info"
	// mkeClusterInfoConfigMapClusterNameKey is the name of the configmap key containing the MKE cluster's name.
	mkeClusterInfoConfigMapClusterNameKey = "CLUSTER_NAME"
)

// Framework groups together utility methods and clients used by test functions.
type Framework struct {
	// ClusterName is the name of the Mesos framework that corresponds to the Kubernetes cluster where testing will be performed.
	ClusterName string
	// EdgeLBManager is the instance of the EdgeLB manager to use.
	EdgeLBManager edgelbmanager.EdgeLBManager
	// KubeClient is a client to the Kubernetes base APIs.
	KubeClient kubernetes.Interface
}

// New returns a new instance of the testing framework.
func New(edgelbOptions edgelbmanager.EdgeLBManagerOptions, kubeconfig string) *Framework {
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
	// Return a new instance of the testing framework.
	manager, err := edgelbmanager.NewEdgeLBManager(edgelbOptions)
	if err != nil {
		log.Fatalf("failed to build edgelb manager: %v", err)
	}
	// Detect the name of the MKE cluster.
	var (
		clusterName string
	)
	m, err := kubeClient.CoreV1().ConfigMaps(mkeClusterInfoConfigMapNamespace).Get(mkeClusterInfoConfigMapName, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("failed to read the \"%s/%s\" configmap: %v", mkeClusterInfoConfigMapNamespace, mkeClusterInfoConfigMapName, err)
	}
	if v, exists := m.Data[mkeClusterInfoConfigMapClusterNameKey]; !exists || v == "" {
		log.Fatalf("the mke cluster's name is not present in the %q configmap", kubernetesutil.Key(m))
	} else {
		clusterName = v
	}
	return &Framework{
		ClusterName:   clusterName,
		EdgeLBManager: manager,
		KubeClient:    kubeClient,
	}
}

// CheckTestPrerequisites checks if the prerequisites for running a single test are met.
// These prerequisites include no pre-existing Kubernetes namespaces starting with "KubernetesNamespacePrefix", and no pre-existing EdgeLB pools.
func (f *Framework) CheckTestPrerequisites() error {
	// Check that there are no namespaces whose name starts with "KubernetesNamespacePrefix".
	namespaces, err := f.KubeClient.CoreV1().Namespaces().List(metav1.ListOptions{
		IncludeUninitialized: true,
	})
	Expect(err).NotTo(HaveOccurred(), "failed to list namespaces")
	for _, ns := range namespaces.Items {
		Expect(ns.Name).NotTo(HavePrefix(KubernetesNamespacePrefix), "expected no pre-existing namespaces with prefix %q", KubernetesNamespacePrefix)
	}
	// Check that there are no pre-existing EdgeLB pools.
	ctx, fn := context.WithTimeout(context.Background(), DefaultEdgeLBOperationTimeout)
	defer fn()
	pools, err := f.EdgeLBManager.GetPools(ctx)
	Expect(err).NotTo(HaveOccurred(), "failed to list edgelb pools")
	Expect(len(pools)).To(Equal(0), "expected no pre-existing edgelb pools")
	// Signal that we're good to go.
	return nil
}
