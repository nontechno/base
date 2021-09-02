// Copyright 2021 The NonTechno Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"net"
	"strconv"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

type (
	Connection interface {
		OnNewMessage(id int, data []byte)
		OnDisconnect(reason error)
	}

	NewConnection func(props map[string]interface{}) Connection
)

// Creates a new "receiver", if specified port is set to  0 (zero), a random port will be selected (and returned)
func CreateReceiver(port int, maker NewConnection) (int, error) {
	if port < 0 {
		log.Errorf("port number cannot be negative")
		return port, errNegativePortNumber
	}

	if maker == nil {
		log.Errorf("supplied maker pointer is nil")
		return port, errParamIsNil
	}

	domain, address := getDomainAndAddress(port)
	l, err := net.Listen(domain, address)
	if err != success {
		GetLogger().WithError(err).Errorf("failed to start listen, port: %v", port)
		return 0, err
	}

	if port == 0 {
		parts := strings.Split(l.Addr().String(), ":")
		portPart := parts[len(parts)-1]
		port, err = strconv.Atoi(portPart)
		if err != success {
			GetLogger().WithError(err).Errorf("failed to parse port number (%v)", portPart)
			l.Close()
			return 0, err
		}
	}

	receiver := muxReceiver{
		maker:    maker,
		port:     port,
		listener: l,
	}

	{
		muxGuard.Lock()
		muxReceivers = append(muxReceivers, &receiver)
		muxGuard.Unlock()
	}

	go func(receiver *muxReceiver) {
		defer l.Close()
		log := GetLogger().WithFields(map[string]interface{}{"addr.local": l.Addr().String(), "port": port})

		for {
			log.Tracef("waiting for incoming connection")
			if conn, err := l.Accept(); err != success {
				log.WithError(err).Errorf("Accept failed")
			} else {
				log.Tracef("got new connection from: %v", conn.RemoteAddr().String())
				go muxReceiverThread(log, conn, receiver)
			}
		}

	}(&receiver)

	return port, success

}

type muxReceiver struct {
	maker    NewConnection
	port     int
	listener net.Listener
}

var (
	muxReceivers []*muxReceiver
	muxGuard     sync.Mutex
)

func CloseReceiver(port int) error {

	var receiver *muxReceiver
	var index int

	muxGuard.Lock()
	for index, receiver = range muxReceivers {
		if receiver != nil && receiver.port == port {
			muxReceivers = append(muxReceivers[:index], muxReceivers[index+1:]...)
			break
		}
	}
	muxGuard.Unlock()

	if receiver == nil {
		GetLogger().Warning("failed to find a receiver with specified port [%v]", port)
		return success
	}

	err := receiver.listener.Close()

	return err
}

// ...
// this func takes "chan" as input to allow multiplexing of many stream on the same chan
// you can differenciate between different messages using "id" that the caller specifies and it will be set in Pack.ID
func CreateMuxReceiver(port int, channel chan Pack, id PackIDType) (int, error) {
	if channel == nil {
		GetLogger().Errorf("the supplied chan is nil...")
		return 0, errParamIsNil
	}

	if port < 0 {
		port = 0
	}

	domain, address := getDomainAndAddress(port)
	l, err := net.Listen(domain, address)
	if err != success {
		GetLogger().WithError(err).Errorf("failed to start listen, port: %v", port)
		return 0, err
	}

	if port == 0 {
		parts := strings.Split(l.Addr().String(), ":")
		portPart := parts[len(parts)-1]
		port, err = strconv.Atoi(portPart)
		if err != success {
			GetLogger().WithError(err).Errorf("failed to parse port number (%v)", portPart)
			return 0, err
		}
	}

	go func(l net.Listener, channel chan Pack, id PackIDType) {
		defer l.Close()
		log := GetLogger().WithFields(map[string]interface{}{
			"addr.local": l.Addr().String(),
			"pack.id":    id,
		})

		for {
			log.Tracef("waiting for incoming connection")
			if conn, err := l.Accept(); err != success {
				log.WithError(err).Errorf("Accept failed")
			} else {
				log.Tracef("got new connection from: %v", conn.RemoteAddr().String())
				go muxReceiverThread_prev(log, conn, channel, id)
			}
		}

	}(l, channel, id)

	return port, success
}

func muxReceiverThread(log *log.Entry, conn net.Conn, receiver *muxReceiver) {
	if conn == nil {
		log.Debugf("the supplied connection is nil")
		return
	}

	log = log.WithField("addr.remote", conn.RemoteAddr().String())

	defer conn.Close()

	//	channel <- Pack{Action: Connect, ID: id}

	buf := make([]byte, msgSize)
	var total uint64
	var connection Connection

	for {

		nread, err := conn.Read(buf)
		if err != nil {
			log.WithError(err).Errorf("failed to read")
			break
		}
		if nread == 0 {
			break
		}
		total += uint64(nread)
		log.Tracef("received %v bytes", total)

		buffer := buf[:nread]

		for {
			id, payload, remainder, err := deconstruct(buffer)
			if err == success {
				if id == idFinderPrint {
					fp, err := extractFingerprint(payload)
					if err != success {
						log.WithError(err).Errorf("failed to get the fingerprint")
						// todo: ???????????
					}

					fp["remote.addr"] = conn.RemoteAddr().String()

					if connection == nil {
						connection = receiver.maker(fp)
					} else {
						connection.OnNewMessage(id, payload) // warning: calling user's code on the receiving thread
					}
				} else {
					if connection != nil {
						connection.OnNewMessage(id, payload) // warning: calling user's code on the receiving thread
					} else {
						log.Warning("a message with no connection")
					}
				}
			} else if err == errIncompleteData {
				log.Warning("failed to receive complete package - need to get more")
			} else {
				log.WithError(err).Errorf("failed to deconstruct incoming data")
			}

			if remainder > 0 {
				buffer = buffer[len(buffer)-remainder:]
			} else {
				break
			}
		}

		/*
			} else {

					connection.OnNewMessage(id, payload)
					for remainder > 0 {
						buffer = buffer[len(buffer)-remainder:]
						goto Processing
					}
				}
		*/

	}

	if connection != nil {
		connection.OnDisconnect(nil)
	}
}

func muxReceiverThread_prev(log *log.Entry, conn net.Conn, channel chan Pack, id PackIDType) {
	if conn == nil {
		log.Debugf("the supplied connection is nil")
		return
	}

	log = log.WithField("addr.remote", conn.RemoteAddr().String())

	defer conn.Close()

	channel <- Pack{Action: Connect, ID: id}

	buf := make([]byte, msgSize)
	var total uint64

	for {
		// todo: set a timeout?
		nread, err := conn.Read(buf)
		if err != nil {
			log.WithError(err).Errorf("failed to read")
			break
		}
		if nread == 0 {
			break
		}
		total += uint64(nread)
		log.Tracef("received %v bytes", total)

		pack := Pack{
			Action:  Data,
			ID:      id,
			Payload: make([]byte, nread),
		}
		copy(pack.Payload, buf)

		channel <- pack

		if total > 20014400 { // todo: review this
			// simulate loss of connection
			// break
		}
	}

	channel <- Pack{Action: Disconnect, ID: id}
}
