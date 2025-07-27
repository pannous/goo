// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// VoidReturnTransform handles problematic return statements in void functions.
// It transforms: return println("OK") --> println("OK"); return
type VoidReturnTransform struct{}

func (t *VoidReturnTransform) Name() string {
	return "void_return"
}

func (t *VoidReturnTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	changed := false
	
	// Walk through all declarations in the file
	for _, decl := range file.DeclList {
		if funcDecl, ok := decl.(*syntax.FuncDecl); ok {
			if t.isVoidFunction(funcDecl) {
				walker := &voidReturnWalker{transform: t}
				if walker.walkFunction(funcDecl) {
					changed = true
				}
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

// voidReturnWalker walks the AST looking for problematic return statements
type voidReturnWalker struct {
	transform *VoidReturnTransform
	changed   bool
}

// walkFunction walks through a function's body
func (w *voidReturnWalker) walkFunction(funcDecl *syntax.FuncDecl) bool {
	if funcDecl.Body == nil {
		return false
	}
	
	w.changed = false
	w.walkBlock(funcDecl.Body)
	return w.changed
}

// walkBlock walks through statements in a block
func (w *voidReturnWalker) walkBlock(block *syntax.BlockStmt) {
	if block == nil {
		return
	}
	
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
				
				w.changed = true
				i++ // Skip the newly inserted return statement
			}
		} else {
			// Recursively walk nested statements
			w.walkStatement(stmt)
		}
	}
}

// walkStatement walks nested statements
func (w *voidReturnWalker) walkStatement(stmt syntax.Stmt) {
	switch s := stmt.(type) {
	case *syntax.IfStmt:
		if s.Then != nil {
			w.walkBlock(s.Then)
		}
		if s.Else != nil {
			w.walkStatement(s.Else)
		}
	case *syntax.BlockStmt:
		w.walkBlock(s)
	case *syntax.ForStmt:
		if s.Body != nil {
			w.walkBlock(s.Body)
		}
	}
}

func init() {
	RegisterTransformer(&VoidReturnTransform{})
}