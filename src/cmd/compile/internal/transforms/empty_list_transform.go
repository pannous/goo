// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// EmptyListTransform handles transformation of empty list literals [] to []any{}
type EmptyListTransform struct{}

func (t *EmptyListTransform) Name() string {
	return "empty_list_transform"
}

func (t *EmptyListTransform) Priority() int {
	return 100 // Default priority - between list methods (50) and lambda (200)
}

func (t *EmptyListTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &emptyListVisitor{transform: t, ctx: ctx}
	syntax.Walk(file, visitor)
	return visitor.changed
}

type emptyListVisitor struct {
	transform *EmptyListTransform
	ctx       *TransformContext
	changed   bool
}

func (v *emptyListVisitor) Visit(node syntax.Node) syntax.Visitor {
	switch n := node.(type) {
	case *syntax.CompositeLit:
		// Check if this is an empty list literal [] (no type, no elements)
		if n.Type == nil && len(n.ElemList) == 0 {
			// Transform [] to []any{}
			
			// Create slice type []any
			sliceType := &syntax.SliceType{}
			sliceType.SetPos(n.Pos())
			
			// Create 'any' type name
			anyType := &syntax.Name{Value: "any"}
			anyType.SetPos(n.Pos())
			sliceType.Elem = anyType
			
			// Set the type on the composite literal
			n.Type = sliceType
			v.changed = true
		}
	}
	return v
}

func init() {
	RegisterTransformer(&EmptyListTransform{})
}