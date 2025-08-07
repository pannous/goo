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

func (t *TryCatchTransform) Priority() int {
	return 100 // Default priority - between list methods (50) and lambda (200)
}

// NodeTransformer interface implementation
func (t *TryCatchTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
	// Handle TryCatchStmt and TryStmt nodes
	_, isTryCatch := node.(*syntax.TryCatchStmt)
	_, isTry := node.(*syntax.TryStmt)
	return isTryCatch || isTry
}

func (t *TryCatchTransform) TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node {
	visitor := &tryCatchVisitor{ctx: ctx}
	if tryStmt, ok := node.(*syntax.TryCatchStmt); ok {
		return visitor.transformTryCatch(tryStmt)
	}
	if tryStmt, ok := node.(*syntax.TryStmt); ok {
		return t.transformTry(tryStmt, ctx)
	}
	return nil
}

func (t *TryCatchTransform) PostProcess(file *syntax.File, ctx *TransformContext) bool {
	// No post-processing needed for try-catch transform
	return false
}

// Legacy Transform method for backward compatibility - not used in new architecture
func (t *TryCatchTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	//fmt.Printf("TryCatchTransform.Transform called\n")
	visitor := &tryCatchVisitor{ctx: ctx}

	// Transform function declarations
	for _, decl := range file.DeclList {
		if funcDecl, ok := decl.(*syntax.FuncDecl); ok && funcDecl.Body != nil {
			// Set current function context
			visitor.currentFunc = funcDecl
			visitor.walkBlockStmt(funcDecl.Body)
			visitor.currentFunc = nil
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
	ctx         *TransformContext
	changed     bool
	currentFunc *syntax.FuncDecl // Track current function context
}

// walkBlockStmt walks through all statements in a block
func (v *tryCatchVisitor) walkBlockStmt(block *syntax.BlockStmt) {
	newList := make([]syntax.Stmt, 0, len(block.List))
	
	for _, stmt := range block.List {
		if tryStmt, ok := stmt.(*syntax.TryStmt); ok && tryStmt.Var != nil {
			// Special handling for try val := f() - need to expand to multiple statements
			assign, ifStmt := v.transformTryToStatements(tryStmt)
			newList = append(newList, assign, ifStmt)
			v.changed = true
		} else if newStmt := v.transformStmt(stmt); newStmt != nil {
			newList = append(newList, newStmt)
			v.changed = true
		} else {
			newList = append(newList, stmt)
		}

		// Recursively walk nested blocks
		v.walkNestedStmt(stmt)
	}
	
	block.List = newList
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
		// Create a TryCatchTransform instance for compatibility
		t := &TryCatchTransform{}
		return t.transformTry(tryStmt, v.ctx)
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

// transformTry transforms try f() to { err := f(); if err != nil { return err } }  
// or try val := f() to { val, err := f(); if err != nil { return err } }
// Updated for NodeTransformer interface - simplified without visitor context
func (t *TryCatchTransform) transformTry(tryStmt *syntax.TryStmt, ctx *TransformContext) syntax.Stmt {
	pos := tryStmt.Pos()

	var assign *syntax.AssignStmt
	if tryStmt.Var != nil {
		// try val := f() case - multi-assignment: val, err := f()
		errVar := &syntax.Name{Value: "err"}
		errVar.SetPos(pos)
		
		lhsList := &syntax.ListExpr{ElemList: []syntax.Expr{tryStmt.Var, errVar}}
		lhsList.SetPos(pos)
		
		assign = &syntax.AssignStmt{
			Op:  syntax.Def, // :=
			Lhs: lhsList,
			Rhs: tryStmt.Call,
		}
	} else {
		// try f() case - single assignment: err := f()
		assign = &syntax.AssignStmt{
			Op:  syntax.Def, // :=
			Lhs: &syntax.Name{Value: "err"},
			Rhs: tryStmt.Call,
		}
		assign.Lhs.SetPos(pos)
	}
	assign.SetPos(pos)

	// Create condition: err != nil
	condition := &syntax.Operation{
		Op: syntax.Neq,
		X:  &syntax.Name{Value: "err"},
		Y:  &syntax.Name{Value: "nil"},
	}
	condition.X.SetPos(pos)
	condition.Y.SetPos(pos)
	condition.SetPos(pos)

	// Create context-aware return statement - simplified for NodeTransformer
	returnStmt := t.createSimpleReturn(pos)

	// Create if body
	ifBody := &syntax.BlockStmt{
		List: []syntax.Stmt{returnStmt},
	}
	ifBody.SetPos(pos)

	// Create if statement: if err != nil { return ... }
	ifStmt := &syntax.IfStmt{
		Cond: condition,
		Then: ifBody,
	}
	ifStmt.SetPos(pos)

	// We need to return multiple statements but can only return one
	// So create a sequence using a technique that works with the visitor pattern
	// For now, let's use a wrapper block but mark it specially
	block := &syntax.BlockStmt{
		List: []syntax.Stmt{assign, ifStmt},
	}
	block.SetPos(pos)

	return block
}

// transformTryToStatements transforms try val := f() to separate statements:
// val, err := f(); if err != nil { return err }
func (v *tryCatchVisitor) transformTryToStatements(tryStmt *syntax.TryStmt) (syntax.Stmt, syntax.Stmt) {
	pos := tryStmt.Pos()

	// Create assignment: val, err := f()
	errVar := &syntax.Name{Value: "err"}
	errVar.SetPos(pos)
	
	lhsList := &syntax.ListExpr{ElemList: []syntax.Expr{tryStmt.Var, errVar}}
	lhsList.SetPos(pos)
	
	assign := &syntax.AssignStmt{
		Op:  syntax.Def, // :=
		Lhs: lhsList,
		Rhs: tryStmt.Call,
	}
	assign.SetPos(pos)

	// Create condition: err != nil
	condition := &syntax.Operation{
		Op: syntax.Neq,
		X:  &syntax.Name{Value: "err"},
		Y:  &syntax.Name{Value: "nil"},
	}
	condition.X.SetPos(pos)
	condition.Y.SetPos(pos)
	condition.SetPos(pos)

	// Create simple return statement  
	returnStmt := createSimpleReturnStatic(pos)

	// Create if body
	ifBody := &syntax.BlockStmt{
		List: []syntax.Stmt{returnStmt},
	}
	ifBody.SetPos(pos)

	// Create if statement: if err != nil { return err }
	ifStmt := &syntax.IfStmt{
		Cond: condition,
		Then: ifBody,
	}
	ifStmt.SetPos(pos)

	return assign, ifStmt
}

// createContextAwareReturn creates an appropriate return statement based on function context
func (v *tryCatchVisitor) createContextAwareReturn(pos syntax.Pos, ctx *TransformContext) syntax.Stmt {
	// Analyze the current function's return type
	returnValues := v.analyzeReturnSignature()
	
	switch len(returnValues) {
	case 0:
		// No return values: panic(err)
		errVar := &syntax.Name{Value: "err"}
		errVar.SetPos(pos)
		
		panicCall := &syntax.CallExpr{
			Fun:     &syntax.Name{Value: "panic"},
			ArgList: []syntax.Expr{errVar},
		}
		panicCall.SetPos(pos)
		panicCall.Fun.SetPos(pos)
		
		// Return panic as expression statement
		panicStmt := &syntax.ExprStmt{X: panicCall}
		panicStmt.SetPos(pos)
		return panicStmt
		
	case 1:
		// Single return value (error): return err
		errVar := &syntax.Name{Value: "err"}
		errVar.SetPos(pos)
		
		returnStmt := &syntax.ReturnStmt{Results: errVar}
		returnStmt.SetPos(pos)
		return returnStmt
		
	default:
		// Multiple return values: return zero values..., err
		zeroValues := v.createZeroValues(returnValues[:len(returnValues)-1], pos)
		errVar := &syntax.Name{Value: "err"}
		errVar.SetPos(pos)
		
		// Create list of return values: zero1, zero2, ..., err
		allValues := append(zeroValues, errVar)
		var results syntax.Expr
		if len(allValues) == 1 {
			results = allValues[0]
		} else {
			listExpr := &syntax.ListExpr{ElemList: allValues}
			listExpr.SetPos(pos)
			results = listExpr
		}
		
		returnStmt := &syntax.ReturnStmt{Results: results}
		returnStmt.SetPos(pos)
		return returnStmt
	}
}

// analyzeReturnSignature analyzes the current function's return signature
func (v *tryCatchVisitor) analyzeReturnSignature() []syntax.Expr {
	if v.currentFunc == nil || v.currentFunc.Type == nil {
		return []syntax.Expr{} // No context, assume void function
	}
	
	funcType := v.currentFunc.Type
	if funcType.ResultList == nil {
		return []syntax.Expr{} // No return values
	}
	
	// Convert ResultList to expressions (simplified)
	var results []syntax.Expr
	for _, field := range funcType.ResultList {
		results = append(results, field.Type)
	}
	
	return results
}

// createZeroValues creates zero values for the given types
func (v *tryCatchVisitor) createZeroValues(types []syntax.Expr, pos syntax.Pos) []syntax.Expr {
	var zeros []syntax.Expr
	
	for _, typ := range types {
		var zeroValue syntax.Expr
		
		// Determine zero value based on type
		if name, ok := typ.(*syntax.Name); ok {
			switch name.Value {
			case "int", "int8", "int16", "int32", "int64",
				 "uint", "uint8", "uint16", "uint32", "uint64",
				 "float32", "float64", "byte", "rune":
				zeroValue = &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
			case "string":
				zeroValue = &syntax.BasicLit{Kind: syntax.StringLit, Value: `""`}
			case "bool":
				zeroValue = &syntax.Name{Value: "false"}
			default:
				// For other types, use nil
				zeroValue = &syntax.Name{Value: "nil"}
			}
		} else {
			// For complex types, use nil
			zeroValue = &syntax.Name{Value: "nil"}
		}
		
		zeroValue.SetPos(pos)
		zeros = append(zeros, zeroValue)
	}
	
	return zeros
}

// createSimpleReturn creates a simplified return statement for NodeTransformer interface
func (t *TryCatchTransform) createSimpleReturn(pos syntax.Pos) syntax.Stmt {
	return createSimpleReturnStatic(pos)
}

// createSimpleReturnStatic creates a simplified return statement (static helper)
func createSimpleReturnStatic(pos syntax.Pos) syntax.Stmt {
	// For simplified NodeTransformer interface, just return err
	// More complex function context analysis would require old visitor pattern
	errVar := &syntax.Name{Value: "err"}
	errVar.SetPos(pos)
	
	returnStmt := &syntax.ReturnStmt{Results: errVar}
	returnStmt.SetPos(pos)
	return returnStmt
}

func init() {
	RegisterTransformer(&TryCatchTransform{})
}
