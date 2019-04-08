package api

import (
	"fmt"
)

var (
	// EdgeLBPoolCreationStrategyIfNotPresent denotes the strategy that creates an EdgeLB pool whenever a pool with the same name doesn't already exist.
	EdgeLBPoolCreationStrategyIfNotPresent = EdgeLBPoolCreationStrategy("IfNotPresent")
	// EdgeLBPoolCreationStrategyNever denotes the strategy that never creates an EdgeLB pool, expecting it to have been created out-of-band.
	EdgeLBPoolCreationStrategyNever = EdgeLBPoolCreationStrategy("Never")
	// EdgeLBPoolCreationStrategyOnce denotes the strategy that creates an EdgeLB pool only if a pool for a given Ingress/Service resource has never been created.
	EdgeLBPoolCreationStrategyOnce = EdgeLBPoolCreationStrategy("Once")
)

// EdgeLBPoolCreationStrategy represents a strategy used to create EdgeLB pools.
type EdgeLBPoolCreationStrategy string

// UnmarshalYAML unmarshals the underlying value as an "EdgeLBPoolCreationStrategy" object.
func (s *EdgeLBPoolCreationStrategy) UnmarshalYAML(fn func(interface{}) error) error {
	var buf string
	if err := fn(&buf); err != nil {
		return err
	}
	v := EdgeLBPoolCreationStrategy(buf)
	switch v {
	case EdgeLBPoolCreationStrategyIfNotPresent, EdgeLBPoolCreationStrategyNever, EdgeLBPoolCreationStrategyOnce:
		*s = v
	default:
		return fmt.Errorf("failed to parse %q as an edgelb pool creation strategy", buf)
	}
	return nil
}
