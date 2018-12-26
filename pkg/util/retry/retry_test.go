package retry_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mesosphere/dklb/pkg/util/retry"
)

// TestRetryWithTimeout tests the "RetryWithTimeout" function.
func TestRetryWithTimeout(t *testing.T) {
	var (
		// cur holds the number of the current iteration.
		cur int
		// dur holds the duration of the invocation.
		dur time.Duration
		// err holds the returned error (if any).
		err error
		// now holds the time at which the test started.
		now time.Time
	)

	tests := []struct {
		// description is a description of the test.
		description string
		// timeout is the timeout to use as an argument to "RetryWithTimeout".
		timeout time.Duration
		// interval is the interval to use as an argument to "RetryWithTimeout".
		interval time.Duration
		// condition is the condition to be verified.
		condition retry.ConditionFunc
		// lowerBound is a lower bound on the duration of the invocation.
		lowerBound time.Duration
		// upperBound is an upper bound on the duration of the invocation.
		upperBound time.Duration
		// error is the expected error (if any).
		err error
	}{
		{
			description: "condition is verified within the timeout",
			timeout:     500 * time.Millisecond,
			interval:    100 * time.Millisecond,
			condition: func() (bool, error) {
				// Verify the condition after three iterations.
				cur++
				return cur == 3, nil
			},
			lowerBound: 300 * time.Millisecond,
			upperBound: 500 * time.Millisecond,
			err:        nil,
		},
		{
			description: "condition function returns an error within the timeout",
			timeout:     500 * time.Millisecond,
			interval:    100 * time.Millisecond,
			condition: func() (bool, error) {
				// Return an error after three iterations.
				cur++
				if cur < 3 {
					return false, nil
				}
				return false, fmt.Errorf("fake error")
			},
			lowerBound: 300 * time.Millisecond,
			upperBound: 500 * time.Millisecond,
			err:        fmt.Errorf("fake error"),
		},
		{
			description: "condition function is not verified within the timeout",
			timeout:     500 * time.Millisecond,
			interval:    100 * time.Millisecond,
			condition: func() (bool, error) {
				// Never verify the condition.
				cur++
				return cur < 0, nil
			},
			lowerBound: 0 * time.Millisecond,
			// Use a small margin in the upper bound.
			upperBound: 550 * time.Millisecond,
			err:        fmt.Errorf("timeout exceeded"),
		},
	}

	for _, test := range tests {
		t.Logf("test: %s", test.description)
		// Reset the iteration counter.
		cur = 0
		// Take note of the current time.
		now = time.Now()
		// Call the function with the specified arguments.
		err = retry.WithTimeout(test.timeout, test.interval, test.condition)
		// Take note of the duration of the invocation.
		dur = time.Since(now)
		// Make sure the error matches the expected one (if any).
		assert.Equal(t, test.err, err)
		// Make sure that the invocation took more time than the minimum (lower bound).
		assert.True(t, dur >= test.lowerBound)
		// Make sure that the invocation took less time than the maximum (upper bound).
		assert.True(t, dur <= test.upperBound)
	}
}
