// Copyright 2021 The NonTechno Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	hook "github.com/bin3377/logrus-datadog-hook"
	log "github.com/sirupsen/logrus"
)

var (
	formatter log.Formatter = &log.JSONFormatter{} // &log.JSONFormatter{TimestampFormat: time.RFC3339}
	logger    *log.Logger
	guard     sync.Mutex
)

func GetLogger() *log.Logger {
	if logger != nil {
		return logger
	}

	initPipes()

	guard.Lock()
	defer guard.Unlock()

	if logger != nil { // need this in case it was created on another thread, while we were locked ...
		return logger
	}

	host := GetValue("datadog.host", hook.DatadogUSHost) // "DATADOG_HOST"
	apiKey := GetValue("datadog.apikey", "")             // DATADOG_APIKEY

	l := log.New()
	l.SetFormatter(&log.JSONFormatter{})
	l.SetOutput(getLogOutput())
	l.SetLevel(getLogLevel())

	if len(apiKey) > 0 {
		hostName, _ := os.Hostname()

		tags := GetValue("log.tags", "tag1;tag2;tag3") // "LOG_TAGS"

		options := hook.Options{
			Source:   GetValue("log.source", "(your.source.here)"),   // "LOG_SOURCE"
			Service:  GetValue("log.service", "(your.service.here)"), // "LOG_SERVICE"
			Hostname: hostName,
			Tags:     strings.Split(tags, ";"),
		}

		ddhook := hook.NewHook(host, apiKey, 5*time.Second, 3, log.TraceLevel, formatter, options)
		l.Hooks.Add(ddhook)
	} else {
		l.Infof("not using datadog (missing api key)")
	}

	logger = l.WithFields(log.Fields{
		"service.name": GetValue("service.name", "preference.service"),
	}).Logger
	return logger
}

func getLogLevel() log.Level {
	logLevel := strings.ToUpper(GetValue("log.level", "TRACE")) // "LOG_LEVEL"
	switch logLevel {
	case "PANIC":
		return log.PanicLevel
	case "FATAL":
		return log.FatalLevel
	case "ERROR":
		return log.ErrorLevel
	case "WARN":
		return log.WarnLevel
	case "INFO":
		return log.InfoLevel
	case "DEBUG":
		return log.DebugLevel
	case "TRACE":
		return log.TraceLevel
	default:
		return log.TraceLevel
	}
}

// todo: handle more than one output. both "or" and "and" combinations
func getLogOutput() io.Writer {
	logOutput := GetValue("log.output", "stdout")
	logOutputLower := strings.ToLower(logOutput)
	switch logOutputLower {
	case "stdout", "":
		return os.Stdout
	case "stderr":
		return os.Stderr
	}

	const filePrefix = "file://"
	if strings.HasPrefix(logOutput, filePrefix) {
		fileName := logOutput[len(filePrefix):]
		f, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.WithError(err).Errorf("failed to open file (%s) to be used as log destination", fileName)
		} else {
			runtime.SetFinalizer(f, closeOpenFile) // this will close this open file on app exit
			return f
		}
	}

	const ipcPrefix = "ipc://"
	if strings.HasPrefix(logOutput, ipcPrefix) {
		addressPart := logOutput[len(ipcPrefix):]
		parts := strings.Split(addressPart, ":")
		portPart := parts[len(parts)-1]
		port, err := strconv.Atoi(portPart)
		if err != nil {
			log.WithError(err).Errorf("failed to extract port nimber from (%s)", logOutput)
		} else {
			return CreateIpcWriter(port)
		}
	}

	return os.Stdout
}

func closeOpenFile(file *os.File) {
	if file != nil {
		file.Close()
	}
}
