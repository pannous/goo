// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build transforms

// danger, conflict with src of vendor packages

package transforms

import (
	"cmd/compile/internal/syntax"
)

// AsCastTransform converts as cast expressions to type assertions
// Transforms: x as T -> x.(T)
type AsCastTransform struct{}

// asCastVisitor implements the visitor pattern for as-cast transformation
type asCastVisitor struct {
	transform *AsCastTransform
	ctx       *TransformContext
	changed   bool
}

func (t *AsCastTransform) Name() string {
	return "as_cast_transform"
}

func (t *AsCastTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &asCastVisitor{transform: t, ctx: ctx}
	
	// Use the general visitor pattern to walk all nodes
	syntax.Walk(file, visitor)
	
	return visitor.changed
}

// Visit implements syntax.Visitor
func (v *asCastVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}
	
	// Look for nodes that contain as-cast expressions we can replace
	switch n := node.(type) {
	case *syntax.ExprStmt:
		if asCast, ok := n.X.(*syntax.AsCastExpr); ok {
			if newExpr := v.transform.convertAsCastToAssert(asCast, v.ctx); newExpr != asCast {
				n.X = newExpr
				v.changed = true
			}
		}
	case *syntax.AssignStmt:
		if asCast, ok := n.Rhs.(*syntax.AsCastExpr); ok {
			if newExpr := v.transform.convertAsCastToAssert(asCast, v.ctx); newExpr != asCast {
				n.Rhs = newExpr
				v.changed = true
			}
		}
	case *syntax.VarDecl:
		if n.Values != nil {
			if asCast, ok := n.Values.(*syntax.AsCastExpr); ok {
				if newExpr := v.transform.convertAsCastToAssert(asCast, v.ctx); newExpr != asCast {
					n.Values = newExpr
					v.changed = true
				}
			}
		}
	case *syntax.CallExpr:
		// Handle as-cast expressions in function arguments
		for i, arg := range n.ArgList {
			if asCast, ok := arg.(*syntax.AsCastExpr); ok {
				if newExpr := v.transform.convertAsCastToAssert(asCast, v.ctx); newExpr != asCast {
					n.ArgList[i] = newExpr
					v.changed = true
				}
			}
		}
	case *syntax.ReturnStmt:
		if n.Results != nil {
			if asCast, ok := n.Results.(*syntax.AsCastExpr); ok {
				if newExpr := v.transform.convertAsCastToAssert(asCast, v.ctx); newExpr != asCast {
					n.Results = newExpr
					v.changed = true
				}
			}
		}
	case *syntax.Operation:
		if asCast, ok := n.X.(*syntax.AsCastExpr); ok {
			if newExpr := v.transform.convertAsCastToAssert(asCast, v.ctx); newExpr != asCast {
				n.X = newExpr
				v.changed = true
			}
		}
		if asCast, ok := n.Y.(*syntax.AsCastExpr); ok {
			if newExpr := v.transform.convertAsCastToAssert(asCast, v.ctx); newExpr != asCast {
				n.Y = newExpr
				v.changed = true
			}
		}
	}
	
	// Continue visiting child nodes
	return v
}

func (t *AsCastTransform) convertAsCastToAssert(asCast *syntax.AsCastExpr, ctx *TransformContext) syntax.Expr {
	// Check if this is a no-op cast (same type)
	// For now, we'll implement a simple optimization for obvious cases
	// TODO: Add more sophisticated type checking
	
	// If casting to the same type name, just return the original expression
	if t.isSameType(asCast.X, asCast.Type, ctx) {
		return asCast.X
	}
	
	// Determine if we need type assertion or type conversion
	if t.shouldUseTypeConversion(asCast.X, asCast.Type, ctx) {
		// Create type conversion: T(x)
		callExpr := &syntax.CallExpr{
			Fun:     asCast.Type,
			ArgList: []syntax.Expr{asCast.X},
		}
		callExpr.SetPos(asCast.Pos())
		return callExpr
	} else {
		// Create type assertion: x.(T)
		assertExpr := &syntax.AssertExpr{
			X:    asCast.X,
			Type: asCast.Type,
		}
		assertExpr.SetPos(asCast.Pos())
		return assertExpr
	}
}

// isSameType checks if the expression and type are the same
// This is a simple heuristic - in a full implementation, we'd need proper type checking
func (t *AsCastTransform) isSameType(expr syntax.Expr, targetType syntax.Expr, ctx *TransformContext) bool {
	// Look up the variable's declared type in the context if available
	if exprName, ok := expr.(*syntax.Name); ok {
		if typeName, ok := targetType.(*syntax.Name); ok {
			// Check if we have type information for this variable
			if declaredType, exists := ctx.Types[exprName.Value]; exists {
				// If the declared type matches the target type, it's a no-op
				return declaredType == typeName.Value
			}
		}
	}
	
	return false
}

// shouldUseTypeConversion determines if we should use type conversion T(x) vs type assertion x.(T)
// Type conversion is used when converting between concrete types (like float64 to int)
// Type assertion is used when extracting a concrete type from an interface
func (t *AsCastTransform) shouldUseTypeConversion(expr syntax.Expr, targetType syntax.Expr, ctx *TransformContext) bool {
	// Look up the variable's declared type in the context if available
	if exprName, ok := expr.(*syntax.Name); ok {
		if typeName, ok := targetType.(*syntax.Name); ok {
			// Check if we have type information for this variable
			if declaredType, exists := ctx.Types[exprName.Value]; exists {
				// If the declared type is concrete (not interface), use conversion
				// Common concrete types that can be converted between
				concreteTypes := map[string]bool{
					"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
					"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
					"float32": true, "float64": true,
					"byte": true, "rune": true,
					"string": true,
				}
				
				// If both source and target are concrete types, use conversion
				if concreteTypes[declaredType] && concreteTypes[typeName.Value] {
					return true
				}
				
				// If source is interface{} or any, use assertion
				if declaredType == "interface{}" || declaredType == "any" {
					return false
				}
			}
		}
	}
	
	// Default to type assertion for safety
	return false
}

func init() {
	RegisterTransformer(&AsCastTransform{})
}
