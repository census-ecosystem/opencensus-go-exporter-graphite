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

// Command graphite is an example program that collects data for
// video size. Collected data is exported to Graphite.
package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"time"

	"contrib.go.opencensus.io/exporter/graphite"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

// Create measures. We will record measures for the size of
// processed videos and the number of videos.
var (
	videoCount = stats.Int64("count", "number of processed videos", stats.UnitDimensionless)
	videoSize  = stats.Int64("size", "size of processed video", stats.UnitBytes)
)

func main() {
	ctx := context.Background()

	exporter, err := graphite.NewExporter(graphite.Options{Namespace: "opencensus"})
	if err != nil {
		log.Fatal(err)
	}
	view.RegisterExporter(exporter)

	// Create view to see the number of processed videos cumulatively.
	// Create view to see the amount of video processed
	// Subscribe will allow view data to be exported.
	// Once no longer needed, we can unsubscribe from the view.
	if err = view.Register(
		&view.View{
			Name:        "video_count",
			Description: "number of videos processed over time",
			Measure:     videoCount,
			TagKeys:     nil,
			Aggregation: view.Count(),
		},
		&view.View{
			Name:        "video_size",
			Description: "processed video size over time",
			Measure:     videoSize,
			TagKeys:     nil,
			Aggregation: view.Count(),
		},
	); err != nil {
		log.Fatalf("Cannot register the view: %v", err)
	}

	view.SetReportingPeriod(1 * time.Second)

	// Record some data points...
	// The path of the records will be
	// opencensus.video_count
	// and
	// opencensus.video_size
	go func() {
		for {
			stats.Record(ctx, videoCount.M(1), videoSize.M(rand.Int63()))
			<-time.After(time.Millisecond * time.Duration(1+rand.Intn(400)))
		}
	}()

	addr := ":2004"
	log.Printf("Serving at %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
