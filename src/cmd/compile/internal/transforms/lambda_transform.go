// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transforms

import (
	"cmd/compile/internal/syntax"
)

// LambdaTransform converts lambda expressions to function literals
type LambdaTransform struct{}

type lambdaVisitor struct {
	transform *LambdaTransform
	ctx       *TransformContext
	changed   bool
}

func (t *LambdaTransform) Name() string {
	return "lambda_transform"
}

func (t *LambdaTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	changed := false
	
	// Transform all declarations
	for i, decl := range file.DeclList {
		if newDecl := t.transformDecl(decl); newDecl != decl {
			file.DeclList[i] = newDecl
			changed = true
		}
	}
	
	return changed
}

func (t *LambdaTransform) transformDecl(decl syntax.Decl) syntax.Decl {
	switch d := decl.(type) {
	case *syntax.FuncDecl:
		if newBody := t.transformStmt(d.Body); newBody != d.Body {
			newDecl := *d
			if blockStmt, ok := newBody.(*syntax.BlockStmt); ok {
				newDecl.Body = blockStmt
			}
			return &newDecl
		}
	case *syntax.VarDecl:
		if d.Values != nil {
			if newValues := t.transformExpr(d.Values); newValues != d.Values {
				newDecl := *d
				newDecl.Values = newValues
				return &newDecl
			}
		}
	}
	return decl
}

func (t *LambdaTransform) transformStmt(stmt syntax.Stmt) syntax.Stmt {
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
	case *syntax.ExprStmt:
		if newExpr := t.transformExpr(s.X); newExpr != s.X {
			newStmt := *s
			newStmt.X = newExpr
			return &newStmt
		}
	case *syntax.AssignStmt:
		lhsChanged := false
		rhsChanged := false
		newLhs := t.transformExpr(s.Lhs)
		newRhs := t.transformExpr(s.Rhs)
		if newLhs != s.Lhs {
			lhsChanged = true
		}
		if newRhs != s.Rhs {
			rhsChanged = true
		}
		if lhsChanged || rhsChanged {
			newStmt := *s
			newStmt.Lhs = newLhs
			newStmt.Rhs = newRhs
			return &newStmt
		}
	case *syntax.ReturnStmt:
		if s.Results != nil {
			if newResults := t.transformExpr(s.Results); newResults != s.Results {
				newStmt := *s
				newStmt.Results = newResults
				return &newStmt
			}
		}
	}
	return stmt
}

func (t *LambdaTransform) transformExpr(expr syntax.Expr) syntax.Expr {
	if expr == nil {
		return nil
	}
	
	switch e := expr.(type) {
	case *syntax.LambdaExpr:
		// This is what we're looking for! Convert to FuncLit
		return t.convertLambdaToFuncLit(e)
	case *syntax.CallExpr:
		funChanged := false
		argsChanged := false
		newFun := t.transformExpr(e.Fun)
		if newFun != e.Fun {
			funChanged = true
		}
		var newArgList []syntax.Expr
		if e.ArgList != nil {
			newArgList = make([]syntax.Expr, len(e.ArgList))
			for i, arg := range e.ArgList {
				newArg := t.transformExpr(arg)
				newArgList[i] = newArg
				if newArg != arg {
					argsChanged = true
				}
			}
		}
		if funChanged || argsChanged {
			newCall := *e
			newCall.Fun = newFun
			newCall.ArgList = newArgList
			return &newCall
		}
	case *syntax.Operation:
		xChanged := false
		yChanged := false
		newX := t.transformExpr(e.X)
		if newX != e.X {
			xChanged = true
		}
		var newY syntax.Expr
		if e.Y != nil {
			newY = t.transformExpr(e.Y)
			if newY != e.Y {
				yChanged = true
			}
		}
		if xChanged || yChanged {
			newOp := *e
			newOp.X = newX
			newOp.Y = newY
			return &newOp
		}
	}
	return expr
}

func (t *LambdaTransform) convertLambdaToFuncLit(lambda *syntax.LambdaExpr) *syntax.FuncLit {
	// Check if lambda body is nil  
	if lambda.Body == nil {
		return nil
	}
	
	// Create a return type (for now, use 'int' to match the parameter)
	// TODO: Implement proper type inference
	returnType := &syntax.Name{Value: "int"}
	returnType.SetPos(lambda.Pos())
	
	returnField := &syntax.Field{Type: returnType}
	returnField.SetPos(lambda.Pos())
	
	// Create function type
	funcType := &syntax.FuncType{
		ParamList:  lambda.ParamList,
		ResultList: []*syntax.Field{returnField},
	}
	// Make sure to set the position
	funcType.SetPos(lambda.Pos())
	
	// Create return statement with lambda body
	returnStmt := &syntax.ReturnStmt{
		Results: lambda.Body,
	}
	returnStmt.SetPos(lambda.Body.Pos())
	
	// Create block statement containing the return
	blockStmt := &syntax.BlockStmt{
		List: []syntax.Stmt{returnStmt},
	}
	blockStmt.SetPos(lambda.Pos())
	
	// Create function literal
	funcLit := &syntax.FuncLit{
		Type: funcType,
		Body: blockStmt,
	}
	funcLit.SetPos(lambda.Pos())
	
	return funcLit
}

func init() {
	RegisterTransformer(&LambdaTransform{})
}