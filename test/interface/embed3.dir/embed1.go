// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "./embed0"

type X1 struct{}

func (X1) Foo() {}

type X2 struct{}

func (X2) foo() {}

type X3 struct{}

func (X3) foo(int) {}

type X4 struct{ p.M1 }

type X5 struct{ p.M1 }

func (X5) foo(int) {}

type X6 struct{ p.M2 }

type X7 struct{ p.M2 }

func (X7) foo() {}

type X8 struct{ p.M2 }

func (X8) foo(int) {}

func main() {
	var i1 any = X1{}
	checks(func() { _ = i1.(p.I1) }, "interface conversion: main.X1 is not p.I1: missing method Foo")

	var i2 any = X2{}
	checks(func() { _ = i2.(p.I2) }, "interface conversion: main.X2 is not p.I2: missing method foo")

	var i3 any = X3{}
	checks(func() { _ = i3.(p.I2) }, "interface conversion: main.X3 is not p.I2: missing method foo")

	var i4 any = X4{}
	checks(func() { _ = i4.(p.I2) }, "interface conversion: main.X4 is not p.I2: missing method foo")

	var i5 any = X5{}
	checks(func() { _ = i5.(p.I2) }, "interface conversion: main.X5 is not p.I2: missing method foo")

	var i6 any = X6{}
	checks(func() { _ = i6.(p.I2) }, "")

	var i7 any = X7{}
	checks(func() { _ = i7.(p.I2) }, "")

	var i8 any = X8{}
	checks(func() { _ = i8.(p.I2) }, "")
}

func checks(f func(), msg string) {
	defer func() {
		v := recover()
		if v == nil {
			if msg == "" {
				return
			}
			panic("did not panic")
		}
		got := v.(error).Error()
		if msg != got {
			panic("want '" + msg + "', got '" + got + "'")
		}
	}()
	f()
}
