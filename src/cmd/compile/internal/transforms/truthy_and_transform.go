// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// TruthyAndTransform handles Python-style truthy 'and' operator.
// It transforms expressions like:
// user and user.Name --> func() any { if isTruthy(user) { return user.Name }; return user }()
// x and y --> func() any { if isTruthy(x) { return y }; return x }()

type TruthyAndTransform struct{}

func (t *TruthyAndTransform) Name() string {
	return "truthy_and_transform"
}

func (t *TruthyAndTransform) Priority() int {
	return 10 // Run very early, before most other transforms
}

func (t *TruthyAndTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	changed := false
	
	// Walk the AST and transform truthy and operations
	for i, decl := range file.DeclList {
		newDecl := t.transformDecl(decl)
		if newDecl != decl {
			file.DeclList[i] = newDecl
			changed = true
		}
	}
	
	return changed
}

func (t *TruthyAndTransform) transformDecl(decl syntax.Decl) syntax.Decl {
	switch d := decl.(type) {
	case *syntax.FuncDecl:
		if newBody := t.transformStmt(d.Body); newBody != d.Body {
			newDecl := *d
			if blockStmt, ok := newBody.(*syntax.BlockStmt); ok {
				newDecl.Body = blockStmt
				return &newDecl
			}
		}
	}
	return decl
}

func (t *TruthyAndTransform) transformStmt(stmt syntax.Stmt) syntax.Stmt {
	if stmt == nil {
		return nil
	}

	switch s := stmt.(type) {
	case *syntax.BlockStmt:
		changed := false
		newList := make([]syntax.Stmt, len(s.List))
		for i, stmt := range s.List {
			newStmt := t.transformStmt(stmt)
			newList[i] = newStmt
			if newStmt != stmt {
				changed = true
			}
		}
		if changed {
			newBlock := *s
			newBlock.List = newList
			return &newBlock
		}
	case *syntax.IfStmt:
		// First transform any 'and' operations in the condition
		newCond := t.transformExpr(s.Cond)

		// Then wrap the entire condition in truthy() to handle non-boolean types
		// (numbers, strings, slices, etc.)
		wrappedCond := t.wrapInTruthy(newCond)

		newThen := t.transformStmt(s.Then)
		var newElse syntax.Stmt
		if s.Else != nil {
			newElse = t.transformStmt(s.Else)
		}

		if wrappedCond != s.Cond || newThen != s.Then || newElse != s.Else {
			newIf := *s
			newIf.Cond = wrappedCond
			if thenBlock, ok := newThen.(*syntax.BlockStmt); ok {
				newIf.Then = thenBlock
			}
			newIf.Else = newElse
			return &newIf
		}
	case *syntax.ExprStmt:
		newExpr := t.transformExpr(s.X)
		if newExpr != s.X {
			newStmt := *s
			newStmt.X = newExpr
			return &newStmt
		}
	case *syntax.CheckStmt:
		newCond := t.transformExpr(s.Cond)
		if newCond != s.Cond {
			newCheck := *s
			newCheck.Cond = newCond
			return &newCheck
		}
	case *syntax.AssignStmt:
		// Handle assignment statements like y := x and 7
		newRhs := t.transformExpr(s.Rhs)
		if newRhs != s.Rhs {
			newAssign := *s
			newAssign.Rhs = newRhs
			return &newAssign
		}
	case *syntax.DeclStmt:
		// Handle declaration statements  
		decl := s.DeclList[0] // Assuming single declaration
		if varDecl, ok := decl.(*syntax.VarDecl); ok {
			if varDecl.Values != nil && len(varDecl.Values.(*syntax.ListExpr).ElemList) > 0 {
				values := varDecl.Values.(*syntax.ListExpr).ElemList
				changed := false
				newValues := make([]syntax.Expr, len(values))
				for i, value := range values {
					newValue := t.transformExpr(value)
					newValues[i] = newValue
					if newValue != value {
						changed = true
					}
				}
				if changed {
					newDecl := *s
					newVarDecl := *varDecl
					newValuesList := &syntax.ListExpr{ElemList: newValues}
					newValuesList.SetPos(varDecl.Values.Pos())
					newVarDecl.Values = newValuesList
					newDecl.DeclList = []syntax.Decl{&newVarDecl}
					return &newDecl
				}
			}
		}
	}
	
	return stmt
}

