// run

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test simple type switches on basic types.

package main

import "fmt"

const (
	a = iota
	b
	c
	d
	e
)

var x = []int{1, 2, 3}

func f(x int, len *byte) {
	*len = byte(x)
}

func whatis(x interface{}) string {
	switch xx := x.(type) {
	default:
		return fmt.Sprint("default ", xx)
	case int, int8, int16, int32:
		return fmt.Sprint("signed ", xx)
	case int64:
		return fmt.Sprint("signed64 ", int64(xx))
	case uint, uint8, uint16, uint32:
		return fmt.Sprint("unsigned ", xx)
	case uint64:
		return fmt.Sprint("unsigned64 ", uint64(xx))
	case nil:
		return fmt.Sprint("nil ", xx)
	}
	panic("not reached")
}

func whatis1(x interface{}) string {
	xx := x
	switch xx.(type) {
	default:
		return fmt.Sprint("default ", xx)
	case int, int8, int16, int32:
		return fmt.Sprint("signed ", xx)
	case int64:
		return fmt.Sprint("signed64 ", xx.(int64))
	case uint, uint8, uint16, uint32:
		return fmt.Sprint("unsigned ", xx)
	case uint64:
		return fmt.Sprint("unsigned64 ", xx.(uint64))
	case nil:
		return fmt.Sprint("nil ", xx)
	}
	panic("not reached")
}

func checks(x interface{}, s string) {
	w := whatis(x)
	if w != s {
		fmt.Println("whatis", x, "=>", w, "!=", s)
		panic("fail")
	}

	w = whatis1(x)
	if w != s {
		fmt.Println("whatis1", x, "=>", w, "!=", s)
		panic("fail")
	}
}

func main() {
	checks(1, "signed 1")
	checks(uint(1), "unsigned 1")
	checks(int64(1), "signed64 1")
	checks(uint64(1), "unsigned64 1")
	checks(1.5, "default 1.5")
	checks(nil, "nil <nil>")
}
