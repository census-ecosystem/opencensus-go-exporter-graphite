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

package client

import (
	"bytes"
	"fmt"
	"net"
	"time"
)

// Graphite is a struct that defines the relevant properties of a graphite
// connection
type Graphite struct {
	Host    string
	Port    int
	Timeout time.Duration
	conn    net.Conn
}

// defaultTimeout is the default number of seconds that we're willing to wait
// before forcing the connection establishment to fail
const defaultTimeout = 5

// NewGraphite is a method that's used to create a new Graphite
func NewGraphite(host string, port int) (*Graphite, error) {
	var graphite *Graphite

	graphite = &Graphite{Host: host, Port: port}
	err := graphite.Connect()
	if err != nil {
		return nil, err
	}

	return graphite, nil
}

// Connect populates the Graphite.conn field with an
// appropriate TCP connection
func (graphite *Graphite) Connect() error {
	if graphite.conn != nil {
		graphite.conn.Close()
	}

	address := fmt.Sprintf("%s:%d", graphite.Host, graphite.Port)
	if graphite.Timeout == 0 {
		graphite.Timeout = defaultTimeout * time.Second
	}

	var err error
	var conn net.Conn

	conn, err = net.DialTimeout("tcp", address, graphite.Timeout)

	graphite.conn = conn

	return err
}

// Disconnect closes the Graphite.conn field
func (graphite *Graphite) Disconnect() error {
	err := graphite.conn.Close()
	graphite.conn = nil
	return err
}

// SendMetric method can be used to just pass a metric name and value and
// have it be sent to the Graphite host
func (graphite *Graphite) SendMetric(stat string, value string, timestamp time.Time) error {
	metrics := make([]Metric, 1)
	metrics[0] = Metric{
		Name:      stat,
		Value:     value,
		Timestamp: timestamp,
	}
	err := graphite.sendMetrics(metrics)
	if err != nil {
		return err
	}
	return nil
}

// sendMetrics is an unexported function that is used to write to the TCP
// connection in order to communicate metrics to the remote Graphite host
func (graphite *Graphite) sendMetrics(metrics []Metric) error {
	zeroedMetric := Metric{}
	var buf bytes.Buffer
	for _, metric := range metrics {
		if metric == zeroedMetric {
			continue
		}

		buf.WriteString(metric.String())
	}
	_, err := graphite.conn.Write(buf.Bytes())
	if err != nil {
		return err
	}
	return nil
}
