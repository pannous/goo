// compile

// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Used to crash when a type switch was present in dead code
// in an inlineable function.

package p

func Then() {
	var i any
	if false {
		switch i.(type) {
		}
	}
}

func Else() {
	var i any
	if true {
		_ = i
	} else {
		switch i.(type) {
		}
	}
}

func Switch() {
	var i any
	switch 5 {
	case 3:
		switch i.(type) {
		}
	case 5:
	}
}
