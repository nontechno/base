// Copyright 2021 The NonTechno Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/user"
	"runtime"
	"strings"
	"time"
)

const (
	sizeOfInt     = 4
	prefixSize    = sizeOfInt + sizeOfInt
	idFinderPrint = 0

	currentVersion = "0.7.4"
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

func getIdFromFingerprint(finger Fingerprint) string {
	if data, err := json.Marshal(finger); err == success {
		h := sha1.New()
		h.Write(data)
		return hex.EncodeToString(h.Sum(nil))
	}
	return fmt.Sprintf("(pid:%v)", os.Getpid())
}

func assembleFingerprint() Stream {
	facts := make(Fingerprint)

	facts["pid"] = os.Getpid()
	facts["arch"] = runtime.GOARCH
	facts["os"] = runtime.GOOS
	facts["cpu"] = runtime.NumCPU()
	facts["version"] = runtime.Version()
	facts["app"] = os.Args[0]

	if len(os.Args) > 1 {
		facts["args"] = strings.Join(os.Args[1:], ";")
	}

	facts["vesion"] = currentVersion
	facts["config"] = GetValue("config.filename", "?")
	facts["comment"] = GetValue("config.comment", "?")

	if hostname, err := os.Hostname(); err == nil {
		facts["hostname"] = hostname
	}

	if usr, err := user.Current(); err == nil {
		facts["user"] = usr.Name
		facts["username"] = usr.Username
	}

	facts["id"] = getIdFromFingerprint(facts)

	if value := GetValue("name", ""); len(value) > 0 {
		facts["name"] = value
	}
	if value := GetValue("description", ""); len(value) > 0 {
		facts["description"] = value
	}

	// var mem runtime.MemStats
	// runtime.ReadMemStats(&mem)

	if data, err := json.Marshal(facts); err != success {
		return []byte{}
	} else {
		return data
	}
}

func sendFingerprint(conn net.Conn) error {
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
