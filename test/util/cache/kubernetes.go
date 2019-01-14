package cache

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	kubecache "k8s.io/client-go/tools/cache"

	dklbcache "github.com/mesosphere/dklb/pkg/cache"
)

// NewFakeKubernetesResourceCache returns a Kubernetes resource cache that will list the specified resources.
func NewFakeKubernetesResourceCache(res ...runtime.Object) dklbcache.KubernetesResourceCache {
	return dklbcache.NewKubernetesResourceCache(NewFakeSharedInformerFactory(res...))
}

// NewFakeSharedInformerFactory returns a shared informer factory whose listers will list the specified resources.
func NewFakeSharedInformerFactory(res ...runtime.Object) kubeinformers.SharedInformerFactory {
	// Create a fake Kubernetes clientset that will list the specified resources.
	fakeClient := fake.NewSimpleClientset(res...)
	// Create a shared informer factory that uses the fake Kubernetes clientset.
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(fakeClient, 30*time.Second)
	// Start all the required informers.
	ingressInformer := kubeInformerFactory.Extensions().V1beta1().Ingresses()
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	go ingressInformer.Informer().Run(wait.NeverStop)
	go serviceInformer.Informer().Run(wait.NeverStop)
	// Wait for the caches to be synced.
	if !kubecache.WaitForCacheSync(wait.NeverStop, ingressInformer.Informer().HasSynced, serviceInformer.Informer().HasSynced) {
		panic("failed to wait for caches to be synced")
	}
	// Return the shared informer factory.
	return kubeInformerFactory
}
