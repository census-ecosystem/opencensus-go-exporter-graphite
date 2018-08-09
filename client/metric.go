package client

import (
	"fmt"
	"time"
)

// Metric contains the metric fields expected by Graphite.
type Metric struct {
	Name      string
	Value     string
	Timestamp time.Time
}

// String formats a Metric to the format expected bt Graphite.
func (metric Metric) String() string {
	return fmt.Sprintf(
		"%s %s %d\n",
		metric.Name,
		metric.Value,
		metric.Timestamp.Unix(),
	)
}
