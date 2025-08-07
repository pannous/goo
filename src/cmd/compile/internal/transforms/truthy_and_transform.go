// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// TruthyAndTransform handles Python-style truthy 'and' operator.
// It transforms expressions like:
// user and user.Name --> func() any { if isTruthy(user) { return user.Name }; return user }()
// x and y --> func() any { if isTruthy(x) { return y }; return x }()

type TruthyAndTransform struct{}

func (t *TruthyAndTransform) Name() string {
	return "truthy_and_transform"
}

func (t *TruthyAndTransform) Priority() int {
	return 10 // Run very early, before most other transforms
}

// NodeTransformer interface implementation
func (t *TruthyAndTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
	// Only handle Operation nodes with TruthyAnd operator
	if op, ok := node.(*syntax.Operation); ok {
		return op.Op == syntax.TruthyAnd
	}
	return false
}

func (t *TruthyAndTransform) TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node {
	if op, ok := node.(*syntax.Operation); ok && op.Op == syntax.TruthyAnd {
		return t.createTruthyAndCall(op.X, op.Y, op.Pos())
	}
	return nil
}

func (t *TruthyAndTransform) PostProcess(file *syntax.File, ctx *TransformContext) bool {
	// No post-processing needed for truthy and transform
	return false
}

// Legacy Transform method for backward compatibility - not used in new architecture
func (t *TruthyAndTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	// This method is kept for interface compatibility but not used
	// The new NodeTransformer interface methods are used instead
	return false
}

// createTruthyAndCall creates a truthy and operation using truthy calls with &&
func (t *TruthyAndTransform) createTruthyAndCall(left, right syntax.Expr, pos syntax.Pos) syntax.Expr {
	// Create truthy(left) && truthy(right)
	
	truthyName1 := &syntax.Name{Value: "truthy"}
	truthyName1.SetPos(pos)
	
	leftTruthyCall := &syntax.CallExpr{
		Fun:     truthyName1,
		ArgList: []syntax.Expr{left},
	}
	leftTruthyCall.SetPos(pos)
	
	truthyName2 := &syntax.Name{Value: "truthy"}
	truthyName2.SetPos(pos)
	
	rightTruthyCall := &syntax.CallExpr{
		Fun:     truthyName2,
		ArgList: []syntax.Expr{right},
	}
	rightTruthyCall.SetPos(pos)
	
	// Create truthy(left) && truthy(right)
	andOp := &syntax.Operation{
		Op: syntax.AndAnd,
		X:  leftTruthyCall,
		Y:  rightTruthyCall,
	}
	andOp.SetPos(pos)
	
	return andOp
}

func init() {
	RegisterTransformer(&TruthyAndTransform{})
}