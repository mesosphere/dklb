package kubernetes

import (
	"k8s.io/client-go/tools/cache"
)

// Key returns the key for a Kubernetes API resource.
// NOTE: obj should implement meta.Interface.
func Key(obj interface{}) string {
	res, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return "(unknown)"
	}
	return res
}
