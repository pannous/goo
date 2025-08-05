// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// OptionalChainTransform converts optional chaining expressions to safe navigation
// Transforms expressions like user?.Profile to conditional expressions
type OptionalChainTransform struct{}

type optionalChainVisitor struct {
	transform *OptionalChainTransform
	ctx       *TransformContext
	changed   bool
}

func (t *OptionalChainTransform) Name() string {
	return "optional_chain_transform"
}

func (t *OptionalChainTransform) Priority() int {
	return 75 // before lambda but after list methods
}

func (t *OptionalChainTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	// TODO: OptionalChainExpr not defined in this Go version - transform disabled
	return false
}

// Visit implements syntax.Visitor interface - DISABLED
func (v *optionalChainVisitor) Visit(node syntax.Node) syntax.Visitor {
	// TODO: OptionalChainExpr not defined in this Go version
	return nil
}

// All other methods disabled due to missing OptionalChainExpr

func init() {
	RegisterTransformer(&OptionalChainTransform{})
}