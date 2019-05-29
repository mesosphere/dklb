// +build e2e

package e2e_test

import (
	"flag"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/edgelb/manager"
	e2eframework "github.com/mesosphere/dklb/test/e2e/framework"
)

const (
	// awsPublicSubnetIDFlagName is the name of the flag that specifies the IDs of the subnets to use in cloud load-balancer configurations.
	awsPublicSubnetIDsFlagName = "aws-public-subnet-id"
	// cooldownDuration is the duration of the "cool-down" period between successive tests.
	cooldownDuration = 2 * time.Second
)

var (
	// awsPublicSubnetID is the ID of the subnet to use in cloud load-balancer configurations.
	awsPublicSubnetIDs []string
	// edgelbOptions is the set of options used to configure the EdgeLB Manager.
	edgelbOptions manager.EdgeLBManagerOptions
	// kubeconfig is the path to the kubeconfig file to use when running outside a Kubernetes cluster.
	kubeconfig string
	// logLevel is the log level to use while running the test suite.
	logLevel string
	// framework is the instance of the test framework to be used for testing.
	f *e2eframework.Framework
)

type stringSliceValue struct {
	slice *[]string
}

var _ flag.Value = &stringSliceValue{}

func (v stringSliceValue) String() string {
	return fmt.Sprintf("%v", *v.slice)
}

func (v stringSliceValue) Set(s string) error {
	*v.slice = append(*v.slice, s)
	return nil
}

func init() {
	flag.Var(&stringSliceValue{&awsPublicSubnetIDs}, awsPublicSubnetIDsFlagName, "the id(s) of the subnet to use in cloud load-balancer configurations")
	flag.StringVar(&edgelbOptions.BearerToken, "edgelb-bearer-token", "", "the (optional) bearer token to use when communicating with the edgelb api server")
	flag.StringVar(&edgelbOptions.Host, "edgelb-host", constants.DefaultEdgeLBHost, "the host at which the edgelb api server can be reached")
	flag.BoolVar(&edgelbOptions.InsecureSkipTLSVerify, "edgelb-insecure-skip-tls-verify", false, "whether to skip verification of the tls certificate presented by the edgelb api server")
	flag.StringVar(&edgelbOptions.Path, "edgelb-path", constants.DefaultEdgeLBPath, "the path at which the edgelb api server can be reached")
	flag.StringVar(&edgelbOptions.Scheme, "edgelb-scheme", constants.DefaultEdgeLBScheme, "the scheme to use when communicating with the edgelb api server")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "the path to the kubeconfig file to user")
	flag.StringVar(&logLevel, "log-level", log.InfoLevel.String(), "the log level to use while running the test suite")
	flag.Parse()
}

var _ = BeforeSuite(func() {
	// Create a new instance of the test framework.
	f = e2eframework.New(edgelbOptions, kubeconfig)
	// Output some information about the current MKE cluster.
	log.Infof("running the end-to-end test suite against the %q cluster", f.ClusterName)
})

var _ = BeforeEach(func() {
	// Introduce a slight "cool-down" period in order to prevent tests from failing due to the existence of EdgeLB pools that are still being deleted.
	time.Sleep(cooldownDuration)
	// Make sure the test prerequisites have been met.
	if err := f.CheckTestPrerequisites(); err != nil {
		log.Fatalf("failed to meet test prerequisites: %v", err)
	}
})

func TestEndToEnd(t *testing.T) {
	// Set the log level to the requested value.
	if l, err := log.ParseLevel(logLevel); err != nil {
		log.Fatal(err)
	} else {
		log.SetLevel(l)
	}
	// Register a failure handler and run the test suite.
	RegisterFailHandler(Fail)
	RunSpecs(t, "dklb end-to-end test suite")
}
