// This file has been modified by Tommaso Doninelli
// Major changes includes: package change and new methods to build the metric path
//
// Copyright 2018, OpenCensus Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package graphite

import (
	"fmt"
	"strconv"
	"time"
)

// Metric contains the metric fields expected by Graphite.
type Metric struct {
	Name      string
	Value     float64
	Timestamp time.Time
}

// String formats a Metric to the format expected bt Graphite.
func (m Metric) String() string {
	return fmt.Sprintf(
		"%s %s %d\n",
		m.Name,
		strconv.FormatFloat(m.Value, 'f', -1, 64),
		m.Timestamp.Unix(),
	)
}
