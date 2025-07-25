// run

// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Issue 23546: type..eq function not generated when
// DWARF is disabled.

package main

func main() {
	use(f() == f())
}

func f() [2]any {
	var out [2]any
	return out
}

//go:noinline
func use(bool) {}
