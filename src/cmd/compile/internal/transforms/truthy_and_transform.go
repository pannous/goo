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

// createTruthyAndCall creates a truthy and operation using a type-safe dummy operation
// Transforms: x and y --> truthyAndOp(x, y)  where truthyAndOp is a simple function call
func (t *TruthyAndTransform) createTruthyAndCall(left, right syntax.Expr, pos syntax.Pos) syntax.Expr {
	println("DEBUG: Creating truthyAndOp function call")
	// Create truthyAndOp function call: truthyAndOp(x, y)
	truthyAndOpName := &syntax.Name{Value: "truthyAndOp"}
	truthyAndOpName.SetPos(pos)
	
	call := &syntax.CallExpr{
		Fun:     truthyAndOpName,
		ArgList: []syntax.Expr{left, right},
	}
	call.SetPos(pos)
	
	return call
}

func init() {
	RegisterTransformer(&TruthyAndTransform{})
}