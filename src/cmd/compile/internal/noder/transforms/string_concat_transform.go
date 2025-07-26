// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transforms

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
	changed := false
	
	// Only look at function declarations for now
	for _, decl := range file.DeclList {
		if funcDecl, ok := decl.(*syntax.FuncDecl); ok && funcDecl.Body != nil {
			if t.transformBlock(funcDecl.Body) {
				changed = true
			}
		}
	}
	
	// If we made changes, ensure strconv import exists
	if changed {
		t.ensureStrconvImport(file)
	}
	
	return changed
}

// ensureStrconvImport adds strconv import if not already present
func (t *StringConcatTransformer) ensureStrconvImport(file *syntax.File) {
	// Check if strconv is already imported
	for _, decl := range file.DeclList {
		if importDecl, ok := decl.(*syntax.ImportDecl); ok {
			if importDecl.Path != nil && importDecl.Path.Value == `"strconv"` {
				return // Already imported
			}
		}
	}
	
	// Add strconv import at the beginning
	strconvImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: `"strconv"`,
			Kind:  syntax.StringLit,
		},
	}
	
	// Simply append the import at the end for now - Go will sort them during compilation
	file.DeclList = append(file.DeclList, strconvImport)
}

// transformBlock safely transforms a block statement
func (t *StringConcatTransformer) transformBlock(stmt syntax.Stmt) bool {
	blockStmt, ok := stmt.(*syntax.BlockStmt)
	if !ok {
		return false
	}
	
	changed := false
	for _, s := range blockStmt.List {
		if t.transformStatement(s) {
			changed = true
		}
	}
	return changed
}

// transformStatement safely transforms individual statements
func (t *StringConcatTransformer) transformStatement(stmt syntax.Stmt) bool {
	switch s := stmt.(type) {
	case *syntax.ExprStmt:
		return t.transformExpressionSafely(s.X)
	case *syntax.AssignStmt:
		return t.transformExpressionSafely(s.Rhs)
	}
	return false
}

// transformExpressionSafely transforms expressions with nil checks
func (t *StringConcatTransformer) transformExpressionSafely(expr syntax.Expr) bool {
	if expr == nil {
		return false
	}
	
	// Look for function calls first
	if callExpr, ok := expr.(*syntax.CallExpr); ok {
		changed := false
		for _, arg := range callExpr.ArgList {
			if t.transformConcatenation(arg) {
				changed = true
			}
		}
		return changed
	}
	
	return t.transformConcatenation(expr)
}

// transformConcatenation handles the specific string + variable case
func (t *StringConcatTransformer) transformConcatenation(expr syntax.Expr) bool {
	operation, ok := expr.(*syntax.Operation)
	if !ok || operation.Op != syntax.Add {
		return false
	}
	
	if operation.X == nil || operation.Y == nil {
		return false
	}
	
	// Check for "string" + identifier pattern
	if t.isStringLiteral(operation.X) && t.isIdentifier(operation.Y) {
		operation.Y = t.createItoacCall(operation.Y)
		return true
	}
	
	// Check for identifier + "string" pattern  
	if t.isIdentifier(operation.X) && t.isStringLiteral(operation.Y) {
		operation.X = t.createItoacCall(operation.X)
		return true
	}
	
	return false
}

// isStringLiteral checks if expression is a string literal
func (t *StringConcatTransformer) isStringLiteral(expr syntax.Expr) bool {
	if basic, ok := expr.(*syntax.BasicLit); ok {
		return basic.Kind == syntax.StringLit
	}
	return false
}

// isIdentifier checks if expression is a simple identifier
func (t *StringConcatTransformer) isIdentifier(expr syntax.Expr) bool {
	_, ok := expr.(*syntax.Name)
	return ok
}

// createItoacCall creates strconv.Itoa(expr) call
func (t *StringConcatTransformer) createItoacCall(expr syntax.Expr) syntax.Expr {
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