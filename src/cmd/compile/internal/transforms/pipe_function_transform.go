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

func (t *PipeFunctionTransform) Name() string {
	return "pipe_function_transform"
}

func (t *PipeFunctionTransform) Priority() int {
	return 75 // before lambda but after list methods
}

// NodeTransformer interface implementation
func (t *PipeFunctionTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
	// Only handle Operation nodes with pipe operator (Or)
	if op, ok := node.(*syntax.Operation); ok {
		return op.Op == syntax.Or
	}
	return false
}

func (t *PipeFunctionTransform) TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node {
	if op, ok := node.(*syntax.Operation); ok && op.Op == syntax.Or {
		return t.transformPipeOperation(op, ctx)
	}
	return nil
}

func (t *PipeFunctionTransform) PostProcess(file *syntax.File, ctx *TransformContext) bool {
	// No post-processing needed for pipe function transform
	return false
}

// Legacy Transform method for backward compatibility - not used in new architecture
func (t *PipeFunctionTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	// This method is kept for interface compatibility but not used
	// The new NodeTransformer interface methods are used instead
	return false
}

// transformPipeOperation transforms a pipe operation (value | function) to a function call
func (t *PipeFunctionTransform) transformPipeOperation(op *syntax.Operation, ctx *TransformContext) syntax.Expr {
	// Pipe operation: left | right -> right(left)
	left := op.X   // The value being piped
	right := op.Y  // The function to apply

	pos := op.Pos()
	
	// Create function call: right(left)
	callExpr := &syntax.CallExpr{
		Fun:     right,
		ArgList: []syntax.Expr{left},
	}
	callExpr.SetPos(pos)
	
	return callExpr
}

func init() {
	RegisterTransformer(&PipeFunctionTransform{})
}