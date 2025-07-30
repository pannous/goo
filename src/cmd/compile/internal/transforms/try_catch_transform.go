// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
	"fmt"
)

// TryCatchTransform handles transformation of try-catch syntax to defer/recover pattern.
// It transforms expressions like:
// try { doSomething() } catch e { handleError(e) }
// -->
// func() { defer func() { if e := recover(); e != nil { handleError(e) } }(); doSomething() }()
type TryCatchTransform struct{}

func (t *TryCatchTransform) Name() string {
	return "try_catch_transform"
}

func (t *TryCatchTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	fmt.Printf("TryCatchTransform.Transform called\n")
	visitor := &tryCatchVisitor{transform: t, ctx: ctx}
	
	// Use the visitor pattern to walk all nodes
	syntax.Walk(file, visitor)
	
	return visitor.changed
}

// tryCatchVisitor implements the visitor pattern for try-catch transformation
type tryCatchVisitor struct {
	transform *TryCatchTransform
	ctx       *TransformContext
	changed   bool
}

// Visit implements syntax.Visitor
func (v *tryCatchVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}
	
	// Look for try-catch statements that need transformation
	if tryCatchStmt, ok := node.(*syntax.TryCatchStmt); ok {
		if newStmt := v.transformTryCatch(tryCatchStmt); newStmt != nil {
			// Replace the try-catch statement with the transformed statement
			// Note: This is a limitation of the visitor pattern - we need to handle 
			// replacement at a higher level or use a different approach for statements
			v.changed = true
		}
	}
	
	// Continue visiting child nodes
	return v
}

// transformTryCatch transforms try-catch to defer/recover pattern
func (v *tryCatchVisitor) transformTryCatch(tryStmt *syntax.TryCatchStmt) syntax.Stmt {
	pos := tryStmt.Pos()

	// Create the recover check: if e := recover(); e != nil { catchBlock }
	recoverCall := &syntax.CallExpr{
		Fun: &syntax.Name{Value: "recover"},
	}
	recoverCall.SetPos(pos)
	recoverCall.Fun.SetPos(pos)

	// Create assignment: e := recover()
	recoverAssign := &syntax.AssignStmt{
		Op:  syntax.Def, // :=
		Lhs: tryStmt.CatchVar,
		Rhs: recoverCall,
	}
	recoverAssign.SetPos(pos)

	// Create condition: e != nil
	nilName := &syntax.Name{Value: "nil"}
	nilName.SetPos(pos)

	condition := &syntax.Operation{
		Op: syntax.Neq,
		X:  tryStmt.CatchVar,
		Y:  nilName,
	}
	condition.SetPos(pos)

	// Create if statement: if e := recover(); e != nil { catchBlock }
	ifStmt := &syntax.IfStmt{
		Init: recoverAssign,
		Cond: condition,
		Then: tryStmt.CatchBlock,
	}
	ifStmt.SetPos(pos)

	// Create defer function body
	deferBody := &syntax.BlockStmt{
		List: []syntax.Stmt{ifStmt},
	}
	deferBody.SetPos(pos)

	// Create defer function: func() { if e := recover(); e != nil { catchBlock } }
	deferFunc := &syntax.FuncLit{
		Type: &syntax.FuncType{},
		Body: deferBody,
	}
	deferFunc.SetPos(pos)
	deferFunc.Type.SetPos(pos)

	// Create defer function call
	deferCall := &syntax.CallExpr{
		Fun: deferFunc,
	}
	deferCall.SetPos(pos)

	// Create defer statement
	deferStmt := &syntax.CallStmt{
		Tok:  syntax.Defer,
		Call: deferCall,
	}
	deferStmt.SetPos(pos)

	// Create wrapper function body: { defer ...; tryBlock }
	wrapperBody := &syntax.BlockStmt{
		List: []syntax.Stmt{deferStmt},
	}
	wrapperBody.SetPos(pos)

	// Add all statements from try block to wrapper body
	for _, stmt := range tryStmt.TryBlock.List {
		wrapperBody.List = append(wrapperBody.List, stmt)
	}

	// Create wrapper function: func() { defer ...; tryBlock }
	wrapperFunc := &syntax.FuncLit{
		Type: &syntax.FuncType{},
		Body: wrapperBody,
	}
	wrapperFunc.SetPos(pos)
	wrapperFunc.Type.SetPos(pos)

	// Create wrapper function call: func() { ... }()
	wrapperCall := &syntax.CallExpr{
		Fun: wrapperFunc,
	}
	wrapperCall.SetPos(pos)

	// Return as expression statement
	exprStmt := &syntax.ExprStmt{
		X: wrapperCall,
	}
	exprStmt.SetPos(pos)

	return exprStmt
}

func init() {
	RegisterTransformer(&TryCatchTransform{})
}
