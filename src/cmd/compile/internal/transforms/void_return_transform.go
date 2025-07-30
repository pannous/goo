// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// VoidReturnTransform handles problematic return statements in void functions.
// It transforms: return println("OK") --> println("OK"); return
type VoidReturnTransform struct{}

// voidReturnVisitor implements the visitor pattern for void return transformation
type voidReturnVisitor struct {
	transform       *VoidReturnTransform
	ctx             *TransformContext
	changed         bool
	currentFunction *syntax.FuncDecl // Track current function context
}

func (t *VoidReturnTransform) Name() string {
	return "void_return_transform"
}

func (t *VoidReturnTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &voidReturnVisitor{transform: t, ctx: ctx}
	
	// Use the general visitor pattern to walk all nodes
	syntax.Walk(file, visitor)
	
	return visitor.changed
}

// Visit implements syntax.Visitor - hybrid approach
func (v *voidReturnVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}
	
	// Look for void functions and transform them using the existing logic
	if funcDecl, ok := node.(*syntax.FuncDecl); ok {
		if v.transform.isVoidFunction(funcDecl) {
			// Use the existing block transformation logic for complex statement manipulation
			if v.transformVoidFunction(funcDecl) {
				v.changed = true
			}
		}
	}
	
	// Continue visiting child nodes
	return v
}

// transformVoidFunction transforms a void function using the existing block logic
func (v *voidReturnVisitor) transformVoidFunction(funcDecl *syntax.FuncDecl) bool {
	if funcDecl.Body == nil {
		return false
	}
	
	return v.walkBlock(funcDecl.Body)
}

// walkBlock walks through statements in a block (reused existing logic)
func (v *voidReturnVisitor) walkBlock(block *syntax.BlockStmt) bool {
	if block == nil {
		return false
	}
	
	changed := false
	for i := 0; i < len(block.List); i++ {
		stmt := block.List[i]

		// Check if this is a problematic return statement
		if retStmt, ok := stmt.(*syntax.ReturnStmt); ok {
			if retStmt.Results != nil {
				// Transform: return expr --> expr; return
				exprStmt := &syntax.ExprStmt{X: retStmt.Results}
				exprStmt.SetPos(retStmt.Pos())

				emptyReturn := &syntax.ReturnStmt{}
				emptyReturn.SetPos(retStmt.Pos())

				// Replace current statement with expression
				block.List[i] = exprStmt

				// Insert empty return after expression
				newStmts := make([]syntax.Stmt, 0, len(block.List)+1)
				newStmts = append(newStmts, block.List[:i+1]...)
				newStmts = append(newStmts, emptyReturn)
				newStmts = append(newStmts, block.List[i+1:]...)
				block.List = newStmts

				changed = true
				i++ // Skip the newly inserted return statement
			}
		} else {
			// Recursively walk nested statements
			if v.walkStatement(stmt) {
				changed = true
			}
		}
	}
	return changed
}

// walkStatement walks nested statements (reused existing logic)
func (v *voidReturnVisitor) walkStatement(stmt syntax.Stmt) bool {
	changed := false
	switch s := stmt.(type) {
	case *syntax.IfStmt:
		if s.Then != nil {
			if v.walkBlock(s.Then) {
				changed = true
			}
		}
		if s.Else != nil {
			if v.walkStatement(s.Else) {
				changed = true
			}
		}
	case *syntax.BlockStmt:
		if v.walkBlock(s) {
			changed = true
		}
	case *syntax.ForStmt:
		if s.Body != nil {
			if v.walkBlock(s.Body) {
				changed = true
			}
		}
	}
	return changed
}

// isVoidFunction checks if a function has no return types
func (t *VoidReturnTransform) isVoidFunction(funcDecl *syntax.FuncDecl) bool {
	return funcDecl.Type == nil ||
		funcDecl.Type.ResultList == nil ||
		len(funcDecl.Type.ResultList) == 0
}

func init() {
	RegisterTransformer(&VoidReturnTransform{})
}
