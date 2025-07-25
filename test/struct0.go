// run

// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test zero length structs.
// Used to not be evaluated.
// Issue 2232.

package main

func recv(c chan any) struct{} {
	return (<-c).(struct{})
}

var m = make(map[any]int)

func recv1(c chan any) {
	defer rec()
	m[(<-c).(struct{})] = 0
}

func rec() {
	recover()
}

func main() {
	c := make(chan any)
	go recv(c)
	c <- struct{}{}
	go recv1(c)
	c <- struct{}{}
}
