package api

import (
	edgelbmanager "github.com/mesosphere/dklb/pkg/edgelb/manager"
)

var (
	// manager is the instance of the EdgeLB manager to use whenever access to EdgeLB is required.
	manager edgelbmanager.EdgeLBManager
)

// SetEdgeLBManager instructs the Translator API to use the specified instance of the EdgeLB manager whenever access to EdgeLB is required.
func SetEdgeLBManager(m edgelbmanager.EdgeLBManager) {
	manager = m
}
