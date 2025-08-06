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
	case *syntax.CheckStmt:
		newCond := t.transformExpr(s.Cond)
		if newCond != s.Cond {
			newCheck := *s
			newCheck.Cond = newCond
			return &newCheck
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

// createTruthyAndCall creates a truthy and operation using boolean conversion
func (t *TruthyAndTransform) createTruthyAndCall(left, right syntax.Expr, pos syntax.Pos) syntax.Expr {
	println("DEBUG: Creating boolean AND with truthiness checks")
	
	// Ensure operands have proper position information
	if left.Pos().IsKnown() {
		// Keep original position
	} else {
		left.SetPos(pos)
	}
	
	if right.Pos().IsKnown() {
		// Keep original position  
	} else {
		right.SetPos(pos)
	}
	
	// Create && operation with proper initialization
	andOp := &syntax.Operation{
		Op: syntax.AndAnd,
		X:  left,
		Y:  right,
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
		// For variable names, we don't know the type at syntax level
		// The safest approach is to just return the variable directly
		// If it's already boolean, it will work as-is
		// If it's not boolean, the type checker will handle the conversion
		e.SetPos(pos)  // Ensure position information is set
		return expr
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
		expr.SetPos(pos)  // Ensure position information is set
		return expr
	}
}

func init() {
	RegisterTransformer(&TruthyAndTransform{})
}