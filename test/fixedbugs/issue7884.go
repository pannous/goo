// compile

// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "fmt"

func main() {
	var ii any = 5
	zz, err := ii.(any)
	fmt.Println(zz, err)
}
