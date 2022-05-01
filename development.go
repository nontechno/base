// Copyright 2022 The NonTechno Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"io/ioutil"
	"os"
	"strings"

	ps "github.com/mitchellh/go-ps"
)

func IsUnderDevelopment() bool {
	return checkProcess(12, os.Getpid())
}

func checkProcess(depth int, pid int) bool {

	if depth <= 0 {
		return false
	}

	if p, err := ps.FindProcess(pid); err != nil {
		// fmt.Println("Error : ", err)
		return false
	} else if p == nil {
		return false
	} else if knownEntry(p.Executable()) {
		return true
	} else {
		return checkProcess(depth-1, p.PPid())
	}
}

// this func loads settings from an external file and applies them to the process's environment
func LoadEnvFile(name string) {
	data, err := ioutil.ReadFile(name)
	if err != nil {
		return
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.Trim(line, " \t\r")

		for _, prefix := range []string{"export ", "env "} {
			line = strings.TrimPrefix(line, prefix)
		}

		if split := strings.Index(line, "="); split > 0 {
			key := line[:split]
			value := line[split+1:]
			value = strings.Trim(value, "\"")
			os.Setenv(key, value)
		}
	}
}
