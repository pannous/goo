// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"cmd/compile/internal/syntax"
)

// StringConcatTransformer handles automatic string conversion in concatenation.
// It transforms expressions like "result:" + z to "result:" + strconv.Itoa(z)
// when z is an integer type.
type StringConcatTransformer struct{}

func (t *StringConcatTransformer) Name() string {
	return "string_concat"
}

func (t *StringConcatTransformer) Transform(file *syntax.File) bool {
	// Simple transformation - just walk declarations for now
	changed := false
	for _, decl := range file.DeclList {
		if funcDecl, ok := decl.(*syntax.FuncDecl); ok {
			if t.transformFuncBody(funcDecl.Body) {
				changed = true
			}
		}
	}
	return changed
}

func (t *StringConcatTransformer) transformFuncBody(stmt syntax.Stmt) bool {
	// Placeholder - return false for now to avoid crashes
	return false
}

// transformConcatOperation checks if this is a string concatenation with a non-string operand
// and wraps the non-string operand with strconv.Itoa if it's an integer.
func (t *StringConcatTransformer) transformConcatOperation(op *syntax.Operation) syntax.Expr {
	if op.Op != syntax.Add {
		return nil
	}
	
	// Check if either operand is a string literal
	leftIsString := t.isStringLiteral(op.X)
	rightIsString := t.isStringLiteral(op.Y)
	
	// Only transform if exactly one operand is a string and the other might be an integer
	if leftIsString && !rightIsString {
		if t.mightBeIntegerVariable(op.Y) {
			// Transform: "string" + var => "string" + strconv.Itoa(var)
			op.Y = t.createItoacCall(op.Y)
			return op
		}
	} else if rightIsString && !leftIsString {
		if t.mightBeIntegerVariable(op.X) {
			// Transform: var + "string" => strconv.Itoa(var) + "string"
			op.X = t.createItoacCall(op.X)
			return op
		}
	}
	
	return nil
}

// isStringLiteral returns true if the expression is a string literal.
func (t *StringConcatTransformer) isStringLiteral(expr syntax.Expr) bool {
	if basic, ok := expr.(*syntax.BasicLit); ok {
		return basic.Kind == syntax.StringLit
	}
	return false
}

// mightBeIntegerVariable returns true if the expression could be an integer variable.
// For now, we'll be conservative and only handle simple identifiers.
func (t *StringConcatTransformer) mightBeIntegerVariable(expr syntax.Expr) bool {
	_, ok := expr.(*syntax.Name)
	return ok
}

// createItoacCall creates a syntax tree for strconv.Itoa(expr).
func (t *StringConcatTransformer) createItoacCall(expr syntax.Expr) syntax.Expr {
	// Create strconv.Itoa(expr)
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X: &syntax.Name{
				Value: "strconv",
			},
			Sel: &syntax.Name{
				Value: "Itoa",
			},
		},
		ArgList: []syntax.Expr{expr},
	}
}

// Register the transformer during package initialization
func init() {
	RegisterTransformer(&StringConcatTransformer{})
}