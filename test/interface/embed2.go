// errorcheck

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test methods derived from embedded interface and *interface values.

package main

import "os"

const Value = 1e12

type Inter interface {
	M() int64
}

type T int64

func (t T) M() int64 { return int64(t) }

var t = T(Value)
var pt = &t
var ti Inter = t
var pti = &ti

type S struct{ Inter }

var s = S{ti}
var ps = &s

type SP struct{ *Inter } // ERROR "interface"

var i Inter
var pi = &i

var ok = true

func checks(s string, v int64) {
	if v != Value {
		println(s, v)
		ok = false
	}
}

func main() {
	checks("t.M()", t.M())
	checks("pt.M()", pt.M())
	checks("ti.M()", ti.M())
	checks("pti.M()", pti.M()) // ERROR "pointer to interface, not interface|no field or method M"
	checks("s.M()", s.M())
	checks("ps.M()", ps.M())

	i = t
	checks("i = t; i.M()", i.M())
	checks("i = t; pi.M()", pi.M()) // ERROR "pointer to interface, not interface|no field or method M"

	i = pt
	checks("i = pt; i.M()", i.M())
	checks("i = pt; pi.M()", pi.M()) // ERROR "pointer to interface, not interface|no field or method M"

	i = s
	checks("i = s; i.M()", i.M())
	checks("i = s; pi.M()", pi.M()) // ERROR "pointer to interface, not interface|no field or method M"

	i = ps
	checks("i = ps; i.M()", i.M())
	checks("i = ps; pi.M()", pi.M()) // ERROR "pointer to interface, not interface|no field or method M"

	if !ok {
		println("BUG: interface10")
		os.Exit(1)
	}
}
