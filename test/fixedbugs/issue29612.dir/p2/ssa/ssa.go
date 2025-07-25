// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

type T struct{}

func (T) foo() {}

type fooer interface {
	foo()
}

func Works(v any) {
	switch v.(type) {
	case any:
		v.(fooer).foo()
	}
}

func Panics(v any) {
	switch v.(type) {
	case any:
		v.(fooer).foo()
		v.(interface{ foo() }).foo()
	}
}
