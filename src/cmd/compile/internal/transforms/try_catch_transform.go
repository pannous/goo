// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
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
	//fmt.Printf("TryCatchTransform.Transform called\n")
	visitor := &tryCatchVisitor{ctx: ctx}

	// Transform function declarations
	for _, decl := range file.DeclList {
		if funcDecl, ok := decl.(*syntax.FuncDecl); ok && funcDecl.Body != nil {
			visitor.walkBlockStmt(funcDecl.Body)
		}
	}

	// Transform top-level statements
	for i, stmt := range file.TopLevelStmts {
		if newStmt := visitor.transformStmt(stmt); newStmt != nil {
			file.TopLevelStmts[i] = newStmt
			visitor.changed = true
		}
	}

	return visitor.changed
}

// tryCatchVisitor implements the visitor pattern for try-catch transformation
type tryCatchVisitor struct {
	ctx     *TransformContext
	changed bool
}

// walkBlockStmt walks through all statements in a block
func (v *tryCatchVisitor) walkBlockStmt(block *syntax.BlockStmt) {
	for i, stmt := range block.List {
		if newStmt := v.transformStmt(stmt); newStmt != nil {
			block.List[i] = newStmt
			v.changed = true
		}

		// Recursively walk nested blocks
		v.walkNestedStmt(stmt)
	}
}

// walkNestedStmt recursively walks nested statements
func (v *tryCatchVisitor) walkNestedStmt(stmt syntax.Stmt) {
	switch s := stmt.(type) {
	case *syntax.BlockStmt:
		v.walkBlockStmt(s)
	case *syntax.IfStmt:
		if s.Then != nil {
			v.walkBlockStmt(s.Then)
		}
		if elseBlock, ok := s.Else.(*syntax.BlockStmt); ok {
			v.walkBlockStmt(elseBlock)
		}
	case *syntax.ForStmt:
		if s.Body != nil {
			v.walkBlockStmt(s.Body)
		}
	case *syntax.SwitchStmt:
		for _, clause := range s.Body {
			if clause.Body != nil {
				for i, caseStmt := range clause.Body {
					if newStmt := v.transformStmt(caseStmt); newStmt != nil {
						clause.Body[i] = newStmt
						v.changed = true
					}
					v.walkNestedStmt(caseStmt)
				}
			}
		}
	}
}

// transformStmt transforms a single statement if it's a try-catch or try
func (v *tryCatchVisitor) transformStmt(stmt syntax.Stmt) syntax.Stmt {
	if tryCatchStmt, ok := stmt.(*syntax.TryCatchStmt); ok {
		return v.transformTryCatch(tryCatchStmt)
	}
	if tryStmt, ok := stmt.(*syntax.TryStmt); ok {
		return v.transformTry(tryStmt)
	}
	return nil
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

// transformTry transforms try f() to func() { if err := f(); err != nil { panic(err) } }()
func (v *tryCatchVisitor) transformTry(tryStmt *syntax.TryStmt) syntax.Stmt {
	pos := tryStmt.Pos()

	// Create err variable
	errVar := &syntax.Name{Value: "err"}
	errVar.SetPos(pos)

	// Create assignment: err := f()
	assign := &syntax.AssignStmt{
		Op:  syntax.Def, // :=
		Lhs: errVar,
		Rhs: tryStmt.Call,
	}
	assign.SetPos(pos)

	// Create condition: err != nil
	nilName := &syntax.Name{Value: "nil"}
	nilName.SetPos(pos)

	condition := &syntax.Operation{
		Op: syntax.Neq,
		X:  errVar,
		Y:  nilName,
	}
	condition.SetPos(pos)

	// Create panic call: panic(err)
	panicCall := &syntax.CallExpr{
		Fun:     &syntax.Name{Value: "panic"},
		ArgList: []syntax.Expr{errVar},
	}
	panicCall.SetPos(pos)
	panicCall.Fun.SetPos(pos)

	// Create panic statement
	panicStmt := &syntax.ExprStmt{
		X: panicCall,
	}
	panicStmt.SetPos(pos)

	// Create if body
	ifBody := &syntax.BlockStmt{
		List: []syntax.Stmt{panicStmt},
	}
	ifBody.SetPos(pos)

	// Create if statement: if err := f(); err != nil { panic(err) }
	ifStmt := &syntax.IfStmt{
		Init: assign,
		Cond: condition,
		Then: ifBody,
	}
	ifStmt.SetPos(pos)

	// Wrap in function literal like the existing try-catch
	wrapperBody := &syntax.BlockStmt{
		List: []syntax.Stmt{ifStmt},
	}
	wrapperBody.SetPos(pos)

	// Create wrapper function: func() { if ... }
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
