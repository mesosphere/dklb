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
	// The resulting name is of the form "<prefix><cluster-name>--<namespace>--<name>".
	edgeLBPoolNameFormatString = "%s" + "%s" + edgeLBPoolNameComponentSeparator + "%s" + edgeLBPoolNameComponentSeparator + "%s"
)

// ComputeEdgeLBPoolName computes the name of the EdgeLB pool that corresponds to the Kubernetes resource corresponding to the provided coordinates.
// The computed name is of the form "<cluster-name>--<namespace>--<name>" for "regular" EdgeLB pools and "ext--<cluster-name>--<namespace>--<name>" for EdgeLB pools used with cloud load-balancers.
func ComputeEdgeLBPoolName(prefix, clusterName, namespace, name string) string {
	if prefix != "" {
		prefix = fmt.Sprintf("%s%s", prefix, edgeLBPoolNameComponentSeparator)
	}
	return fmt.Sprintf(edgeLBPoolNameFormatString, prefix, strings.ReplaceForwardSlashes(clusterName, edgeLBPoolNameComponentSeparator), namespace, name)
}
