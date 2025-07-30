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

func (t *MapDotTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &mapDotVisitor{ctx: ctx}
	
	// Use the general visitor pattern to walk all nodes
	syntax.Walk(file, visitor)
	
	return visitor.changed
}

// mapDotVisitor implements the visitor pattern for map dot notation transformation
type mapDotVisitor struct {
	ctx     *TransformContext
	changed bool
}

// Visit implements syntax.Visitor
func (v *mapDotVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}
	
	// Look for nodes that contain selector expressions we can replace
	switch n := node.(type) {
	case *syntax.ExprStmt:
		if sel, ok := n.X.(*syntax.SelectorExpr); ok {
			if newExpr := v.transformSelector(sel); newExpr != nil {
				n.X = newExpr
				v.changed = true
			}
		}
	case *syntax.AssignStmt:
		if sel, ok := n.Rhs.(*syntax.SelectorExpr); ok {
			if newExpr := v.transformSelector(sel); newExpr != nil {
				n.Rhs = newExpr
				v.changed = true
			}
		}
	case *syntax.Operation:
		if sel, ok := n.X.(*syntax.SelectorExpr); ok {
			if newExpr := v.transformSelector(sel); newExpr != nil {
				n.X = newExpr
				v.changed = true
			}
		}
		if sel, ok := n.Y.(*syntax.SelectorExpr); ok {
			if newExpr := v.transformSelector(sel); newExpr != nil {
				n.Y = newExpr
				v.changed = true
			}
		}
	case *syntax.CallExpr:
		// Handle selectors in function arguments
		for i, arg := range n.ArgList {
			if sel, ok := arg.(*syntax.SelectorExpr); ok {
				if newExpr := v.transformSelector(sel); newExpr != nil {
					n.ArgList[i] = newExpr
					v.changed = true
				}
			}
		}
	case *syntax.CheckStmt:
		// Handle selectors in check conditions
		if sel, ok := n.Cond.(*syntax.SelectorExpr); ok {
			if newExpr := v.transformSelector(sel); newExpr != nil {
				n.Cond = newExpr
				v.changed = true
			}
		}
		// Also handle nested operations in check conditions
		if op, ok := n.Cond.(*syntax.Operation); ok {
			if sel, ok := op.X.(*syntax.SelectorExpr); ok {
				if newExpr := v.transformSelector(sel); newExpr != nil {
					op.X = newExpr
					v.changed = true
				}
			}
			if sel, ok := op.Y.(*syntax.SelectorExpr); ok {
				if newExpr := v.transformSelector(sel); newExpr != nil {
					op.Y = newExpr
					v.changed = true
				}
			}
		}
	}
	
	// Continue visiting child nodes
	return v
}

// transformSelector transforms a selector expression if it's a map access
func (v *mapDotVisitor) transformSelector(sel *syntax.SelectorExpr) syntax.Expr {
	// Check if the base expression is a variable we know is a map with string keys
	if name, ok := sel.X.(*syntax.Name); ok {
		varType, exists := v.ctx.Types[name.Value]
		if exists && strings.HasPrefix(varType, "map[string]") {
			// Transform m.key to m["key"]
			keyName := sel.Sel.Value
			stringLit := &syntax.BasicLit{
				Kind:  syntax.StringLit,
				Value: `"` + keyName + `"`,
			}
			indexExpr := &syntax.IndexExpr{
				X:     sel.X,
				Index: stringLit,
			}
			// Preserve position information
			indexExpr.SetPos(sel.Pos())
			return indexExpr
		}
	}
	return nil
}

func init() {
	RegisterTransformer(&MapDotTransform{})
}