func (t *TruthyAndTransform) transformExpr(expr syntax.Expr) syntax.Expr {
	if expr == nil {
		return nil
	}
	
	// Debug removed for cleaner output

	switch e := expr.(type) {
	case *syntax.Operation:
		// Look for 'and' operations (new TruthyAnd token for truthy and)
		if e.Op == syntax.TruthyAnd {
			return t.createTruthyAndCall(e.X, e.Y, e.Pos())
		}
		
		// Transform operands recursively
		newX := t.transformExpr(e.X)
		var newY syntax.Expr
		if e.Y != nil {
			newY = t.transformExpr(e.Y)
		}
		
		if newX != e.X || newY != e.Y {
			newOp := *e
			newOp.X = newX
			newOp.Y = newY
			return &newOp
		}
	case *syntax.ParenExpr:
		// Transform parenthesized expressions
		newX := t.transformExpr(e.X)
		if newX != e.X {
			newParen := *e
			newParen.X = newX
			return &newParen
		}
	case *syntax.CallExpr:
		// Transform function arguments
		newFun := t.transformExpr(e.Fun)
		var newArgs []syntax.Expr
		argsChanged := false
		
		if e.ArgList != nil {
			newArgs = make([]syntax.Expr, len(e.ArgList))
			for i, arg := range e.ArgList {
				newArg := t.transformExpr(arg)
				newArgs[i] = newArg
				if newArg != arg {
					argsChanged = true
				}
			}
		}
		
		if newFun != e.Fun || argsChanged {
			newCall := *e
			newCall.Fun = newFun
			newCall.ArgList = newArgs
			return &newCall
		}
	}
	
	return expr
}

// wrapInTruthy wraps an expression in a truthy() call
// Skip wrapping if it's already a boolean operation
func (t *TruthyAndTransform) wrapInTruthy(expr syntax.Expr) syntax.Expr {
	// Check if it's already a boolean operation that doesn't need wrapping
	if op, ok := expr.(*syntax.Operation); ok {
		switch op.Op {
		case syntax.Eql, syntax.Neq, syntax.Lss, syntax.Leq, syntax.Gtr, syntax.Geq,
			syntax.AndAnd, syntax.OrOr:
			// These already return bool, no need to wrap
			return expr
		case syntax.Not:
			// ! already returns bool
			return expr
		}
	}

	// Check if it's already a truthy call
	if call, ok := expr.(*syntax.CallExpr); ok {
		if name, ok := call.Fun.(*syntax.Name); ok && name.Value == "truthy" {
			// Already wrapped, don't double-wrap
			return expr
		}
	}

	// Wrap in truthy()
	truthyName := &syntax.Name{Value: "truthy"}
	truthyName.SetPos(expr.Pos())

	call := &syntax.CallExpr{
		Fun:     truthyName,
		ArgList: []syntax.Expr{expr},
	}
	call.SetPos(expr.Pos())

	return call
}

// createTruthyAndCall creates a truthy and operation using truthy calls with &&
func (t *TruthyAndTransform) createTruthyAndCall(left, right syntax.Expr, pos syntax.Pos) syntax.Expr {
	// Create truthy(left) call
	truthyName1 := &syntax.Name{Value: "truthy"}
	truthyName1.SetPos(pos)
	
	leftTruthyCall := &syntax.CallExpr{
		Fun:     truthyName1,
		ArgList: []syntax.Expr{left},
	}
	leftTruthyCall.SetPos(pos)
	
	// Create truthy(right) call
	truthyName2 := &syntax.Name{Value: "truthy"}
	truthyName2.SetPos(pos)
	
	rightTruthyCall := &syntax.CallExpr{
		Fun:     truthyName2,
		ArgList: []syntax.Expr{right},
	}
	rightTruthyCall.SetPos(pos)
	
	// Create && operation with truthy calls
	andOp := &syntax.Operation{
		Op: syntax.AndAnd,
		X:  leftTruthyCall,
		Y:  rightTruthyCall,
	}
	andOp.SetPos(pos)
	
	return andOp
}

