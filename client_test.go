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
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Change these to be your own graphite server if you so please

const (
	graphiteHost = "127.0.0.1"
	graphitePort = 2003
)

var output = &SafeString{}

type SafeString struct {
	output string
	m      sync.Mutex
}

func (i *SafeString) Get() string {
	// The `Lock` method of the mutex blocks if it is already locked
	// if not, then it blocks other calls until the `Unlock` method is called
	i.m.Lock()
	// Defer `Unlock` until this method returns
	defer i.m.Unlock()
	// Return the value
	return i.output
}

func (i *SafeString) Set(val string) {
	// Similar to the `Get` method, except we Lock until we are done
	// writing to `i.closeConn`
	i.m.Lock()
	defer i.m.Unlock()
	i.output = i.output + val
}

var closeConn = &SafeBool{}

type SafeBool struct {
	closeConn bool
	m         sync.Mutex
}

func (i *SafeBool) Get() bool {
	// The `Lock` method of the mutex blocks if it is already locked
	// if not, then it blocks other calls until the `Unlock` method is called
	i.m.Lock()
	// Defer `Unlock` until this method returns
	defer i.m.Unlock()
	// Return the value
	return i.closeConn
}

func (i *SafeBool) Set(val bool) {
	// Similar to the `Get` method, except we Lock until we are done
	// writing to `i.closeConn`
	i.m.Lock()
	defer i.m.Unlock()
	i.closeConn = val
}

func handler(l net.Listener) {
	for {
		if closeConn.Get() {
			l.Close()
			return
		}
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}

		// Make a buffer to hold incoming data.
		buf := make([]byte, 1024)
		r := bufio.NewReader(conn)

		// Read the incoming connection into the buffer.
		reqLen, err := r.Read(buf)
		data := string(buf[:reqLen])

		switch err {
		case nil:
			output.Set(data)
		default:
			log.Fatal("Connection Error")
			return
		}
	}
}

func TestSendMetric(t *testing.T) {
	closeConn.Set(false)

	l, err := net.Listen("tcp", graphiteHost+":"+strconv.Itoa(graphitePort))
	if err != nil {
		t.Fatalf("Error listening: %v", err.Error())
	}
	go func() {
		handler(l)
	}()
	gr, err := NewGraphite(graphiteHost, graphitePort)

	if err != nil {
		t.Error(err)
	}

	if _, ok := gr.Conn.(*net.TCPConn); !ok {
		t.Error("GraphiteHost.conn is not a TCP connection")
	}

	metricName := "graphite.path"
	metricValue := 2.0
	gr.SendMetric(Metric{metricName, metricValue, time.Now()})
	<-time.After(10 * time.Millisecond)

	closeConn.Set(true)

	o := output.Get()
	if !strings.Contains(o, "graphite.path 2") {
		t.Fatal("metric name and value are not being sent:", o)
	}

	gr.Disconnect()
	<-time.After(10 * time.Millisecond)
}

func TestNewGraphite(t *testing.T) {
	closeConn.Set(false)

	l, err := net.Listen("tcp", graphiteHost+":")
	if err != nil {
		t.Fatalf("Error listening: %v", err.Error())
	}
	go func() {
		handler(l)
	}()

	port, _ := strconv.Atoi(strings.Split(l.Addr().String(), ":")[1])
	gh, err := NewGraphite(graphiteHost, port)
	if err != nil {
		t.Error(err)
	}

	closeConn.Set(true)

	if _, ok := gh.Conn.(*net.TCPConn); !ok {
		t.Error("GraphiteHost.conn is not a TCP connection")
	}

}

func TestGraphiteFactoryTCP(t *testing.T) {
	closeConn.Set(false)

	l, err := net.Listen("tcp", graphiteHost+":")
	if err != nil {
		t.Fatalf("Error listening: %v", err.Error())
	}
	go func() {
		handler(l)
	}()

	port, _ := strconv.Atoi(strings.Split(l.Addr().String(), ":")[1])
	gr, err := NewGraphite(graphiteHost, port)

	if err != nil {
		t.Error(err)
	}

	if _, ok := gr.Conn.(*net.TCPConn); !ok {
		t.Error("GraphiteHost.conn is not a TCP connection")
	}

	closeConn.Set(true)
}

func TestInvalidHost(t *testing.T) {
	closeConn.Set(false)

	l, err := net.Listen("tcp", graphiteHost+":")
	if err != nil {
		t.Fatalf("Error listening: %v", err.Error())
	}
	go func() {
		handler(l)
	}()

	port, _ := strconv.Atoi(strings.Split(l.Addr().String(), ":")[1])

	_, err = NewGraphite("Invalid", port)
	if err == nil {
		t.Fatal("an error should have been raised")
	}
	closeConn.Set(true)
}
