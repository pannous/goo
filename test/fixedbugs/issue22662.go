// run

// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Verify effect of various line directives.
// TODO: check columns

package main

import (
	"fmt"
	"runtime"
)

func checks(file string, line int) {
	_, f, l, ok := runtime.Caller(1)
	if !ok {
		panic("runtime.Caller(1) failed")
	}
	if f != file || l != line {
		panic(fmt.Sprintf("got %s:%d; want %s:%d", f, l, file, line))
	}
}

func main() {
//-style line directives
//line :1
	checks("??", 1) // no file specified
//line foo.go:1
	checks("foo.go", 1)
//line bar.go:10:20
	checks("bar.go", 10)
//line :11:22
	checks("bar.go", 11) // no file, but column specified => keep old filename

/*-style line directives */
/*line :1*/ checks("??", 1) // no file specified
/*line foo.go:1*/ checks("foo.go", 1)
/*line bar.go:10:20*/ checks("bar.go", 10)
/*line :11:22*/ checks("bar.go", 11) // no file, but column specified => keep old filename

	/*line :10*/ checks("??", 10); /*line foo.go:20*/ checks("foo.go", 20); /*line :30:1*/ checks("foo.go", 30)
	checks("foo.go", 31)
}
