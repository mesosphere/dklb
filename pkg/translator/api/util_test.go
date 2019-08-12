package api

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mesosphere/dklb/pkg/cluster"
)

func TestNewRandomEdgeLBPoolName(t *testing.T) {
	tests := []struct {
		description            string
		expectedPoolNameRegexp string
		clusterName            string
		prefix                 string
	}{
		{
			description:            "should create an edgelb pool name",
			expectedPoolNameRegexp: "^dev--test",
			clusterName:            "dev/test",
			prefix:                 "",
		},
		{
			description:            "should truncate cluster name",
			expectedPoolNameRegexp: "^one--sixty--three--character--string----",
			clusterName:            "one--sixty--three--character--string--used--for--testing--this",
			prefix:                 "",
		},
		{
			description:            "should truncate cluster name with a prefix",
			expectedPoolNameRegexp: "^cloud--one--sixty--three--character--s",
			clusterName:            "one--sixty--three--character--string--used--for--testing--this",
			prefix:                 "cloud",
		},
	}

	for _, test := range tests {
		t.Logf("test case: %s", test.description)

		cluster.Name = test.clusterName
		poolName := newRandomEdgeLBPoolName(test.prefix)

		assert.Regexp(t, regexp.MustCompile(test.expectedPoolNameRegexp), poolName)
	}
}
