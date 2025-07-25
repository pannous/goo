// errorcheck

// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package p

var _ = map[any]int{
	0: 0,
	0: 0, // ERROR "duplicate"
}

var _ = map[any]int{
	any(0): 0,
	any(0): 0, // ok
}

func _() {
	switch any(0) {
	case 0:
	case 0: // ERROR "duplicate"
	}

	switch any(0) {
	case any(0):
	case any(0): // ok
	}
}