// createTruthyCheck creates a truthiness check for different types
// Returns appropriate condition based on the expression type
func (t *TruthyAndTransform) createTruthyCheck(expr syntax.Expr, pos syntax.Pos) syntax.Expr {
	// For complex expressions that we can't easily determine the type for,
	// we'll just use a conservative approach: check if it's "truthy" by assuming it's non-zero
	
	switch e := expr.(type) {
	case *syntax.BasicLit:
		switch e.Kind {
		case syntax.IntLit:
			// Check if int != 0
			zero := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
			zero.SetPos(pos)
			neq := &syntax.Operation{Op: syntax.Neq, X: e, Y: zero}
			neq.SetPos(pos)
			return neq
		case syntax.StringLit:
			// Check if string != ""
			empty := &syntax.BasicLit{Kind: syntax.StringLit, Value: `""`}
			empty.SetPos(pos)
			neq := &syntax.Operation{Op: syntax.Neq, X: e, Y: empty}
			neq.SetPos(pos)
			return neq
		case syntax.FloatLit:
			// Check if float != 0.0
			zero := &syntax.BasicLit{Kind: syntax.FloatLit, Value: "0.0"}
			zero.SetPos(pos)
			neq := &syntax.Operation{Op: syntax.Neq, X: e, Y: zero}
			neq.SetPos(pos)
			return neq
		}
	case *syntax.Name:
		// For variables, we need to make assumptions about their types
		// Let's just assume they're integers for now (this is a limitation)
		zero := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
		zero.SetPos(pos)
		neq := &syntax.Operation{Op: syntax.Neq, X: e, Y: zero}
		neq.SetPos(pos)
		return neq
	}
	
	// For everything else, let's try the simple comparison 
	// This might fail for some types, but it's better than nil comparison
	zero := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
	zero.SetPos(pos)
	neq := &syntax.Operation{Op: syntax.Neq, X: expr, Y: zero}
	neq.SetPos(pos)
	return neq
}

// createSmartTruthyCheck creates a smarter truthiness check
// For variables/names, assume they might be structs and check != nil
func (t *TruthyAndTransform) createSmartTruthyCheck(expr syntax.Expr, pos syntax.Pos) syntax.Expr {
	switch e := expr.(type) {
	case *syntax.Name:
		// For variables, assume they might be structs/pointers and check != nil
		// This handles the common case better than != 0
		nilName := &syntax.Name{Value: "nil"}
		nilName.SetPos(pos)
		neq := &syntax.Operation{Op: syntax.Neq, X: e, Y: nilName}
		neq.SetPos(pos)
		return neq
	case *syntax.BasicLit:
		// For literals, use the appropriate zero comparison
		return t.createTruthyCheck(e, pos)
	default:
		// For complex expressions, try nil comparison (works for pointers/interfaces)
		nilName := &syntax.Name{Value: "nil"}
		nilName.SetPos(pos)
		neq := &syntax.Operation{Op: syntax.Neq, X: e, Y: nilName}
		neq.SetPos(pos)
		return neq
	}
}

// createBooleanTruthyCheck creates a boolean expression for truthiness
// This is more conservative and handles type mismatches better
func (t *TruthyAndTransform) createBooleanTruthyCheck(expr syntax.Expr, pos syntax.Pos) syntax.Expr {
	switch e := expr.(type) {
	case *syntax.Name:
		// For variable names, we don't know the type at syntax level
		// The safest approach is to just return the variable directly
		// If it's already boolean, it will work as-is
		// If it's not boolean, the type checker will handle the conversion
		e.SetPos(pos)  // Ensure position information is set
		return e
	case *syntax.SelectorExpr:
		// For field access like user.Name, assume it's a string and check != ""
		empty := &syntax.BasicLit{Kind: syntax.StringLit, Value: `""`}
		empty.SetPos(pos)
		neq := &syntax.Operation{Op: syntax.Neq, X: e, Y: empty}
		neq.SetPos(pos)
		return neq
	case *syntax.BasicLit:
		// Use the original truthiness check for literals
		return t.createTruthyCheck(e, pos)
	default:
		// For other expressions, try to be more conservative
		// Just return the expression as-is if it might already be boolean
		e.SetPos(pos)  // Ensure position information is set
		return e
	}
}

func init() {
	RegisterTransformer(&TruthyAndTransform{})
}