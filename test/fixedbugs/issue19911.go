// run

// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strings"
)

type ET struct{}

func (*ET) Error() string { return "err" }

func main() {
	checks("false", fmt.Sprintf("(*ET)(nil) == error(nil): %v", (*ET)(nil) == error(nil)))
	checks("true", fmt.Sprintf("(*ET)(nil) != error(nil): %v", (*ET)(nil) != error(nil)))

	nilET := (*ET)(nil)
	nilError := error(nil)

	checks("false", fmt.Sprintf("nilET == nilError: %v", nilET == nilError))
	checks("true", fmt.Sprintf("nilET != nilError: %v", nilET != nilError))
}

func checks(want, gotfull string) {
	got := gotfull[strings.Index(gotfull, ": ")+len(": "):]
	if got != want {
		panic("want " + want + " got " + got + " from " + gotfull)
	}
}
