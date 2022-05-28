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
	"context"
	"log"
	"strings"
	"testing"

	"bytes"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

type mSlice []*stats.Int64Measure

func (measures *mSlice) createAndAppend(name, desc, unit string) {
	m := stats.Int64(name, desc, unit)
	*measures = append(*measures, m)
}

type vCreator []*view.View

func (vc *vCreator) createAndAppend(name, description string, keys []tag.Key, measure stats.Measure, agg *view.Aggregation) {
	v := &view.View{
		Name:        name,
		Description: description,
		TagKeys:     keys,
		Measure:     measure,
		Aggregation: agg,
	}
	*vc = append(*vc, v)
}

func TestMetricsEndpointOutput(t *testing.T) {
	done := registerExporter(t)

	names := []string{"foo", "bar"}

	var measures mSlice
	for _, name := range names {
		measures.createAndAppend("tests_"+name, name, "")
	}

	var vc vCreator
	for _, m := range measures {
		vc.createAndAppend(m.Name(), m.Description(), nil, m, view.Count())
	}

	if err := view.Register(vc...); err != nil {
		t.Fatalf("failed to create views: %v", err)
	}

	for _, m := range measures {
		stats.Record(context.Background(), m.M(1))
	}

	view.Unregister(vc...) // trigger flushing of views
	output := done()

	if strings.Contains(output, "collected before with the same name and label values") {
		t.Fatal("metric name and labels being duplicated but must be unique")
	}

	if strings.Contains(output, "error(s) occurred") {
		t.Fatal("error reported by graphite registry")
	}

	for _, name := range names {
		if !strings.Contains(output, "tests_"+name) {
			t.Fatalf("measurement missing in output: %v", name)
		}
	}
}

func TestMetricsTagsOutput(t *testing.T) {
	done := registerExporter(t)

	name := "bar"

	var measures mSlice
	measures.createAndAppend(name, name, "")

	key1, err := tag.NewKey("key1")
	if err != nil {
		log.Fatal(err)
	}

	key2, err := tag.NewKey("key2")
	if err != nil {
		log.Fatal(err)
	}

	ctx, err := tag.New(context.Background(),
		tag.Insert(key1, "value1"),
		tag.Upsert(key2, "value2"),
	)
	if err != nil {
		log.Fatal(err)
	}

	var keys []tag.Key

	keys = append(keys, key1)
	keys = append(keys, key2)

	var vc vCreator
	for _, m := range measures {
		vc.createAndAppend("foo", m.Description(), keys, m, view.Count())
	}

	if err := view.Register(vc...); err != nil {
		t.Fatalf("failed to create views: %v", err)
	}

	for _, m := range measures {
		stats.Record(ctx, m.M(1))
	}

	view.Unregister(vc...) // trigger flushing of views
	output := done()
	str := strings.Trim(output, "\n")
	lines := strings.Split(str, "\n")
	want := "foo;key1=value1;key2=value2"
	ok := false
	for _, line := range lines {
		if strings.Contains(line, want) {
			ok = true
		}
	}

	if !ok {
		t.Fatalf("\ngot:\n%s\n\nwant:\n%s\n", output, want)
	}
}

func TestMetricsPathOutput(t *testing.T) {
	done := registerExporter(t)
	name := "bar"

	var measures mSlice
	measures.createAndAppend(name, name, "")

	var vc vCreator
	for _, m := range measures {
		vc.createAndAppend("foo", m.Description(), nil, m, view.Count())
	}

	if err := view.Register(vc...); err != nil {
		t.Fatalf("failed to create views: %v", err)
	}
	defer view.Unregister(vc...)

	for _, m := range measures {
		stats.Record(context.Background(), m.M(1))
	}

	view.Unregister(vc...) // trigger flushing of views
	output := done()

	lines := strings.Split(output, "\n")
	ok := false
	for _, line := range lines {
		if ok {
			break
		}
		for _, sentence := range strings.Split(line, " ") {
			if sentence == "opencensus.foo" {
				ok = true
				break
			}

			if ok {
				break
			}
		}
	}
	if !ok {
		t.Fatal("path not correct")
	}
}

