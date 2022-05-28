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

// Package graphite contains a Graphite exporter that supports exporting
// OpenCensus views as Graphite metrics.
package graphite // import "contrib.go.opencensus.io/exporter/graphite"

import (
	"bytes"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
	"unicode"

	"os"

	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"google.golang.org/api/support/bundler"
)

var debug bool

func init() {
	debug = os.Getenv("OPENCENSUS_GRAPHITE_DEBUG") != ""
}

// Exporter exports stats to Graphite
type Exporter struct {
	// Options used to register and log stats
	opts            Options
	tags            []tag.Tag
	bundler         *bundler.Bundler
	pathBuilder     func(nameSpace, viewName string, rowTags, defaultTags []tag.Tag) string
	connectGraphite func() (*Graphite, error)
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

	// Tags specifies a set of default tags to attach to each metric.
	// Tags is optional and will work only for Graphite above 1.1.x.
	Tags []tag.Tag

	// OnError is the hook to be called when there is
	// an error uploading the stats or tracing data.
	// If no custom hook is set, errors are logged.
	// Optional.
	OnError func(err error)

	// MetricPathBuilder return a custom metric path
	// namespace is Options.Namespace, rowTags are the metric tags and
	// defaultTags are tags from Options.Tags
	MetricPathBuilder func(nameSpace, viewName string, rowTags, defaultTags []tag.Tag) string
}

const (
	defaultBufferedViewDataLimit = 10000 // max number of view.Data in flight
	defaultBundleCountThreshold  = 100   // max number of view.Data per bundle
)

// defaultDelayThreshold is the amount of time we wait to receive new view.Data
// from the aggregation pipeline. We normally expect to receive it in rapid
// succession, so we set this to a small value to avoid waiting
// unnecessarily before submitting.
const defaultDelayThreshold = 200 * time.Millisecond

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

	e.tags = append(e.tags, o.Tags...)

	e.pathBuilder = defaultPathBuilder
	if o.MetricPathBuilder != nil {
		e.pathBuilder = o.MetricPathBuilder
	}

	b := bundler.NewBundler((*view.Data)(nil), func(items interface{}) {
		vds := items.([]*view.Data)
		e.sendBundle(vds)
	})
	e.bundler = b

	e.bundler.BufferedByteLimit = defaultBufferedViewDataLimit
	e.bundler.BundleCountThreshold = defaultBundleCountThreshold
	e.bundler.DelayThreshold = defaultDelayThreshold

	e.connectGraphite = func() (*Graphite, error) {
		return NewGraphite(o.Host, o.Port)
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
	e.bundler.Add(vd, 1)
}

func (e *Exporter) Flush() {
	e.bundler.Flush()
}

// toMetric receives the view data information and creates metrics that are adequate according to
// graphite documentation.
func (e *Exporter) toMetric(v *view.View, row *view.Row, vd *view.Data) []Metric {
	switch data := row.Data.(type) {
	case *view.CountData:
		return []Metric{e.formatTimeSeriesMetric(data.Value, row, vd)}
	case *view.SumData:
		return []Metric{e.formatTimeSeriesMetric(data.Value, row, vd)}
	case *view.LastValueData:
		return []Metric{e.formatTimeSeriesMetric(data.Value, row, vd)}
	case *view.DistributionData:
		// Graphite does not support histogram. In order to emulate one,
		// we use the accumulative count of the bucket.
		indicesMap := make(map[float64]int)
		buckets := make([]float64, 0, len(v.Aggregation.Buckets))
		for i, b := range v.Aggregation.Buckets {
			if _, ok := indicesMap[b]; !ok {
				indicesMap[b] = i
				buckets = append(buckets, b)
			}
		}
		sort.Float64s(buckets)

		var metrics []Metric

		// Now that the buckets are sorted by magnitude
		// we can create cumulative indicesmap them back by reverse index
		cumCount := uint64(0)
		for _, b := range buckets {
			i := indicesMap[b]
			cumCount += uint64(data.CountPerBucket[i])
			rowTags := append(row.Tags, tag.Tag{Key: tag.MustNewKey("bucket"), Value: fmt.Sprintf("le=%.2f", b)})
			metric := Metric{
				Name:      e.pathBuilder(sanitize(e.opts.Namespace), sanitize(vd.View.Name), rowTags, e.tags),
				Value:     float64(cumCount),
				Timestamp: vd.End,
			}
			metrics = append(metrics, metric)
		}
		rowTags := append(row.Tags, tag.Tag{Key: tag.MustNewKey("bucket"), Value: "le=+Inf"})
		metric := Metric{
			Name:      e.pathBuilder(sanitize(e.opts.Namespace), sanitize(vd.View.Name), rowTags, e.tags),
			Value:     float64(cumCount),
			Timestamp: vd.End,
		}
		metrics = append(metrics, metric)
		return metrics
	default:
		e.opts.onError(fmt.Errorf("aggregation %T is not yet supported", data))
		return nil
	}
}

// formatTimeSeriesMetric creates a CountData metric, SumData or LastValueData
// and returns it to toMetric
func (e *Exporter) formatTimeSeriesMetric(value interface{}, row *view.Row, vd *view.Data) Metric {
	var val float64
	switch x := value.(type) {
	case int64:
		val = float64(x)
	case float64:
		val = x
	}
	return Metric{
		Name:      e.pathBuilder(sanitize(e.opts.Namespace), sanitize(vd.View.Name), row.Tags, e.tags),
		Value:     val,
		Timestamp: vd.End,
	}
}

// sendBundle extracts stats data and calls toMetric
// to convert the data to metrics formatted to graphite
func (e *Exporter) sendBundle(vds []*view.Data) {
	g, err := e.connectGraphite()
	if err != nil {
		e.opts.onError(err)
		return
	}
	defer g.Disconnect()
	for _, vd := range vds {
		for _, row := range vd.Rows {
			for _, metric := range e.toMetric(vd.View, row, vd) {
				debugOut("send", metric)
				err = g.SendMetric(metric)
				if err != nil {
					e.opts.onError(err)
				}
			}
		}
	}
}

func defaultPathBuilder(nameSpace, viewName string, rowTags, defaultTags []tag.Tag) string {
	names := []string{nameSpace, viewName}
	return buildPath(names, tagValues(rowTags), tagValues(defaultTags))
}

// buildPath creates the path for the metric that
// is expected by graphite. It takes a list of strings
// and joins with a dot (".")
func buildPath(names []string, tags string, eTags string) string {
	var values []string
	for _, name := range names {
		if name != "" {
			values = append(values, name)
		}
	}
	path := strings.Join(values, ".")
	return path + tags + eTags
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

func debugOut(a ...interface{}) {
	if debug {
		p := make([]interface{}, len(a)+1)
		copy(p[1:], a)
		p[0] = "graphite:"
		fmt.Println(p...)
	}
}
