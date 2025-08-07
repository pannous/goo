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

// autoReturnVisitor implements the visitor pattern for auto-return transformation
type autoReturnVisitor struct {
	transform *AutoReturnTransform
	ctx       *TransformContext
	changed   bool
}

func (t *AutoReturnTransform) Name() string {
	return "auto_return"
}

func (t *AutoReturnTransform) Priority() int {
	return 100 // Default priority - between list methods (50) and lambda (200)
}

// NodeTransformer interface implementation
func (t *AutoReturnTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
	// Check if this is a function declaration that needs auto-return
	if funcDecl, ok := node.(*syntax.FuncDecl); ok {
		return t.needsAutoReturn(funcDecl)
	}
	return false
}

func (t *AutoReturnTransform) TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node {
	if funcDecl, ok := node.(*syntax.FuncDecl); ok {
		if t.transformFunction(funcDecl, ctx) {
			return funcDecl // Return modified function
		}
	}
	return nil
}

func (t *AutoReturnTransform) PostProcess(file *syntax.File, ctx *TransformContext) bool {
	// No post-processing needed for auto return transform
	return false
}

func (t *AutoReturnTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &autoReturnVisitor{transform: t, ctx: ctx}
	
	// Use the general visitor pattern to walk all nodes
	syntax.Walk(file, visitor)
	
	return visitor.changed
}

// Visit implements syntax.Visitor
func (v *autoReturnVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}
	
	// Look for function declarations that need auto-return transformation
	if funcDecl, ok := node.(*syntax.FuncDecl); ok {
		if v.transform.needsAutoReturn(funcDecl) {
			if v.transform.transformFunction(funcDecl, v.ctx) {
				v.changed = true
			}
		}
	}
	
	// Continue visiting child nodes
	return v
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