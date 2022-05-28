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
	"io"
	"net"
	"time"
)

// Graphite is a struct that defines the relevant properties of a graphite
// connection
type Graphite struct {
	Host    string
	Port    int
	Timeout time.Duration
	Conn    io.Writer
}

// defaultTimeout is the default number of seconds that we're willing to wait
// before forcing the connection establishment to fail
const defaultTimeout = 5

// NewGraphite is a method that's used to create a new Graphite
func NewGraphite(host string, port int) (*Graphite, error) {
	var graphite *Graphite

	graphite = &Graphite{Host: host, Port: port}
	err := graphite.connect()
	if err != nil {
		return nil, err
	}

	return graphite, nil
}

// connect populates the Graphite.conn field with an
// appropriate TCP connection
func (g *Graphite) connect() error {
	if cl, ok := g.Conn.(io.Closer); ok {
		cl.Close()
	}

	address := fmt.Sprintf("%s:%d", g.Host, g.Port)
	if g.Timeout == 0 {
		g.Timeout = defaultTimeout * time.Second
	}

	var err error
	var conn net.Conn

	conn, err = net.DialTimeout("tcp", address, g.Timeout)

	g.Conn = conn

	return err
}

// Disconnect closes the Graphite.conn field
func (g *Graphite) Disconnect() (err error) {
	if cl, ok := g.Conn.(io.Closer); ok {
		err = cl.Close()
	}
	g.Conn = nil
	return err
}

// SendMetric method can be used to just pass a metric name and value and
// have it be sent to the Graphite host
func (g *Graphite) SendMetric(metric Metric) error {
	_, err := fmt.Fprint(g.Conn, metric.String())
	if err != nil {
		return err
	}
	return nil
}
