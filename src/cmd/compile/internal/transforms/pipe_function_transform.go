// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// PipeFunctionTransform handles pipe operator with function calls.
// It transforms expressions like:
// 2 | square --> square(2)
// value | fn   --> fn(value)

type PipeFunctionTransform struct{}

// pipeFunctionVisitor implements syntax.Visitor to transform pipe operations
type pipeFunctionVisitor struct {
	transform *PipeFunctionTransform
	ctx       *TransformContext
	changed   bool
}

func (t *PipeFunctionTransform) Name() string {
	return "pipe_function_transform"
}

func (t *PipeFunctionTransform) Priority() int {
	return 75 // before lambda but after list methods
}

func (t *PipeFunctionTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &pipeFunctionVisitor{transform: t, ctx: ctx}
	syntax.Walk(file, visitor)
	return visitor.changed
}

// Visit implements syntax.Visitor interface
func (v *pipeFunctionVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}

	// Look for parent nodes that contain pipe operations we can replace
	switch n := node.(type) {
	case *syntax.ExprStmt:
		if op, ok := n.X.(*syntax.Operation); ok && op.Op == syntax.Or {
			if newExpr := v.transform.transformPipeOperation(op, v.ctx); newExpr != nil {
				n.X = newExpr
				v.changed = true
			}
		}
	case *syntax.AssignStmt:
		if op, ok := n.Rhs.(*syntax.Operation); ok && op.Op == syntax.Or {
			if newExpr := v.transform.transformPipeOperation(op, v.ctx); newExpr != nil {
				n.Rhs = newExpr
				v.changed = true
			}
		}
	case *syntax.VarDecl:
		if n.Values != nil {
			if op, ok := n.Values.(*syntax.Operation); ok && op.Op == syntax.Or {
				if newExpr := v.transform.transformPipeOperation(op, v.ctx); newExpr != nil {
					n.Values = newExpr
					v.changed = true
				}
			}
		}
	case *syntax.CallExpr:
		// Handle pipe operations in function arguments
		for i, arg := range n.ArgList {
			if op, ok := arg.(*syntax.Operation); ok && op.Op == syntax.Or {
				if newExpr := v.transform.transformPipeOperation(op, v.ctx); newExpr != nil {
					n.ArgList[i] = newExpr
					v.changed = true
				}
			}
		}
	case *syntax.ReturnStmt:
		if n.Results != nil {
			if op, ok := n.Results.(*syntax.Operation); ok && op.Op == syntax.Or {
				if newExpr := v.transform.transformPipeOperation(op, v.ctx); newExpr != nil {
					n.Results = newExpr
					v.changed = true
				}
			}
		}
	case *syntax.Operation:
		// Handle nested operations: check both X and Y operands
		if opX, ok := n.X.(*syntax.Operation); ok && opX.Op == syntax.Or {
			if newExpr := v.transform.transformPipeOperation(opX, v.ctx); newExpr != nil {
				n.X = newExpr
				v.changed = true
			}
		}
		if opY, ok := n.Y.(*syntax.Operation); ok && opY.Op == syntax.Or {
			if newExpr := v.transform.transformPipeOperation(opY, v.ctx); newExpr != nil {
				n.Y = newExpr
				v.changed = true
			}
		}
	case *syntax.ParenExpr:
		// Handle pipe operations inside parentheses
		if op, ok := n.X.(*syntax.Operation); ok && op.Op == syntax.Or {
			if newExpr := v.transform.transformPipeOperation(op, v.ctx); newExpr != nil {
				n.X = newExpr
				v.changed = true
			}
		}
	}

	return v // Continue walking
}

// transformPipeOperation checks if this is a pipe operation with a function
// and transforms it to a function call
func (t *PipeFunctionTransform) transformPipeOperation(op *syntax.Operation, ctx *TransformContext) syntax.Expr {
	if op.Op != syntax.Or {
		return nil
	}

	// Check if the right-hand side is a function name (not a function call)
	if name, ok := op.Y.(*syntax.Name); ok {
		// This looks like: expr | functionName
		// Transform to: functionName(expr)
		call := &syntax.CallExpr{
			Fun:     name,
			ArgList: []syntax.Expr{op.X},
		}
		call.SetPos(op.Pos())
		return call
	}

	return nil
}

func init() {
	RegisterTransformer(&PipeFunctionTransform{})
}
