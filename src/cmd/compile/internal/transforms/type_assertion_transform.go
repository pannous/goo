// Copyright 2025 The Goo Authors. All rights reserved.

//go:build DONT_USE_transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// TypeAssertionTransform handles transformation of "x is Type" expressions
// to Go type assertion syntax "_, ok := x.(Type); ok"
type TypeAssertionTransform struct{}

type typeAssertionVisitor struct {
	transform *TypeAssertionTransform
	ctx       *TransformContext
	changed   bool
}

func (t *TypeAssertionTransform) Name() string {
	return "type_assertion_transform"
}

func (t *TypeAssertionTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	println("TypeAssertionTransform.Transform called")

	visitor := &typeAssertionVisitor{transform: t, ctx: ctx}
	syntax.Walk(file, visitor)

	return visitor.changed
}

// Visit implements syntax.Visitor interface
func (v *typeAssertionVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}

	// Look for binary operations with "is" / "is?" / "isa" operator
	if op, ok := node.(*syntax.Operation); ok {
		if op.Op == syntax.IS {
			println("Found 'is' binary operation")
			if callExpr := v.transform.transformIsOperation(op); callExpr != nil {
				println("TRANSFORMING 'isa' operation")
				// Transform to: instanceOf(x, Type) == true
				op.Op = syntax.Eql
				op.X = callExpr
				op.Y = &syntax.Name{Value: "true"}
				v.changed = true
			}
		}
	}
	return v
}

// transformIsOperation converts "x is Type" to instanceOf() runtime function call
func (t *TypeAssertionTransform) transformIsOperation(op *syntax.Operation) *syntax.CallExpr {
	variable := op.X
	typeExpr := op.Y

	// Create a call to the global instanceOf() runtime function that returns bool
	return &syntax.CallExpr{
		Fun:     &syntax.Name{Value: "instanceOf"},
		ArgList: []syntax.Expr{variable, typeExpr},
	}
}

func init() {
	//RegisterTransformer(&TypeAssertionTransform{})
}
