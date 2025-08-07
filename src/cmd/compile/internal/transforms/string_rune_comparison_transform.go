// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// StringRuneComparisonTransform handles comparison between strings and runes
// Transforms "你" == '你' into "你" == string('你')
type StringRuneComparisonTransform struct{}

func (t *StringRuneComparisonTransform) Name() string {
	return "string_rune_comparison_transform"
}

func (t *StringRuneComparisonTransform) Priority() int {
	return 100 // Default priority - between list methods (50) and lambda (200)
}

// NodeTransformer interface implementation
func (t *StringRuneComparisonTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
	// Only handle equality/inequality Operation nodes
	if op, ok := node.(*syntax.Operation); ok {
		if op.Op == syntax.Eql || op.Op == syntax.Neq {
			// Check if this is a string vs rune comparison
			return t.isStringRuneComparison(op.X, op.Y)
		}
	}
	return false
}

func (t *StringRuneComparisonTransform) TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node {
	if op, ok := node.(*syntax.Operation); ok {
		if op.Op == syntax.Eql || op.Op == syntax.Neq {
			if t.isStringRuneComparison(op.X, op.Y) {
				return t.transformStringRuneComparison(op.X, op.Y, op.Op, op.Pos())
			}
		}
	}
	return nil
}

func (t *StringRuneComparisonTransform) PostProcess(file *syntax.File, ctx *TransformContext) bool {
	// No post-processing needed for string rune comparison transform
	return false
}

// Legacy Transform method for backward compatibility - not used in new architecture
func (t *StringRuneComparisonTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	// This method is kept for interface compatibility but not used
	// The new NodeTransformer interface methods are used instead
	return false
}

func (t *StringRuneComparisonTransform) isStringRuneComparison(left, right syntax.Expr) bool {
	leftIsString := t.isStringType(left)
	rightIsString := t.isStringType(right)
	leftIsRune := t.isRuneType(left)
	rightIsRune := t.isRuneType(right)

	// Return true if we have string vs rune comparison (either direction)
	return (leftIsString && rightIsRune) || (leftIsRune && rightIsString)
}

// transformStringRuneComparison handles string vs rune comparisons
func (t *StringRuneComparisonTransform) transformStringRuneComparison(left, right syntax.Expr, op syntax.Operator, pos syntax.Pos) syntax.Expr {
	leftIsString := t.isStringType(left)
	rightIsString := t.isStringType(right)
	leftIsRune := t.isRuneType(left)
	rightIsRune := t.isRuneType(right)

	var transformedExpr syntax.Expr

	// Transform when we have string vs rune comparison (literals or variables)
	if leftIsString && rightIsRune {
		// string_var == rune_var -> string_var == string(rune_var)
		stringConversion := t.createStringConversion(right, pos)
		transformedExpr = &syntax.Operation{
			Op: op,
			X:  left,
			Y:  stringConversion,
		}
	} else if leftIsRune && rightIsString {
		// rune_var == string_var -> string(rune_var) == string_var
		stringConversion := t.createStringConversion(left, pos)
		transformedExpr = &syntax.Operation{
			Op: op,
			X:  stringConversion,
			Y:  right,
		}
	}

	if transformedExpr != nil {
		transformedExpr.SetPos(pos)
	}

	return transformedExpr
}

func (t *StringRuneComparisonTransform) isStringType(expr syntax.Expr) bool {
	// Check if it's a string literal
	if lit, ok := expr.(*syntax.BasicLit); ok {
		return lit.Kind == syntax.StringLit
	}
	return false
}

func (t *StringRuneComparisonTransform) isRuneType(expr syntax.Expr) bool {
	// Check if it's a rune literal
	if lit, ok := expr.(*syntax.BasicLit); ok {
		return lit.Kind == syntax.RuneLit
	}
	return false
}

func (t *StringRuneComparisonTransform) createStringConversion(runeExpr syntax.Expr, pos syntax.Pos) syntax.Expr {
	// Create string(runeExpr)
	stringName := &syntax.Name{Value: "string"}
	stringName.SetPos(pos)

	callExpr := &syntax.CallExpr{
		Fun:     stringName,
		ArgList: []syntax.Expr{runeExpr},
	}
	callExpr.SetPos(pos)

	return callExpr
}

func init() {
	RegisterTransformer(&StringRuneComparisonTransform{})
}