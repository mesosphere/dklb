package translator

import (
	"fmt"

	"github.com/mesosphere/dklb/pkg/util/strings"
)

const (
	// edgeLBPoolNameComponentSeparator is the string used as to separate the components of an EdgeLB pool's name.
	// "--" is chosen as the value since the name of an EdgeLB pool must match the "^[a-z0-9]([a-z0-9-]*[a-z0-9])?$" regular expression.
	edgeLBPoolNameComponentSeparator = "--"
	// edgeLBPoolNameFormatString is the format string used to compute the name of an EdgeLB pool corresponding to a given Kubernetes resource.
	// The resulting name is of the form "<cluster-name>--<namespace>--<name>".
	edgeLBPoolNameFormatString = "%s" + edgeLBPoolNameComponentSeparator + "%s" + edgeLBPoolNameComponentSeparator + "%s"
)

// computeEdgeLBPoolName computes the name of the EdgeLB pool that corresponds to the Kubernetes resource corresponding to the provided coordinates.
func computeEdgeLBPoolName(clusterName, namespace, name string) string {
	return fmt.Sprintf(edgeLBPoolNameFormatString, strings.ReplaceForwardSlashes(clusterName, edgeLBPoolNameComponentSeparator), namespace, name)
}
