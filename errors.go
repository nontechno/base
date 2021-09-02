// Copyright 2021 The NonTechno Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"errors"
	"fmt"
)

var (
	errNegativePortNumber = errors.New("port number cannot be negative")
	errStreamIsClosed     = errors.New("the stream is closed")
	errInternalUseId      = errors.New("this id is reserved for internal use")
	errParamIsNil         = errors.New("a required parameter is nil")
	errNotConnected       = errors.New("there is no connection...")

	errIncompleteData = errors.New("incomplete data")

	success error = nil
)

func warning(format string, a ...interface{}) {
	_, _ = fmt.Printf(format, a...)
}
