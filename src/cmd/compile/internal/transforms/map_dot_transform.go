// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transforms

import (
	"cmd/compile/internal/syntax"
	"strings"
)

// MapDotTransform converts map dot notation (m.key) to index notation (m["key"])
// for maps with string keys.
type MapDotTransform struct{}

func (t *MapDotTransform) Name() string {
	return "map_dot_transform"
}

func (t *MapDotTransform) Priority() int {
	return 100 // Default priority - between list methods (50) and lambda (200)
}

// NodeTransformer interface implementation
func (t *MapDotTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
	// Only handle SelectorExpr nodes for map dot notation
	if sel, ok := node.(*syntax.SelectorExpr); ok {
		// Check if the base expression is a variable we know is a map with string keys
		if name, ok := sel.X.(*syntax.Name); ok {
			varType, exists := ctx.Types[name.Value]
			return exists && strings.HasPrefix(varType, "map[string]")
		}
	}
	return false
}

func (t *MapDotTransform) TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node {
	if sel, ok := node.(*syntax.SelectorExpr); ok {
		return t.transformSelector(sel, ctx)
	}
	return nil
}

func (t *MapDotTransform) PostProcess(file *syntax.File, ctx *TransformContext) bool {
	// No post-processing needed for map dot transform
	return false
}

// Legacy Transform method for backward compatibility - not used in new architecture
func (t *MapDotTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	// This method is kept for interface compatibility but not used
	// The new NodeTransformer interface methods are used instead
	return false
}

func (t *MapDotTransform) transformSelector(sel *syntax.SelectorExpr, ctx *TransformContext) syntax.Expr {
	// Check if the base expression is a variable we know is a map with string keys
	if name, ok := sel.X.(*syntax.Name); ok {
		varType, exists := ctx.Types[name.Value]
		if exists && strings.HasPrefix(varType, "map[string]") {
			// Transform m.key to m["key"]
			keyName := sel.Sel.Value
			stringLit := &syntax.BasicLit{
				Kind:  syntax.StringLit,
				Value: `"` + keyName + `"`,
			}
			stringLit.SetPos(sel.Sel.Pos())
			indexExpr := &syntax.IndexExpr{
				X:     sel.X,
				Index: stringLit,
			}
			indexExpr.SetPos(sel.Pos())
			return indexExpr
		}
	}
	return nil
}

func init() {
	RegisterTransformer(&MapDotTransform{})
}