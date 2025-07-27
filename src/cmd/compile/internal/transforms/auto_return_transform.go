// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// AutoReturnTransform handles automatic return statement insertion.
// It transforms expressions like:
// def meaning() int {42} --> def meaning() int {return 42}
type AutoReturnTransform struct{}

func (t *AutoReturnTransform) Name() string {
	return "auto_return"
}

func (t *AutoReturnTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	changed := false
	
	// Walk through all declarations in the file
	for _, decl := range file.DeclList {
		if funcDecl, ok := decl.(*syntax.FuncDecl); ok {
			if t.needsAutoReturn(funcDecl) {
				if t.transformFunction(funcDecl, ctx) {
					changed = true
				}
			}
		}
	}

	return changed
}

// needsAutoReturn checks if a function needs automatic return transformation
func (t *AutoReturnTransform) needsAutoReturn(funcDecl *syntax.FuncDecl) bool {
	// Must have a function body
	if funcDecl.Body == nil {
		return false
	}
	
	// Must have return type(s)
	if funcDecl.Type == nil || funcDecl.Type.ResultList == nil || len(funcDecl.Type.ResultList) == 0 {
		return false
	}
	
	// Must have statements in body
	if len(funcDecl.Body.List) == 0 {
		return false
	}
	
	// Check if the last statement is not already a return statement
	lastStmt := funcDecl.Body.List[len(funcDecl.Body.List)-1]
	_, isReturn := lastStmt.(*syntax.ReturnStmt)
	
	return !isReturn
}

// transformFunction transforms a function to add automatic return
func (t *AutoReturnTransform) transformFunction(funcDecl *syntax.FuncDecl, ctx *TransformContext) bool {
	pos := funcDecl.Pos()
	
	if len(funcDecl.Body.List) == 0 {
		return false
	}
	
	lastStmt := funcDecl.Body.List[len(funcDecl.Body.List)-1]
	
	// Check if last statement is an expression statement
	if exprStmt, ok := lastStmt.(*syntax.ExprStmt); ok {
		// Create a return statement with the expression
		returnStmt := &syntax.ReturnStmt{
			Results: exprStmt.X,
		}
		returnStmt.SetPos(pos)
		
		// Replace the last statement with the return statement
		funcDecl.Body.List[len(funcDecl.Body.List)-1] = returnStmt
		
		return true
	}
	
	return false
}

func init() {
	RegisterTransformer(&AutoReturnTransform{})
}