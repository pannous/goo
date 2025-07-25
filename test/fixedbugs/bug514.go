// run

// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build cgo

package main

import "runtime/cgo"

type iface interface {
	Get() int
}

type notInHeap struct {
	_ cgo.Incomplete
	i int
}

type myInt struct {
	f *notInHeap
}

func (mi myInt) Get() int {
	return int(mi.f.i)
}

type embed struct {
	*myInt
}

var val = 1234

var valNotInHeap = notInHeap{i: val}

func main() {
	i := val
	checks(i)
	mi := myInt{f: &valNotInHeap}
	checks(mi.Get())
	ifv := iface(mi)
	checks(ifv.Get())
	ifv = iface(&mi)
	checks(ifv.Get())
	em := embed{&mi}
	checks(em.Get())
	ifv = em
	checks(ifv.Get())
	ifv = &em
	checks(ifv.Get())
}

func checks(v int) {
	if v != val {
		panic(v)
	}
}
