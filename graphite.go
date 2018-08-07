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
package graphite // import "opencensus-go-exporter-graphite"

import (
	"bytes"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"go.opencensus.io/exporter/graphite/client"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

// Exporter exports stats to Graphite
type Exporter struct {
	// Options used to register and log stats
	opts Options
	c    *collector
}

// Options contains options for configuring the exporter.
type Options struct {
	// Host contains de host address for the graphite server
	// The default value is "127.0.0.1"
	Host string

	// Port is the port in which the carbon endpoint is available
	// The default value is 2003
	Port int

	// Namespace is optional and will be the first element in the path
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

	collector := newCollector(o)
	e := &Exporter{
		opts: o,
		c:    collector,
	}

	return e, nil
}

var _ view.Exporter = (*Exporter)(nil)

// registerViews creates the view map and prevents duplicated views
func (c *collector) registerViews(views ...*view.View) {
	for _, thisView := range views {
		sig := viewSignature(c.opts.Host, thisView)
		c.registeredViewsMu.Lock()
		_, ok := c.registeredViews[sig]
		c.registeredViewsMu.Unlock()
		if !ok {
			desc := sanitize(thisView.Name)
			c.registeredViewsMu.Lock()
			c.registeredViews[sig] = desc
			c.registeredViewsMu.Unlock()
		}
	}
}

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
	e.c.addViewData(vd)

	extractData(e, vd)
}

// toMetric receives the view data information and creates metrics that are adequate according to
// graphite documentation.
func (c *collector) toMetric(v *view.View, row *view.Row, vd *view.Data, e *Exporter) {
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
			path.WriteString(buildPath(names))
			path.WriteString(fmt.Sprintf(";le=%.2f", b))
			path.WriteString(tagValues(row.Tags))
			metric, _ := newConstMetric(path.String(), float64(cumCount))
			sendRequest(e, metric)
		}
		path.Reset()
		names := []string{sanitize(e.opts.Namespace), sanitize(vd.View.Name), "bucket"}
		path.WriteString(buildPath(names))
		path.WriteString(fmt.Sprintf(";le=+Inf"))
		path.WriteString(tagValues(row.Tags))
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
	path.WriteString(buildPath(names))
	path.WriteString(tagValues(row.Tags))
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
	sig := viewSignature(e.c.opts.Namespace, vd.View)
	e.c.registeredViewsMu.Lock()
	_ = e.c.registeredViews[sig]
	e.c.registeredViewsMu.Unlock()

	for _, row := range vd.Rows {
		e.c.toMetric(vd.View, row, vd, e)
	}
}

// buildPath creates the path for the metric that
// is expected by graphite. It takes a list of strings
// and joins with a dot (".")
func buildPath(names []string) string {
	var values []string
	for _, name := range names {
		if name != "" {
			values = append(values, name)
		}
	}
	return strings.Join(values, ".")
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

type collector struct {
	opts Options
	mu   sync.Mutex // mu guards all the fields.

	// viewData's are accumulated and atomically
	// appended to on every Export invocation, from
	// stats. These views are cleared out when
	// Collect is invoked and the cycle is repeated.
	viewData map[string]*view.Data

	registeredViewsMu sync.Mutex

	registeredViews map[string]string
}

// addViewData assigns the view data to the correct view
func (c *collector) addViewData(vd *view.Data) {
	c.registerViews(vd.View)
	sig := viewSignature(c.opts.Host, vd.View)

	c.mu.Lock()
	c.viewData[sig] = vd
	c.mu.Unlock()
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

// newCollector returns a collector struct
func newCollector(opts Options) *collector {
	return &collector{
		opts:            opts,
		registeredViews: make(map[string]string),
		viewData:        make(map[string]*view.Data),
	}
}

// viewName builds a unique name composed of the namespace
// and the sanitized view name. Therefore, if the namespace
// is 'opencensus and the viewName is 'cash/register'
// the return will be o'pencensus_cash_register'
func viewName(namespace string, v *view.View) string {
	var name string
	if namespace != "" {
		name = namespace + "_"
	}
	return name + sanitize(v.Name)
}

// viewSignature builds a signature that will identify a view
// The signature consists of the namespace, the viewName and the
// list of tags. Example: Namespace_viewName-tagName...
func viewSignature(namespace string, v *view.View) string {
	var buf bytes.Buffer
	buf.WriteString(viewName(namespace, v))
	for _, k := range v.TagKeys {
		buf.WriteString("-" + k.Name())
	}
	return buf.String()
}

// sendRequest sends a package of data containing one metric
func sendRequest(e *Exporter, data constMetric) {
	Graphite, err := client.NewGraphite(e.opts.Host, e.opts.Port)
	if err != nil {
		e.opts.onError(fmt.Errorf("Error creating graphite: %#v", err))
	} else {
		Graphite.SendMetric(data.desc, strconv.FormatFloat(data.val, 'f', -1, 64), time.Now())
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
