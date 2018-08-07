package client

import (
	"fmt"
	"time"
)

type Metric struct {
	Name      string
	Value     string
	Timestamp time.Time
}

func (metric Metric) String() string {
	return fmt.Sprintf(
		"%s %s %d\n",
		metric.Name,
		metric.Value,
		metric.Timestamp.Unix(),
	)
}
