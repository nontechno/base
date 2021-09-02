// Copyright 2021 The NonTechno Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"encoding/binary"
	"encoding/json"
	"net"
	"os"
	"os/user"
	"runtime"
	"time"
)

const (
	MuxStdout  = 1
	MuxStderr  = 3
	MuxLogger  = 7
	MuxMetrics = 11
	MuxUser    = 100
)

const (
	sizeOfInt     = 4
	prefixSize    = sizeOfInt + sizeOfInt
	idFinderPrint = 0
)

type (
	Fingerprint = map[string]interface{}
)

func construct(id int, p Stream) Stream {
	l := len(p)
	result := make(Stream, prefixSize+l)
	binary.LittleEndian.PutUint32(result[0:], uint32(id))
	binary.LittleEndian.PutUint32(result[sizeOfInt:], uint32(l))
	if l > 0 {
		copy(result[prefixSize:], p)
	}
	return result
}

func deconstruct(input Stream) (int, Stream, int, error) {
	if len(input) < prefixSize {
		return 0, nil, 0, errIncompleteData
	}
	id := int(binary.LittleEndian.Uint32(input[:sizeOfInt]))
	size := binary.LittleEndian.Uint32(input[sizeOfInt:prefixSize])

	if len(input) < (int(size) + prefixSize) {
		return 0, nil, 0, errIncompleteData
	}

	data := make(Stream, size)
	copy(data, input[prefixSize:])

	return id, data, len(input) - int(size) - prefixSize, nil
}

func assembleFingerprint() Stream {
	facts := make(Fingerprint)

	facts["pid"] = os.Getpid()
	facts["arch"] = runtime.GOARCH
	facts["os"] = runtime.GOOS
	facts["cpu"] = runtime.NumCPU()
	facts["version"] = runtime.Version()
	facts["app"] = os.Args[0]

	if hostname, err := os.Hostname(); err == nil {
		facts["hostname"] = hostname
	}

	if usr, err := user.Current(); err == nil {
		facts["user"] = usr.Name
		facts["username"] = usr.Username
	}

	// var mem runtime.MemStats
	// runtime.ReadMemStats(&mem)

	if data, err := json.Marshal(facts); err != success {
		return []byte{}
	} else {
		return data
	}
}

func sendFinderprint(conn net.Conn) error {
	fp := assembleFingerprint()
	what := construct(idFinderPrint, fp)

	if err := conn.SetWriteDeadline(time.Now().Add(timeoutWrite)); err != success {
		return err
	}
	n, err := conn.Write(what)
	if n != len(what) {
		// todo: react
	}
	return err
}

func extractFingerprint(from Stream) (Fingerprint, error) {
	facts := make(map[string]interface{})
	if err := json.Unmarshal(from, &facts); err != success {
		GetLogger().WithError(err).Errorf("failed to extract connection fingerprint")
		return nil, err
	}
	return facts, success
}