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

var output = ""
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

func startServer() {
	l, err := net.Listen("tcp", graphiteHost+":"+strconv.Itoa(graphitePort))
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		return
	}

	// Close the listener when the application closes.
	defer l.Close()
	fmt.Println("Listening on " + graphiteHost + ":" + strconv.Itoa(graphitePort))
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
		// Handle connections in a new goroutine.
		handleRequest(conn)
	}
}

// Handles incoming requests.
func handleRequest(conn net.Conn) {
	// Make a buffer to hold incoming data.
	buf := make([]byte, 1024)
	r := bufio.NewReader(conn)

	defer conn.Close()
	// Read the incoming connection into the buffer.
	reqLen, err := r.Read(buf)
	data := string(buf[:reqLen])

	switch err {
	case nil:
		output = output + data
	default:
		log.Fatalf("Receive data failed:%s", err)
		return
	}
}

func TestSendMetric(t *testing.T) {
	closeConn.Set(false)
	go startServer()
	gr, err := NewGraphite(graphiteHost, graphitePort)

	if err != nil {
		t.Error(err)
	}

	if _, ok := gr.conn.(*net.TCPConn); !ok {
		t.Error("GraphiteHost.conn is not a TCP connection")
	}

	metricName := "graphite.path"
	metricValue := "2"
	gr.SendMetric(metricName, metricValue, time.Now())
	<-time.After(10 * time.Millisecond)

	closeConn.Set(true)
	<-time.After(10 * time.Millisecond)

	if !strings.Contains(output, metricName+" "+metricValue) {
		t.Fatal("metric name and value are not being sent")
	}
}

func TestGraphiteFactoryTCP(t *testing.T) {
	closeConn.Set(false)
	go startServer()
	gr, err := NewGraphite(graphiteHost, graphitePort)

	if err != nil {
		t.Error(err)
	}

	closeConn.Set(true)
	if _, ok := gr.conn.(*net.TCPConn); !ok {
		t.Error("GraphiteHost.conn is not a TCP connection")
	}

}

func TestNewGraphite(t *testing.T) {
	closeConn.Set(false)
	go startServer()
	gh, err := NewGraphite(graphiteHost, graphitePort)
	if err != nil {
		t.Error(err)
	}

	if _, ok := gh.conn.(*net.TCPConn); !ok {
		t.Error("GraphiteHost.conn is not a TCP connection")
	}

	closeConn.Set(true)
}

func TestInvalidHost(t *testing.T) {
	closeConn.Set(false)
	go startServer()
	_, err := NewGraphite("Invalid", graphitePort)
	if err == nil {
		t.Fatal("an error should have been raised")
	}
	closeConn.Set(true)
}
