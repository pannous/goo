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

// NodeTransformer interface implementation
func (t *NullCoalesceTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
	// Only handle null coalescing operations directly
	if op, ok := node.(*syntax.Operation); ok {
		return op.Op == syntax.NullCoalesce
	}
	return false
}

func (t *NullCoalesceTransform) TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node {
	if op, ok := node.(*syntax.Operation); ok && op.Op == syntax.NullCoalesce {
		return t.createNullCoalesceCall(op)
	}
	return nil
}

func (t *NullCoalesceTransform) PostProcess(file *syntax.File, ctx *TransformContext) bool {
	// No post-processing needed for null coalesce transform
	return false
}

func (t *NullCoalesceTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &nullCoalesceVisitor{
		transform: t,
		ctx:       ctx,
	}
	
	syntax.Walk(file, visitor)
	return visitor.changed
}

// Visit implements syntax.Visitor interface
func (v *nullCoalesceVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}
	
	// Look for contexts where we can replace null coalescing operations
	switch n := node.(type) {
	case *syntax.ExprStmt:
		if op, ok := n.X.(*syntax.Operation); ok && op.Op == syntax.NullCoalesce {
			n.X = v.transform.createNullCoalesceCall(op)
			v.changed = true
		}
	case *syntax.AssignStmt:
		if op, ok := n.Rhs.(*syntax.Operation); ok && op.Op == syntax.NullCoalesce {
			n.Rhs = v.transform.createNullCoalesceCall(op)
			v.changed = true
		}
		// Handle multiple assignment values
		if rhs, ok := n.Rhs.(*syntax.ListExpr); ok {
			for i, expr := range rhs.ElemList {
				if op, ok := expr.(*syntax.Operation); ok && op.Op == syntax.NullCoalesce {
					rhs.ElemList[i] = v.transform.createNullCoalesceCall(op)
					v.changed = true
				}
			}
		}
	case *syntax.VarDecl:
		if n.Values != nil {
			if op, ok := n.Values.(*syntax.Operation); ok && op.Op == syntax.NullCoalesce {
				n.Values = v.transform.createNullCoalesceCall(op)
				v.changed = true
			}
		}
	case *syntax.CallExpr:
		// Handle null coalescing in function arguments
		for i, arg := range n.ArgList {
			if op, ok := arg.(*syntax.Operation); ok && op.Op == syntax.NullCoalesce {
				n.ArgList[i] = v.transform.createNullCoalesceCall(op)
				v.changed = true
			}
		}
	case *syntax.Operation:
		// Handle nested operations (like in binary expressions)
		if n.Op == syntax.NullCoalesce {
			// This won't directly replace it, but we need to handle this
			// at the parent level. Mark that we found one.
		} else {
			// Check operands for null coalescing
			if op, ok := n.X.(*syntax.Operation); ok && op.Op == syntax.NullCoalesce {
				n.X = v.transform.createNullCoalesceCall(op)
				v.changed = true
			}
			if op, ok := n.Y.(*syntax.Operation); ok && op.Op == syntax.NullCoalesce {
				n.Y = v.transform.createNullCoalesceCall(op)
				v.changed = true
			}
		}
	}
	
	return v
}

// createNullCoalesceCall converts x ?? y to a simple call to helper function
func (t *NullCoalesceTransform) createNullCoalesceCall(op *syntax.Operation) syntax.Expr {
	pos := op.Pos()
	
	// For now, let's just create a simpler transformation
	// Convert x ?? y to: (x != nil ? x : y) using a helper call
	
	// We'll use a simpler approach: func() any { if x != nil { return x }; return y }()
	// But with proper position handling
	
	// Create a nil identifier
	nilIdent := &syntax.Name{Value: "nil"}
	if pos.IsKnown() {
		nilIdent.SetPos(pos)
	}
	
	// Create condition: x != nil
	condition := &syntax.Operation{
		Op: syntax.Neq,
		X:  op.X,
		Y:  nilIdent,
	}
	if pos.IsKnown() {
		condition.SetPos(pos)
	}
	
	// Create return x statement
	returnX := &syntax.ReturnStmt{
		Results: op.X,
	}
	if pos.IsKnown() {
		returnX.SetPos(pos)
	}
	
	// Create then block
	thenBlock := &syntax.BlockStmt{
		List: []syntax.Stmt{returnX},
	}
	if pos.IsKnown() {
		thenBlock.SetPos(pos)
	}
	
	// Create return y statement
	returnY := &syntax.ReturnStmt{
		Results: op.Y,
	}
	if pos.IsKnown() {
		returnY.SetPos(pos)
	}
	
	// Create if statement
	ifStmt := &syntax.IfStmt{
		Cond: condition,
		Then: thenBlock,
	}
	if pos.IsKnown() {
		ifStmt.SetPos(pos)
	}
	
	// Create function body
	funcBody := &syntax.BlockStmt{
		List: []syntax.Stmt{ifStmt, returnY},
	}
	if pos.IsKnown() {
		funcBody.SetPos(pos)
	}
	
	// Create function type
	anyType := &syntax.Name{Value: "any"}
	if pos.IsKnown() {
		anyType.SetPos(pos)
	}
	
	field := &syntax.Field{Type: anyType}
	if pos.IsKnown() {
		field.SetPos(pos)
	}
	
	funcType := &syntax.FuncType{
		ResultList: []*syntax.Field{field},
	}
	if pos.IsKnown() {
		funcType.SetPos(pos)
	}
	
	// Create function literal
	funcLit := &syntax.FuncLit{
		Type: funcType,
		Body: funcBody,
	}
	if pos.IsKnown() {
		funcLit.SetPos(pos)
	}
	
	// Create function call
	callExpr := &syntax.CallExpr{
		Fun: funcLit,
	}
	if pos.IsKnown() {
		callExpr.SetPos(pos)
	}
	
	return callExpr
}

func init() {
	RegisterTransformer(&NullCoalesceTransform{})
}