func TestMetricsSumDataPathOutput(t *testing.T) {
	done := registerExporter(t)

	name := "lorem"

	var measures mSlice
	measures.createAndAppend(name, name, "")

	var vc vCreator
	for _, m := range measures {
		vc.createAndAppend("ipsum", m.Description(), nil, m, view.Sum())
	}

	if err := view.Register(vc...); err != nil {
		t.Fatalf("failed to create views: %v", err)
	}
	defer view.Unregister(vc...)

	for _, m := range measures {
		stats.Record(context.Background(), m.M(1))
	}

	view.Unregister(vc...) // trigger flushing of views
	output := done()

	lines := strings.Split(output, "\n")
	ok := false
	for _, line := range lines {
		if ok {
			break
		}
		for _, sentence := range strings.Split(line, " ") {
			if sentence == "opencensus.ipsum" {
				ok = true
				break
			}

			if ok {
				break
			}
		}
	}
	if !ok {
		t.Fatal("path not correct")
	}
}

func TestMetricsLastValueDataPathOutput(t *testing.T) {
	done := registerExporter(t)
	name := "ipsum"

	var measures mSlice
	measures.createAndAppend(name, name, "")

	var vc vCreator
	for _, m := range measures {
		vc.createAndAppend("lorem", m.Description(), nil, m, view.LastValue())
	}

	if err := view.Register(vc...); err != nil {
		t.Fatalf("failed to create views: %v", err)
	}
	defer view.Unregister(vc...)

	for _, m := range measures {
		stats.Record(context.Background(), m.M(1))
	}
	view.Unregister(vc...) // trigger flushing of views
	output := done()
	lines := strings.Split(output, "\n")
	ok := false
	for _, line := range lines {
		if ok {
			break
		}
		for _, sentence := range strings.Split(line, " ") {
			if sentence == "opencensus.lorem" {
				ok = true
				break
			}

			if ok {
				break
			}
		}
	}
	if !ok {
		t.Fatal("path not correct")
	}
}

func TestDistributionData(t *testing.T) {
	done := registerExporter(t)

	m := stats.Float64("tests/bills", "payments by denomination", stats.UnitDimensionless)
	v := &view.View{
		Name:        "cash/register",
		Description: "this is a test",
		Measure:     m,
		Aggregation: view.Distribution(1, 5, 10, 20, 50, 100, 250),
	}
	if err := view.Register(v); err != nil {
		t.Fatalf("Register error: %v", err)
	}
	values := []float64{0.25, 245.67, 12, 1.45, 199.9, 7.69, 187.12}
	ctx := context.Background()
	ms := make([]stats.Measurement, 0)
	for _, value := range values {
		mx := m.M(value)
		ms = append(ms, mx)
	}
	// We want the results that look like this:
	// 1:   [0.25]      		| 1 + prev(i) = 1 + 0 = 1
	// 5:   [1.45]			| 1 + prev(i) = 1 + 1 = 2
	// 10:	[]			| 1 + prev(i) = 1 + 2 = 3
	// 20:  [12]			| 1 + prev(i) = 1 + 3 = 4
	// 50:  []			| 0 + prev(i) = 0 + 4 = 4
	// 100: []			| 0 + prev(i) = 0 + 4 = 4
	// 250: [187.12, 199.9, 245.67]	| 3 + prev(i) = 3 + 4 = 7
	wantLines := []string{
		`opencensus.cash_register.bucket;le=1.00 1`,
		`opencensus.cash_register.bucket;le=5.00 2`,
		`opencensus.cash_register.bucket;le=10.00 3`,
		`opencensus.cash_register.bucket;le=20.00 4`,
		`opencensus.cash_register.bucket;le=50.00 4`,
		`opencensus.cash_register.bucket;le=100.00 4`,
		`opencensus.cash_register.bucket;le=250.00 7`,
		`opencensus.cash_register.bucket;le=+Inf 7`,
	}

	stats.Record(ctx, ms...)
	view.Unregister(v) // trigger flush of view
	output := done()

	for _, line := range wantLines {
		if !strings.Contains(output, line) {
			t.Fatalf("\ngot:\n%s\n\nwant:\n%s\n", output, line)
		}
	}
}

// register an exporter and return a func to close the exporter and return
// everything that has been exported.
func registerExporter(t *testing.T) func() string {
	e, err := NewExporter(Options{
		Namespace: "opencensus",
		OnError: func(err error) {
			log.Printf("OnError %s: %s\n", t.Name(), err)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var (
		g   Graphite
		buf bytes.Buffer
	)
	e.connectGraphite = func() (*Graphite, error) {
		return &g, nil
	}
	g.Conn = &buf
	view.RegisterExporter(e)
	return func() string {
		e.Flush()
		view.UnregisterExporter(e)
		e = nil
		return buf.String()
	}
}
