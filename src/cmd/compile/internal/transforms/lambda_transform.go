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

func (t *LambdaTransform) Priority() int {
	return 200 // Low priority - run after list methods and other transforms
}

func (t *LambdaTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &lambdaVisitor{transform: t, ctx: ctx}
	
	// Use the general visitor pattern to walk all nodes
	syntax.Walk(file, visitor)
	
	return visitor.changed
}

// Visit implements syntax.Visitor
func (v *lambdaVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}
	
	// Look for nodes that contain lambda expressions we can replace
	switch n := node.(type) {
	case *syntax.ExprStmt:
		if lambda, ok := n.X.(*syntax.LambdaExpr); ok {
			if newExpr := v.transform.convertLambdaToFuncLit(lambda); newExpr != nil {
				n.X = newExpr
				v.changed = true
			}
		}
	case *syntax.AssignStmt:
		if lambda, ok := n.Rhs.(*syntax.LambdaExpr); ok {
			if newExpr := v.transform.convertLambdaToFuncLit(lambda); newExpr != nil {
				n.Rhs = newExpr
				v.changed = true
			}
		}
	case *syntax.VarDecl:
		if n.Values != nil {
			if lambda, ok := n.Values.(*syntax.LambdaExpr); ok {
				if newExpr := v.transform.convertLambdaToFuncLit(lambda); newExpr != nil {
					n.Values = newExpr
					v.changed = true
				}
			}
		}
	case *syntax.CallExpr:
		// Handle lambda expressions in function arguments
		for i, arg := range n.ArgList {
			if lambda, ok := arg.(*syntax.LambdaExpr); ok {
				if newExpr := v.transform.convertLambdaToFuncLit(lambda); newExpr != nil {
					n.ArgList[i] = newExpr
					v.changed = true
				}
			}
		}
	case *syntax.ReturnStmt:
		if n.Results != nil {
			if lambda, ok := n.Results.(*syntax.LambdaExpr); ok {
				if newExpr := v.transform.convertLambdaToFuncLit(lambda); newExpr != nil {
					n.Results = newExpr
					v.changed = true
				}
			}
		}
	case *syntax.CheckStmt:
		if lambda, ok := n.Cond.(*syntax.LambdaExpr); ok {
			if newExpr := v.transform.convertLambdaToFuncLit(lambda); newExpr != nil {
				n.Cond = newExpr
				v.changed = true
			}
		}
	case *syntax.Operation:
		if lambda, ok := n.X.(*syntax.LambdaExpr); ok {
			if newExpr := v.transform.convertLambdaToFuncLit(lambda); newExpr != nil {
				n.X = newExpr
				v.changed = true
			}
		}
		if lambda, ok := n.Y.(*syntax.LambdaExpr); ok {
			if newExpr := v.transform.convertLambdaToFuncLit(lambda); newExpr != nil {
				n.Y = newExpr
				v.changed = true
			}
		}
	}
	
	// Continue visiting child nodes
	return v
}

func (t *LambdaTransform) convertLambdaToFuncLit(lambda *syntax.LambdaExpr) *syntax.FuncLit {
	// Check if lambda body is nil
	if lambda.Body == nil {
		return nil
	}

	// Infer return type based on lambda body expression
	returnTypeName := t.inferReturnType(lambda.Body)
	returnType := &syntax.Name{Value: returnTypeName}
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

// inferReturnType analyzes the lambda body expression to infer the return type
func (t *LambdaTransform) inferReturnType(expr syntax.Expr) string {
	switch e := expr.(type) {
	case *syntax.Operation:
		switch e.Op {
		// Comparison operators return bool
		case syntax.Eql, syntax.Neq, syntax.Lss, syntax.Leq, syntax.Gtr, syntax.Geq:
			return "bool"
		// Logical operators return bool
		case syntax.AndAnd, syntax.OrOr:
			return "bool"
		// Arithmetic operators typically return int (simplified)
		case syntax.Add, syntax.Sub, syntax.Mul, syntax.Div, syntax.Rem:
			return "int"
		// Bitwise operators return int
		case syntax.And, syntax.Or, syntax.Xor, syntax.Shl, syntax.Shr, syntax.AndNot:
			return "int"
		}
	case *syntax.BasicLit:
		switch e.Kind {
		case syntax.IntLit:
			return "int"
		case syntax.FloatLit:
			return "float64"
		case syntax.StringLit:
			return "string"
		case syntax.RuneLit:
			return "rune"
		}
	case *syntax.Name:
		// For variable references, we can't easily determine the type without more context
		// Default to int for now
		return "int"
	case *syntax.CallExpr:
		// Function call - would need more complex analysis
		return "int"
	}
	
	// Default to int if we can't determine the type
	return "int"
}

func init() {
	RegisterTransformer(&LambdaTransform{})
}
