// Copyright 2021 The NonTechno Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"sync"
)

const (
	Stdout  = 1
	Stderr  = 3
	Logger  = 7
	Metrics = 11
	User    = 100
)

var (
	pipesGuard                                      sync.Mutex
	pipesInitalized                                 bool
	pipeStdout, pipeStderr, pipeLogger, pipeMetrics io.Writer
)

func GetPipe(kind int) io.Writer {
	var pipe io.Writer
	switch kind {
	case Stdout:
		pipe = pipeStdout
	case Stderr:
		pipe = pipeStderr
	case Logger:
		pipe = pipeLogger
	case Metrics:
		pipe = pipeMetrics
	default:

	}
	return pipe
}

func SetPipe(kind int, writer io.Writer) {
	if writer != nil {
		pipesGuard.Lock()
		defer pipesGuard.Unlock()

		switch kind {
		case Stdout:
			pipeStdout = writer
		case Stderr:
			pipeStderr = writer
		case Logger:
			pipeLogger = writer
		case Metrics:
			pipeMetrics = writer
		default:

		}
	}
}

func Printf(format string, args ...interface{}) {
	if pipeStdout != nil {
		fmt.Fprintf(pipeStdout, format, args...)
	} else {
		_, _ = fmt.Printf(format, args...)
	}
}

func Errorf(format string, args ...interface{}) {
	if pipeStderr != nil {
		fmt.Fprintf(pipeStderr, format, args...)
	} else {
		_, _ = fmt.Printf(format, args...)
	}
}

func initPipes() {
	if pipesInitalized {
		return
	}

	pipesGuard.Lock()
	defer pipesGuard.Unlock()

	if pipesInitalized {
		return
	}

	ports := loadPorts()
	if len(ports) > 0 {
		mux := CreateMuxWriter(ports[0])

		pipeStdout, _ = mux.NewWriter(Stdout)
		pipeStderr, _ = mux.NewWriter(Stderr)
		pipeLogger, _ = mux.NewWriter(Logger)
		pipeMetrics, _ = mux.NewWriter(Metrics)

		pipesInitalized = true
		GetLogger().SetOutput(pipeLogger)
	}
	pipesInitalized = true
}

func loadPorts() []int {
	filename := GetValue("ports.filename", "port.json")
	if data, err := ioutil.ReadFile(filename); err == nil {
		var receiver []int
		if err := json.Unmarshal(data, &receiver); err == nil {
			return receiver
		} else {
			GetLogger().WithError(err).Errorf("failed to unmarshal []int")
		}
	} else {
		GetLogger().WithError(err).Errorf("failed to read file (%s)", filename)
	}
	return nil
}
