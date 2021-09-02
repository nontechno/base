// Copyright 2021 The NonTechno Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"fmt"

	config "github.com/nontechno/config"
)

var (
	// aliases
	GetValue = config.GetValue
	GetFlag  = config.GetFlag
	sprintf  = fmt.Sprintf
)
