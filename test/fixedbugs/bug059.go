// run

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "os"

func P(a []string) string {
	s := "{"
	for i := 0; i < 2; i++ {
		if i > 0 {
			s += ","
		}
		s += `"` + a[i] + `"`
	}
	s += "}"
	return s
}

func main() {
	m := make(map[string][]string)
	ass := new([2]string)
	ass[0] = "0"
	ass[1] = "1"
	m["0"] = ass[0:]

	a := m["0"]
	a[0] = "x"
	m["0"][0] = "deleted"
	if m["0"][0] != "deleted" {
		os.Exit(1)
	}
}
