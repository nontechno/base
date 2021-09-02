// Copyright 2021 The NonTechno Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"net"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

type Operation byte

const (
	Data Operation = iota
	Connect
	Disconnect
)

type (
	PackIDType = interface{}

	// this is the structure being passed around through the channel
	Pack struct {
		Action  Operation
		ID      PackIDType
		Payload []byte
	}

	Hose = chan Pack
)

// ...
// this func takes "chan" as input to allow multiplexing of many stream on the same chan
// you can differenciate between different messages using "id" that the caller specifies and it will be set in Pack.ID
func CreateIpcReceiver(port int, channel chan Pack, id PackIDType) (int, error) {
	if channel == nil {
		GetLogger().Errorf("the supplied chan is nil...")
		return 0, errParamIsNil
	}

	if port < 0 {
		port = 0
	}

	domain, address := getDomainAndAddress(port)
	l, err := net.Listen(domain, address)
	if err != nil {
		GetLogger().WithError(err).Errorf("failed to start listen, port: %v", port)
		return 0, err
	}

	if port == 0 {
		parts := strings.Split(l.Addr().String(), ":")
		portPart := parts[len(parts)-1]
		port, err = strconv.Atoi(portPart)
		if err != nil {
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

		log.Tracef("listening")
		for {
			if conn, err := l.Accept(); err != nil {
				log.WithError(err).Errorf("Accept failed")
			} else {
				go serverThread(log, conn, channel, id)
			}
		}

	}(l, channel, id)

	return port, nil
}

func serverThread(log *log.Entry, conn net.Conn, channel chan Pack, id PackIDType) {
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

var (
	msgSize = 500 * 1000
)
