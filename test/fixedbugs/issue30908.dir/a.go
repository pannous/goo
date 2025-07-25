// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package a

import (
	"errors"
	"strings"
)

var G any

func Unmarshal(data []byte, o any) error {
	G = o
	v, ok := o.(*map[string]any)
	if !ok {
		return errors.New("eek")
	}
	vals := make(map[string]any)
	s := string(data)
	items := strings.Split(s, " ")
	var err error
	for _, item := range items {
		vals[item] = s
		if item == "error" {
			err = errors.New("ouch")
		}
	}
	*v = vals
	return err
}
