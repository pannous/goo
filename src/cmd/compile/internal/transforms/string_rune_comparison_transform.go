// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// StringRuneComparisonTransform handles comparison between strings and runes
// Transforms "你" == '你' into "你" == string('你')
type StringRuneComparisonTransform struct {
	ctx *TransformContext
}

func (t *StringRuneComparisonTransform) Name() string {
	return "string_rune_comparison_transform"
}

func (t *StringRuneComparisonTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	t.ctx = ctx // Store context for use in other methods
	visitor := &stringRuneVisitor{transform: t, ctx: ctx}
	syntax.Walk(file, visitor)
	return visitor.changed
}


// transformStringRuneComparison handles string vs rune comparisons
func (t *StringRuneComparisonTransform) transformStringRuneComparison(left, right syntax.Expr, op syntax.Operator) syntax.Expr {
	leftIsString := t.isStringType(left)
	rightIsString := t.isStringType(right)
	leftIsRune := t.isRuneType(left)
	rightIsRune := t.isRuneType(right)

	var transformedExpr syntax.Expr

	// Transform when we have string vs rune comparison (literals or variables)
	if leftIsString && rightIsRune {
		// string_var == rune_var -> string_var == string(rune_var)
		stringConversion := t.createStringConversion(right)
		transformedExpr = &syntax.Operation{
			Op: op,
			X:  left,
			Y:  stringConversion,
		}
	} else if leftIsRune && rightIsString {
		// rune_var == string_var -> string(rune_var) == string_var
		stringConversion := t.createStringConversion(left)
		transformedExpr = &syntax.Operation{
			Op: op,
			X:  stringConversion,
			Y:  right,
		}
	}

	if transformedExpr != nil {
		transformedExpr.SetPos(left.Pos())
		return transformedExpr
	}

	return nil
}

// isStringType checks if an expression is of string type (literal or variable)
func (t *StringRuneComparisonTransform) isStringType(expr syntax.Expr) bool {
	// Check for string literal
	if lit, ok := expr.(*syntax.BasicLit); ok {
		return lit.Kind == syntax.StringLit
	}
	// Check for variable with string type
	if name, ok := expr.(*syntax.Name); ok && t.ctx != nil && t.ctx.Types != nil {
		varType := t.ctx.Types[name.Value]
		return varType == "string"
	}
	return false
}

// isRuneType checks if an expression is of rune type (literal or variable)
func (t *StringRuneComparisonTransform) isRuneType(expr syntax.Expr) bool {
	// Check for rune literal
	if lit, ok := expr.(*syntax.BasicLit); ok {
		return lit.Kind == syntax.RuneLit
	}
	// Check for variable with rune type
	if name, ok := expr.(*syntax.Name); ok && t.ctx != nil && t.ctx.Types != nil {
		varType := t.ctx.Types[name.Value]
		return varType == "rune" || varType == "int32" // rune is alias for int32
	}
	return false
}

// isStringLiteral checks if an expression is a string literal
func (t *StringRuneComparisonTransform) isStringLiteral(expr syntax.Expr) bool {
	if lit, ok := expr.(*syntax.BasicLit); ok {
		return lit.Kind == syntax.StringLit
	}
	return false
}

// isRuneLiteral checks if an expression is a rune literal
func (t *StringRuneComparisonTransform) isRuneLiteral(expr syntax.Expr) bool {
	if lit, ok := expr.(*syntax.BasicLit); ok {
		return lit.Kind == syntax.RuneLit
	}
	return false
}

// createStringConversion creates string(rune) conversion
func (t *StringRuneComparisonTransform) createStringConversion(runeExpr syntax.Expr) syntax.Expr {
	pos := runeExpr.Pos()

	stringName := &syntax.Name{Value: "string"}
	stringName.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     stringName,
		ArgList: []syntax.Expr{runeExpr},
	}
	call.SetPos(pos)

	return call
}

type stringRuneVisitor struct {
	transform *StringRuneComparisonTransform
	ctx       *TransformContext
	changed   bool
}

func (v *stringRuneVisitor) Visit(node syntax.Node) syntax.Visitor {
	switch n := node.(type) {
	case *syntax.Operation:
		// Check for equality/inequality operations
		if n.Op == syntax.Eql || n.Op == syntax.Neq {
			// Check if we have string vs rune comparison
			transformed := v.transform.transformStringRuneComparison(n.X, n.Y, n.Op)
			if transformed != nil {
				// Replace the operation in place
				*n = *transformed.(*syntax.Operation)
				v.changed = true
			}
		}
	}
	return v
}

func init() {
	RegisterTransformer(&StringRuneComparisonTransform{})
}
