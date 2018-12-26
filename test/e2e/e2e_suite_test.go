// +build e2e

package e2e_test

import (
	"flag"
	"log"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/edgelb/manager"
	e2eframework "github.com/mesosphere/dklb/test/e2e/framework"
)

var (
	// publicIP is the public (external) IP of the DC/OS agent where to deploy EdgeLB pools.
	// TODO (@bcustodio) Remove once EdgeLB is able to report the private/public IP(s) at which a pool can be reached.
	publicIP string
	// edgelbOptions is the set of options used to configure the EdgeLB Manager.
	edgelbOptions manager.EdgeLBManagerOptions
	// kubeconfig is the path to the kubeconfig file to use when running outside a Kubernetes cluster.
	kubeconfig string
)

var (
	// framework is the instance of the test framework to be used for testing.
	f *e2eframework.Framework
)

func init() {
	flag.StringVar(&publicIP, "dcos-public-agent-ip", "", "the public (external) ip of the dc/os agent where to deploy edgelb pools")
	flag.StringVar(&edgelbOptions.BearerToken, "edgelb-bearer-token", "", "the (optional) bearer token to use when communicating with the edgelb api server")
	flag.StringVar(&edgelbOptions.Host, "edgelb-host", constants.DefaultEdgeLBHost, "the host at which the edgelb api server can be reached")
	flag.BoolVar(&edgelbOptions.InsecureSkipTLSVerify, "edgelb-insecure-skip-tls-verify", false, "whether to skip verification of the tls certificate presented by the edgelb api server")
	flag.StringVar(&edgelbOptions.Path, "edgelb-path", constants.DefaultEdgeLBPath, "the path at which the edgelb api server can be reached")
	flag.StringVar(&edgelbOptions.Scheme, "edgelb-scheme", constants.DefaultEdgeLBScheme, "the scheme to use when communicating with the edgelb api server")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "the path to the kubeconfig file to user")
	flag.Parse()
}

var _ = BeforeSuite(func() {
	// Create a new instance of the test framework.
	f = e2eframework.New(edgelbOptions, kubeconfig)
})

var _ = BeforeEach(func() {
	// Make sure the test prerequisites have been met.
	if err := f.CheckTestPrerequisites(); err != nil {
		log.Fatalf("failed to meet test prerequisites: %v", err)
	}
})

func TestEndToEnd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dklb end-to-end test suite")
}
