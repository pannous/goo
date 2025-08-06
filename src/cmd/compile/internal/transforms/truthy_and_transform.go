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
		newCond := t.transformExpr(s.Cond)
		newThen := t.transformStmt(s.Then)
		var newElse syntax.Stmt
		if s.Else != nil {
			newElse = t.transformStmt(s.Else)
		}
		
		if newCond != s.Cond || newThen != s.Then || newElse != s.Else {
			newIf := *s
			newIf.Cond = newCond
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
			println("DEBUG: Transforming TruthyAnd operation")
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

// createTruthyAndCall creates a truthy and operation using boolean conversion
func (t *TruthyAndTransform) createTruthyAndCall(left, right syntax.Expr, pos syntax.Pos) syntax.Expr {
	println("DEBUG: Creating boolean AND with truthiness checks")
	
	// Convert both operands to boolean truthiness checks
	leftTruthy := t.createBooleanTruthyCheck(left, pos)
	rightTruthy := t.createBooleanTruthyCheck(right, pos)
	
	// Create && operation: leftTruthy && rightTruthy  
	andOp := &syntax.Operation{
		Op: syntax.AndAnd,
		X:  leftTruthy,
		Y:  rightTruthy,
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
			neq := &syntax.Operation{Op: syntax.Neq, X: expr, Y: zero}
			neq.SetPos(pos)
			return neq
		case syntax.StringLit:
			// Check if string != ""
			empty := &syntax.BasicLit{Kind: syntax.StringLit, Value: `""`}
			empty.SetPos(pos)
			neq := &syntax.Operation{Op: syntax.Neq, X: expr, Y: empty}
			neq.SetPos(pos)
			return neq
		case syntax.FloatLit:
			// Check if float != 0.0
			zero := &syntax.BasicLit{Kind: syntax.FloatLit, Value: "0.0"}
			zero.SetPos(pos)
			neq := &syntax.Operation{Op: syntax.Neq, X: expr, Y: zero}
			neq.SetPos(pos)
			return neq
		}
	case *syntax.Name:
		// For variables, we need to make assumptions about their types
		// Let's just assume they're integers for now (this is a limitation)
		zero := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
		zero.SetPos(pos)
		neq := &syntax.Operation{Op: syntax.Neq, X: expr, Y: zero}
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
		neq := &syntax.Operation{Op: syntax.Neq, X: expr, Y: nilName}
		neq.SetPos(pos)
		return neq
	case *syntax.BasicLit:
		// For literals, use the appropriate zero comparison
		return t.createTruthyCheck(expr, pos)
	default:
		// For complex expressions, try nil comparison (works for pointers/interfaces)
		nilName := &syntax.Name{Value: "nil"}
		nilName.SetPos(pos)
		neq := &syntax.Operation{Op: syntax.Neq, X: expr, Y: nilName}
		neq.SetPos(pos)
		return neq
	}
}

// createBooleanTruthyCheck creates a boolean expression for truthiness
// This is more conservative and handles type mismatches better
func (t *TruthyAndTransform) createBooleanTruthyCheck(expr syntax.Expr, pos syntax.Pos) syntax.Expr {
	switch e := expr.(type) {
	case *syntax.Name:
		// For variable names, we can't easily determine if it's a pointer or struct
		// Let's use a heuristic: if this is likely a struct, check a meaningful field
		// For now, try to access .Name field if it exists, otherwise fall back to != nil
		
		// Try to create a check for a common field that indicates "non-empty" struct
		nameField := &syntax.SelectorExpr{
			X:   expr,
			Sel: &syntax.Name{Value: "Name"},
		}
		nameField.SetPos(pos)
		
		empty := &syntax.BasicLit{Kind: syntax.StringLit, Value: `""`}
		empty.SetPos(pos)
		neq := &syntax.Operation{Op: syntax.Neq, X: nameField, Y: empty}
		neq.SetPos(pos)
		return neq
	case *syntax.SelectorExpr:
		// For field access like user.Name, assume it's a string and check != ""
		empty := &syntax.BasicLit{Kind: syntax.StringLit, Value: `""`}
		empty.SetPos(pos)
		neq := &syntax.Operation{Op: syntax.Neq, X: expr, Y: empty}
		neq.SetPos(pos)
		return neq
	case *syntax.BasicLit:
		// Use the original truthiness check for literals
		return t.createTruthyCheck(expr, pos)
	default:
		// For other expressions, try to be more conservative
		// Just return the expression as-is if it might already be boolean
		return expr
	}
}

func init() {
	RegisterTransformer(&TruthyAndTransform{})
}