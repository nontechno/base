// Copyright 2021 The NonTechno Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"strings"
)

const (
	star = "*"
)

func MatchString(source string, pattern string) (bool, []string) {
	var fillings []string

	if len(pattern) == 0 {
		return true, nil
	}

	if strings.Count(pattern, star) == 0 {
		return strings.Contains(source, pattern), nil
	}

	fragments := strings.Split(pattern, star)
	src := source
	allowPrefix := false
	for x, fragment := range fragments {
		if len(fragment) > 0 {
			if index := strings.Index(src, fragment); index >= 0 {
				if allowPrefix {
					before := src[:index]
					fillings = append(fillings, before)
				} else if index > 0 {
					return false, nil
				}

				after := src[index+len(fragment):]

				src = after
			} else {
				return false, nil
			}
		} else {
			allowPrefix = true
			if x == len(fragments)-1 {
				fillings = append(fillings, src)
			}
		}
	}
	return true, fillings
}
