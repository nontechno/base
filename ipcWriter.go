// Copyright 2021 The NonTechno Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"io"
	"net"
	"sync"
	"time"
)

var (
	timeoutDial  time.Duration = time.Second * 5
	timeoutWrite time.Duration = time.Second * 5
)

func CreateIpcWriter(port int) io.Writer {
	ipc := ipcWriter{
		port:    port,
		channel: make(chan bool, 6),
	}
	go ipc.sender()

	return &ipc
}

type ipcWriter struct {
	port    int
	conn    net.Conn
	guard   sync.Mutex
	buffer  []byte
	channel chan bool
}

func (ipc *ipcWriter) Write(p []byte) (n int, err error) {

	// 1. preserve / offload
	ipc.lock()
	ipc.buffer = append(ipc.buffer, p...)
	ipc.unlock()

	// 2. signal
	ipc.signal()

	return len(p), nil
}

func (ipc *ipcWriter) Close() error {
	close(ipc.channel) // this should terminate the goroutine
	return nil
}

func (ipc *ipcWriter) lock() {
	ipc.guard.Lock()
}

func (ipc *ipcWriter) unlock() {
	ipc.guard.Unlock()
}

func (ipc *ipcWriter) signal() {
	if len(ipc.channel) < 3 {
		ipc.channel <- true
	}
}

func (ipc *ipcWriter) sender() {
	domain, address := getDomainAndAddress(ipc.port)
	var remains []byte
	var conn net.Conn

	// timeoutWrite
	write := func(b []byte) (n int, err error) {
		if conn != nil {
			if err := conn.SetWriteDeadline(time.Now().Add(timeoutWrite)); err != nil {
			}
			return conn.Write(b)
		}
		return 0, errNotConnected
	}

Outer:
	for { // (re-)connect loop
		var err error
		conn, err = net.DialTimeout(domain, address, timeoutDial)
		if err != nil {
			warning("#4: %v\n", err)
			time.Sleep(time.Second * 10)
			continue
		}

		// send possible remains first
		for len(remains) > 0 {
			if n, err := write(remains); err != nil {
				warning("failed to write: %v\n", err)
				if n > 0 {
					// looks like we managed to send "n" bytes - need to preserve the rest (to send on next connect)
					if n < len(remains) {
						remains = remains[n:]
					}
				}
				break Outer
			} else {
				remains = remains[n:]
			}
		}

	Inner:
		for { // data sending loop
			select {
			case a, b := <-ipc.channel:
				if !b {
					warning("channel closed - quitting")
					return
				}
				_ = a
				ipc.lock()
				data := ipc.buffer
				ipc.buffer = nil
				ipc.unlock()

				if len(data) == 0 {
					break
				}

				if n, err := write(data); err != nil {
					warning("failed to write: %v\n", err)
					if n > 0 {
						// looks like we managed to send "n" bytes - need to preserve the rest (to send on next connect)
						if n < len(data) {
							remains = data[n:]
						}
					}
					break Inner
				} else if n != len(data) {
					// todo: ???
				}
			}
		}

		conn.Close()
	}
}

func getDomainAndAddress(port int) (string, string) {
	return "tcp", sprintf("127.0.0.1:%d", port)
}

/*
	scenarios to consider
	0. if there is no "writes" - there is no "problem"
	1. there are active, repeating writes
		a. if there is no connection:
			- accumulate the data. till when?
			- continue connect attempts. till when?
		b. if there is a connection and "reads" are faster than "writes"
			(we are good - nothing to do here)
		c. if there is a connection and "reads" are slower than "writes"
			- accumulate the unsent data. till when?



*/
