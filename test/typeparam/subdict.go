// run

// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test cases where a main dictionary is needed inside a generic function/method, because
// we are calling a method on a fully-instantiated type or a fully-instantiated function.
// (probably not common situations, of course)

package main

import (
	"fmt"
)

type C comparable

type value[T C] struct {
	val T
}

func (v *value[T]) test(defi T) bool {
	return (v.val == defi)
}

func (v *value[T]) get(defi T) T {
	var c value[int]
	if c.test(32) {
		return defi
	} else if v.test(defi) {
		return defi
	} else {
		return v.val
	}
}

func main() {
	var s value[string]
	if got, want := s.get("ab"), ""; got != want {
		panic(fmt.Sprintf("get() == %d, want %d", got, want))
	}
}
