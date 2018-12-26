package retry

import (
	"fmt"
	"time"
)

// ConditionFunc represents a function that returns whether a given condition has been met, or an error.
type ConditionFunc func() (bool, error)

// WithTimeout retries "fn" every "interval" until one of the following conditions are met:
// 1. "fn" evaluates to "true".
// 2. "fn" returns an error.
// 3. The specified timeout elapses.
func WithTimeout(timeout time.Duration, interval time.Duration, fn ConditionFunc) error {
	tickInt := time.NewTicker(interval)
	defer tickInt.Stop()
	tickOut := time.NewTicker(timeout)
	defer tickOut.Stop()

	for {
		select {
		case <-tickOut.C:
			return fmt.Errorf("timeout exceeded")
		case <-tickInt.C:
			r, err := fn()
			if err != nil {
				return err
			}
			if r {
				return nil
			}
		}
	}
}
