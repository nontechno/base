// +build darwin freebsd netbsd openbsd

// Copyright 2022 The NonTechno Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

var known_entries = map[string]bool{
	"debugserver": true,
	"dlv":         true,
	"goland":      true,
}
