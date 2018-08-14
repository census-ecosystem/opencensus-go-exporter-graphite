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

// Package graphite contains a Graphite exporter that supports exporting
// OpenCensus views as Graphite metrics.
package graphite // import "contrib.go.opencensus.io/exporter/graphite"

import (
	"bytes"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"contrib.go.opencensus.io/exporter/graphite/internal/client"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

// Exporter exports stats to Graphite
type Exporter struct {
	// Options used to register and log stats
	opts Options
}

// Options contains options for configuring the exporter.
type Options struct {
	// Host contains de host address for the graphite server.
	// The default value is "127.0.0.1".
	Host string

	// Port is the port in which the carbon endpoint is available.
	// The default value is 2003.
	Port int

	// Namespace is optional and will be the first element in the path.
	Namespace string

	// OnError is the hook to be called when there is
	// an error uploading the stats or tracing data.
	// If no custom hook is set, errors are logged.
	// Optional.
	OnError func(err error)
}

// NewExporter returns an exporter that exports stats to Graphite.
func NewExporter(o Options) (*Exporter, error) {
	if o.Host == "" {
		// default Host
		o.Host = "127.0.0.1"
	}

	if o.Port == 0 {
		// default Port
		o.Port = 2003
	}

	e := &Exporter{
		opts: o,
	}

	return e, nil
}

var _ view.Exporter = (*Exporter)(nil)

func (o *Options) onError(err error) {
	if o.OnError != nil {
		o.OnError(err)
	} else {
		log.Printf("Failed to export to Graphite: %v", err)
	}
}

// ExportView exports to the Graphite if view data has one or more rows.
// Each OpenCensus stats records will be converted to
// corresponding Graphite Metric
func (e *Exporter) ExportView(vd *view.Data) {
	if len(vd.Rows) == 0 {
		return
	}
	extractData(e, vd)
}

// toMetric receives the view data information and creates metrics that are adequate according to
// graphite documentation.
func (e *Exporter) toMetric(v *view.View, row *view.Row, vd *view.Data) {
	switch data := row.Data.(type) {
	case *view.CountData:
		go sendRequest(e, formatTimeSeriesMetric(data.Value, row, vd, e))
	case *view.SumData:
		go sendRequest(e, formatTimeSeriesMetric(data.Value, row, vd, e))
	case *view.LastValueData:
		go sendRequest(e, formatTimeSeriesMetric(data.Value, row, vd, e))
	case *view.DistributionData:
		// Graphite does not support histogram. In order to emulate one,
		// we use the accumulative count of the bucket.
		var path bytes.Buffer
		indicesMap := make(map[float64]int)
		buckets := make([]float64, 0, len(v.Aggregation.Buckets))
		for i, b := range v.Aggregation.Buckets {
			if _, ok := indicesMap[b]; !ok {
				indicesMap[b] = i
				buckets = append(buckets, b)
			}
		}
		sort.Float64s(buckets)

		// Now that the buckets are sorted by magnitude
		// we can create cumulative indicesmap them back by reverse index
		cumCount := uint64(0)
		for _, b := range buckets {
			i := indicesMap[b]
			cumCount += uint64(data.CountPerBucket[i])
			path.Reset()
			names := []string{sanitize(e.opts.Namespace), sanitize(vd.View.Name), "bucket"}
			tags := tagValues(row.Tags) + fmt.Sprintf(";le=%.2f", b)
			path.WriteString(buildPath(names, tags))
			metric, _ := newConstMetric(path.String(), float64(cumCount))
			go sendRequest(e, metric)
		}
		path.Reset()
		names := []string{sanitize(e.opts.Namespace), sanitize(vd.View.Name), "bucket"}
		tags := tagValues(row.Tags) + ";le=+Inf"
		path.WriteString(buildPath(names, tags))
		metric, _ := newConstMetric(path.String(), float64(cumCount))
		sendRequest(e, metric)
	default:
		e.opts.onError(fmt.Errorf("aggregation %T is not yet supported", data))
	}
}

// formatTimeSeriesMetric creates a CountData metric, SumData or LastValueData
// and returns it to toMetric
func formatTimeSeriesMetric(value interface{}, row *view.Row, vd *view.Data, e *Exporter) constMetric {
	var path bytes.Buffer
	names := []string{sanitize(e.opts.Namespace), sanitize(vd.View.Name)}
	path.WriteString(buildPath(names, tagValues(row.Tags)))
	var metric constMetric
	switch x := value.(type) {
	case int64:
		metric, _ = newConstMetric(path.String(), float64(x))
	case float64:
		metric, _ = newConstMetric(path.String(), x)
	}
	return metric
}

// extractData extracts stats data and calls toMetric
// to convert the data to metrics formatted to graphite
func extractData(e *Exporter, vd *view.Data) {
	for _, row := range vd.Rows {
		e.toMetric(vd.View, row, vd)
	}
}

// buildPath creates the path for the metric that
// is expected by graphite. It takes a list of strings
// and joins with a dot (".")
func buildPath(names []string, tags string) string {
	var values []string
	for _, name := range names {
		if name != "" {
			values = append(values, name)
		}
	}
	path := strings.Join(values, ".")
	return path + tags
}

// tagValues builds the list of tags that is expected
// by graphite. The format consists of
// tagName=tagValue;tagName=tagValue....
func tagValues(t []tag.Tag) string {
	var buffer bytes.Buffer

	for _, t := range t {
		buffer.WriteString(fmt.Sprintf(";%s=%s", t.Key.Name(), t.Value))
	}
	return buffer.String()
}

type constMetric struct {
	desc string
	val  float64
}

// newConstMetric returns a constMetric struct
func newConstMetric(desc string, value float64) (constMetric, error) {
	return constMetric{
		desc: desc,
		val:  value,
	}, nil
}

// sendRequest sends a package of data containing one metric
func sendRequest(e *Exporter, data constMetric) {
	g, err := client.NewGraphite(e.opts.Host, e.opts.Port)
	if err != nil {
		e.opts.onError(fmt.Errorf("Error creating graphite: %#v", err))
	} else {
		g.SendMetric(data.desc, strconv.FormatFloat(data.val, 'f', -1, 64), time.Now())
	}
}

const labelKeySizeLimit = 128

// Sanitize returns a string that is truncated to 128 characters if it's too
// long, and replaces non-alphanumeric characters to underscores.
func sanitize(s string) string {
	if len(s) == 0 {
		return s
	}
	if len(s) > labelKeySizeLimit {
		s = s[:labelKeySizeLimit]
	}
	s = strings.Map(sanitizeRune, s)
	if unicode.IsDigit(rune(s[0])) {
		s = "key_" + s
	}
	if s[0] == '_' {
		s = "key" + s
	}
	return s
}

// sanitizeRune converts anything that is not a letter or digit to an underscore
func sanitizeRune(r rune) rune {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return r
	}
	// Everything else turns into an underscore
	return '_'
}
