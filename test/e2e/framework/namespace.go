package framework

import (
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mesosphere/dklb/pkg/util/retry"
)

// WithTemporaryNamespace creates a Kubernetes namespace with a randomly generated name, calls the provided function with the namespace as its parameter, and deletes the namespace after said function returns.
func (f *Framework) WithTemporaryNamespace(fn func(namespace *corev1.Namespace)) {
	// Create a namespace with a randomly generated name.
	ns, err := f.KubeClient.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: KubernetesNamespacePrefix,
		},
	})
	// Make sure that no error has occurred.
	Expect(err).NotTo(HaveOccurred(), "failed to create temporary namespace")
	// Call the provided function with the namespace.
	fn(ns)
	// Delete the Kubernetes namespace and wait for it to disappear.
	err = f.KubeClient.CoreV1().Namespaces().Delete(ns.Name, metav1.NewDeleteOptions(0))
	Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %q", ns.Name)
	err = retry.WithTimeout(DefaultRetryTimeout, DefaultRetryInterval, func() (bool, error) {
		_, err := f.KubeClient.CoreV1().Namespaces().Get(ns.Name, metav1.GetOptions{})
		return kubeerrors.IsNotFound(err), nil
	})
	Expect(err).NotTo(HaveOccurred(), "timed out while waiting for namespace %q to be deleted", ns.Name)
}
