// run

// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

func main() {
	f[any](nil)
}

func f[T any](x T) {
	var _ any = x
}
