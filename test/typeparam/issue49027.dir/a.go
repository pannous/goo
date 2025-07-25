// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package a

func Conv(v any) string {
	return conv[string](v)
}

func conv[T any](v any) T {
	return v.(T)
}

func Conv2(v any) (string, bool) {
	return conv2[string](v)
}

func conv2[T any](v any) (T, bool) {
	x, ok := v.(T)
	return x, ok
}

func Conv3(v any) string {
	return conv3[string](v)
}

func conv3[T any](v any) T {
	switch v := v.(type) {
	case T:
		return v
	default:
		var z T
		return z
	}
}

type Mystring string

func (Mystring) Foo() {
}

func Conv4(v interface{ Foo() }) Mystring {
	return conv4[Mystring](v)
}

func conv4[T interface{ Foo() }](v interface{ Foo() }) T {
	switch v := v.(type) {
	case T:
		return v
	default:
		var z T
		return z
	}
}
