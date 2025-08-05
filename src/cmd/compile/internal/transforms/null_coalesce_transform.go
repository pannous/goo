// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// NullCoalesceTransform converts null coalescing expressions to conditional expressions
// Transforms expressions like a ?? b to func() any { if a == nil { return b }; return a }()
type NullCoalesceTransform struct{}

type nullCoalesceVisitor struct {
	transform *NullCoalesceTransform
	ctx       *TransformContext
	changed   bool
}

func (t *NullCoalesceTransform) Name() string {
	return "null_coalesce_transform"
}

func (t *NullCoalesceTransform) Priority() int {
	return 75 // before lambda but after list methods
}

func (t *NullCoalesceTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	// TODO: NullCoalesce operator not defined in this Go version - transform disabled
	return false
}

// Visit implements syntax.Visitor interface - DISABLED
func (v *nullCoalesceVisitor) Visit(node syntax.Node) syntax.Visitor {
	// TODO: NullCoalesce operator not defined in this Go version
	return nil
}

// All other methods disabled due to missing NullCoalesce operator

func init() {
	RegisterTransformer(&NullCoalesceTransform{})
}