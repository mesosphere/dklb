package translator

import (
	"fmt"
)

// poolInspectionReport is a utility struct used to convey information about the status of an EdgeLB pool (and the required changes) upon inspection.
type poolInspectionReport struct {
	lines []string
}

// Report adds a formatted message to the pool inspection report.
func (pir *poolInspectionReport) Report(message string, args ...interface{}) {
	pir.lines = append(pir.lines, fmt.Sprintf(message, args...))
}
