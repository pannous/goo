// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "./a"

func main() {
	// Test that inlined type switches without short variable
	// declarations work correctly.
	checks(0, a.F(nil)) // ERROR "inlining call to a.F"
	checks(1, a.F(0))   // ERROR "inlining call to a.F" "does not escape"
	checks(2, a.F(0.0)) // ERROR "inlining call to a.F" "does not escape"
	checks(3, a.F(""))  // ERROR "inlining call to a.F" "does not escape"

	// Test that inlined type switches with short variable
	// declarations work correctly.
	_ = a.G(nil).(*any)                       // ERROR "inlining call to a.G"
	_ = a.G(1).(*int)                         // ERROR "inlining call to a.G" "does not escape"
	_ = a.G(2.0).(*float64)                   // ERROR "inlining call to a.G" "does not escape"
	_ = (*a.G("").(*any)).(string)            // ERROR "inlining call to a.G" "does not escape"
	_ = (*a.G(([]byte)(nil)).(*any)).([]byte) // ERROR "inlining call to a.G" "does not escape"
	_ = (*a.G(true).(*any)).(bool)            // ERROR "inlining call to a.G" "does not escape"
}

//go:noinline
func checks(want, got int) {
	if want != got {
		println("want", want, "but got", got)
	}
}
