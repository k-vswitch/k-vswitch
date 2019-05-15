/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package connection

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"

	"k8s.io/klog"
)

const (
	listenPort = 6653
)

// OFConnecter handles message processes coming from a specific connection
// There should be only one instance of OFConn per connection from the switch
type OFConnect struct {
	listener *net.TCPListener

	queue     []ofp13.OFMessage
	queueMu   sync.Mutex
	queueCond sync.Cond

	conn     *net.TCPConn
	connMu   sync.Mutex
	connCond sync.Cond

	receiveCh chan ofp13.OFMessage
	sendCh    chan ofp13.OFMessage
}

func NewOFConnect() (*OFConnect, error) {
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", listenPort))
	if err != nil {
		return nil, err
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return nil, err
	}

	of := &OFConnect{
		listener:  listener,
		queue:     make([]ofp13.OFMessage, 0),
		receiveCh: make(chan ofp13.OFMessage),
		sendCh:    make(chan ofp13.OFMessage),
	}

	of.queueCond.L = &of.queueMu
	of.connCond.L = &of.connMu
	return of, nil
}

func (of *OFConnect) Receive() ofp13.OFMessage {
	msg := <-of.receiveCh
	return msg
}

func (of *OFConnect) Send(msg ofp13.OFMessage) {
	of.queueMu.Lock()
	defer of.queueMu.Unlock()

	of.queue = append(of.queue, msg)
	of.queueCond.Broadcast()
}

func (of *OFConnect) SetConnection(conn *net.TCPConn) {
	of.connMu.Lock()
	defer of.connMu.Unlock()

	of.conn = conn
	of.connCond.Broadcast()
}

func (of *OFConnect) WriteConnection(msg ofp13.OFMessage) error {
	of.connMu.Lock()
	defer of.connMu.Unlock()

	if of.conn == nil {
		return errors.New("main connection not established yet")
	}

	_, err := of.conn.Write(msg.Serialize())
	return err
}

func (of *OFConnect) ProcessQueue() {
	for {

		of.connMu.Lock()
		// don't start processing the queue until
		// a main connection is established
		if of.conn == nil {
			of.connCond.Wait()
		}
		of.connMu.Unlock()

		of.queueMu.Lock()
		if len(of.queue) == 0 {
			of.queueCond.Wait()
		}

		msg := of.queue[0]
		err := of.WriteConnection(msg)
		if err != nil {
			klog.Errorf("error writing to connection: %v", err)
			of.queueMu.Unlock()
			continue
		}

		of.queue = of.queue[1:]
		of.queueMu.Unlock()
	}
}

func (of *OFConnect) Serve() {
	for {
		conn, err := of.listener.AcceptTCP()
		if err != nil {
			klog.Errorf("error accepting TCP connections: %v", err)
			continue
		}

		of.SetConnection(conn)
		go of.handleConn(conn)
	}
}

func (of *OFConnect) handleConn(conn *net.TCPConn) {
	defer conn.Close()

	for {
		reader := bufio.NewReader(conn)

		// peak into the first 8 bytes (the size of OF header messages)
		// the header message contains the length of the entire message
		// which we need later to move the reader forward
		header, err := reader.Peek(8)
		if err != nil {
			klog.Errorf("could not peek at message header: %v", err)
			continue
		}

		msgLen := MessageLength(header)
		buf := make([]byte, msgLen)

		_, err = reader.Read(buf)
		if err != nil {
			if opErr, ok := err.(*net.OpError); !ok || !opErr.Timeout() {
				if err != io.EOF {
					klog.Errorf("error reading connection: %s", err.Error())
				}
				return
			}

			continue
		}

		msg := ParseMessage(buf)
		of.receiveCh <- msg
	}
}
