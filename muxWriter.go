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

const (
	copyPayloadBuffers = true
)

type (
	Stream = []byte

	MuxWriter interface {
		// io.Writer
		NewWriter(int) (io.Writer, error)
	}
)

type single struct {
	id  int
	mux *muxWriter

	write func([]byte) (int, error)
}

func (s *single) Write(p []byte) (int, error) {
	return s.write(p)
}

func (s *single) activeWrite(p []byte) (int, error) {
	return s.mux.write(s.id, p)
}

func (s *single) closedWrite(p []byte) (int, error) {
	return 0, errStreamIsClosed
}

func (s *single) close() {
	// mark as invalid !!!!
	s.write = s.closedWrite
}

type muxWriter struct {
	port    int
	conn    net.Conn
	guard   sync.Mutex
	packets []Stream
	writers map[int]*single
	channel chan int
}

func CreateMuxWriter(port int) MuxWriter {
	mux := muxWriter{
		port:    port,
		packets: make([]Stream, 0, 100),
		writers: make(map[int]*single),
		channel: make(chan int, 6),
	}

	go mux.sender()

	return &mux
}

func (mux *muxWriter) NewWriter(id int) (io.Writer, error) {
	if id == idFinderPrint {
		return nil, errInternalUseId
	}

	mux.lock()
	defer mux.unlock()

	if mux.writers == nil {
		mux.writers = make(map[int]*single)
	} else if writer, found := mux.writers[id]; found {
		return writer, success
	}

	writer := single{
		id:  id,
		mux: mux,
	}
	writer.write = writer.activeWrite

	mux.writers[id] = &writer
	return &writer, success
}

func (mux *muxWriter) write(id int, p []byte) (n int, err error) {
	if len(p) > 0 {
		mux.add(id, construct(id, p))
	}

	return len(p), success
}

func (mux *muxWriter) add(id int, what Stream) {
	mux.lock()
	mux.packets = append(mux.packets, what)
	mux.unlock()
	mux.signal(id)
}

func (mux *muxWriter) signal(id int) {
	if len(mux.channel) < 3 {
		mux.channel <- id
	}
}

func (mux *muxWriter) lock() {
	mux.guard.Lock()
}

func (mux *muxWriter) unlock() {
	mux.guard.Unlock()
}

func (mux *muxWriter) Close() error {
	mux.lock()
	defer mux.unlock()

	for _, writer := range mux.writers {
		writer.close()
	}

	return nil
}

func (mux *muxWriter) sender() {
	domain, address := getDomainAndAddress(mux.port)
	//	var remains []byte
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

	for { // (re-)connect loop
		var err error
		conn, err = net.DialTimeout(domain, address, timeoutDial)
		if err != success {
			warning("#4: %v\n", err)
			time.Sleep(time.Second * 10)
			continue // reconnect
		}

		if err := sendFingerprint(conn); err != success {
			warning("#4a: %v\n", err)
			continue // reconnect
		}
		/*
			todo: revisit use of remainder ...

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
		*/

	Inner:
		for { // data sending loop
			select {
			case a, channelOpen := <-mux.channel:
				if !channelOpen {
					warning("channel closed - quitting")
					return
				}

				// warning("      woken up by %v\n", a)
				_ = a

				if err := mux.sendAllAvailableData(write); err != success {
					break Inner
				}
			}
		}

		conn.Close()
	}
}

func (mux *muxWriter) sendAllAvailableData(write func(b []byte) (n int, err error)) error {
	for {
		var data Stream

		mux.lock()
		if len(mux.packets) > 0 {
			data = mux.packets[0]
			mux.packets = mux.packets[1:]
		}
		mux.unlock()

		if len(data) == 0 {
			// sent everything there was
			return success
		}

		if n, err := write(data); err != success {
			warning("failed to write: %v\n", err)
			/*
				if n > 0 {
					// looks like we managed to send "n" bytes - need to preserve the rest (to send on next connect)
					if n < len(data) {
						remains = data[n:]
					}
				}
			*/
			return err
		} else if n != len(data) {
			// todo: ???
		}
	}
}
