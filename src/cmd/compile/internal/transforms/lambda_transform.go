// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build transforms

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
	case *syntax.CheckStmt:
		if newCond := t.transformExpr(s.Cond); newCond != s.Cond {
			newStmt := *s
			newStmt.Cond = newCond
			return &newStmt
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

	// Infer return type from lambda body
	returnType := &syntax.Name{Value: t.inferReturnType(lambda.Body)}
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

// inferReturnType analyzes the lambda body expression to determine the return type
func (t *LambdaTransform) inferReturnType(expr syntax.Expr) string {
	if expr == nil {
		return "any"
	}

	switch e := expr.(type) {
	case *syntax.Operation:
		// Handle comparison operations
		switch e.Op {
		case syntax.Eql, syntax.Neq, syntax.Lss, syntax.Leq, syntax.Gtr, syntax.Geq:
			return "bool" // Comparison operators return bool
		case syntax.Land, syntax.Lor:
			return "bool" // Logical operators return bool
		case syntax.Add, syntax.Sub, syntax.Mul, syntax.Div, syntax.Rem:
			// Arithmetic operations - infer from operands
			return t.inferArithmeticType(e.X, e.Y)
		default:
			// For other operations, try to infer from left operand
			return t.inferExpressionType(e.X)
		}
	case *syntax.BasicLit:
		return t.inferLiteralType(e)
	case *syntax.Name:
		// For variable references, we can't easily infer the type
		// In a full implementation, we'd need a symbol table
		return "any"
	case *syntax.CallExpr:
		// Function calls - would need more context to determine return type
		return "any"
	default:
		return "any"
	}
}

// inferLiteralType determines the type of a literal expression
func (t *LambdaTransform) inferLiteralType(lit *syntax.BasicLit) string {
	switch lit.Kind {
	case syntax.IntLit:
		return "int"
	case syntax.FloatLit:
		return "float64"
	case syntax.StringLit:
		return "string"
	case syntax.CharLit:
		return "rune"
	default:
		return "any"
	}
}

// inferArithmeticType determines the result type of arithmetic operations
func (t *LambdaTransform) inferArithmeticType(left, right syntax.Expr) string {
	leftType := t.inferExpressionType(left)
	rightType := t.inferExpressionType(right)
	
	// Simple rule: if both are int, result is int
	// if either is float, result is float
	if leftType == "float64" || rightType == "float64" {
		return "float64"
	}
	if leftType == "int" && rightType == "int" {
		return "int"
	}
	return "any"
}

// inferExpressionType determines the type of an expression
func (t *LambdaTransform) inferExpressionType(expr syntax.Expr) string {
	if expr == nil {
		return "any"
	}
	
	switch e := expr.(type) {
	case *syntax.BasicLit:
		return t.inferLiteralType(e)
	case *syntax.Operation:
		switch e.Op {
		case syntax.Rem: // % operator
			// Modulo operation typically returns int
			return "int"
		case syntax.Add, syntax.Sub, syntax.Mul, syntax.Div:
			return t.inferArithmeticType(e.X, e.Y)
		default:
			return "any"
		}
	case *syntax.Name:
		// Variable reference - would need symbol table for proper inference
		return "any"
	default:
		return "any"
	}
}

func init() {
	RegisterTransformer(&LambdaTransform{})
}
