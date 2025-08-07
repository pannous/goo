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

// NodeTransformer interface implementation
func (t *EmptyListTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
	// Check if this is a CompositeLit that could be an empty list
	if comp, ok := node.(*syntax.CompositeLit); ok {
		// Empty list: no type specified and no elements
		return comp.Type == nil && len(comp.ElemList) == 0
	}
	return false
}

func (t *EmptyListTransform) TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node {
	if comp, ok := node.(*syntax.CompositeLit); ok {
		// Transform [] to []any{}
		
		// Create slice type []any
		sliceType := &syntax.SliceType{}
		sliceType.SetPos(comp.Pos())
		
		// Create 'any' type name
		anyType := &syntax.Name{Value: "any"}
		anyType.SetPos(comp.Pos())
		sliceType.Elem = anyType
		
		// Create new composite literal with []any type
		newComp := &syntax.CompositeLit{
			Type:     sliceType,
			ElemList: comp.ElemList, // Should be empty
		}
		newComp.SetPos(comp.Pos())
		
		return newComp
	}
	return nil
}

func (t *EmptyListTransform) PostProcess(file *syntax.File, ctx *TransformContext) bool {
	// No post-processing needed for empty list transform
	return false
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