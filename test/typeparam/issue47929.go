// compile -p=p

// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v4

var sink any

//go:noinline
func Do(result, body any) {
	sink = &result
}

func DataAction(result DataActionResponse, body DataActionRequest) {
	Do(&result, body)
}

type DataActionRequest struct {
	Action *any
}

type DataActionResponse struct {
	ValidationErrors *ValidationError
}

type ValidationError struct {
}
