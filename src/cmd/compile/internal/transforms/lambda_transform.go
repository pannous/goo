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

func (t *LambdaTransform) Name() string {
	return "lambda_transform"
}

func (t *LambdaTransform) Priority() int {
	return 300 // Very low priority - run after list methods finish fixing parameter types
}

// NodeTransformer interface implementation
func (t *LambdaTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
	// Handle ALL LambdaExpr nodes as fallback (high priority 300 ensures other transforms run first)
	// This prevents any lambda from reaching the Go compiler backend untransformed
	if _, ok := node.(*syntax.LambdaExpr); ok {
		return true // Always handle lambdas as fallback to prevent compiler panics
	}
	return false
}

func (t *LambdaTransform) TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node {
	if lambda, ok := node.(*syntax.LambdaExpr); ok {
		return t.convertLambdaToFuncLit(lambda)
	}
	return nil
}

func (t *LambdaTransform) PostProcess(file *syntax.File, ctx *TransformContext) bool {
	// No post-processing needed for lambda transform
	return false
}

// Legacy Transform method for backward compatibility - not used in new architecture
func (t *LambdaTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	// This method is kept for interface compatibility but not used
	// The new NodeTransformer interface methods are used instead
	return false
}

// convertLambdaToFuncLit converts a lambda expression to a function literal
func (t *LambdaTransform) convertLambdaToFuncLit(lambda *syntax.LambdaExpr) *syntax.FuncLit {
	pos := lambda.Pos()
	
	// Create function type with parameters and return type
	// For now, assume most lambdas return bool (common case for filter/where operations)
	boolType := &syntax.Name{Value: "bool"}
	boolType.SetPos(lambda.Body.Pos()) // Copy position from lambda body
	
	resultField := &syntax.Field{Type: boolType}
	resultField.SetPos(lambda.Body.Pos()) // Copy position from lambda body
	
	funcType := &syntax.FuncType{
		ParamList: lambda.ParamList,
		ResultList: []*syntax.Field{resultField},
	}
	funcType.SetPos(pos)
	
	// Create function body - wrap the expression in a return statement
	returnStmt := &syntax.ReturnStmt{
		Results: lambda.Body,
	}
	returnStmt.SetPos(lambda.Body.Pos())
	
	body := &syntax.BlockStmt{
		List: []syntax.Stmt{returnStmt},
	}
	body.SetPos(pos)
	
	// Create the function literal
	funcLit := &syntax.FuncLit{
		Type: funcType,
		Body: body,
	}
	funcLit.SetPos(pos)
	
	return funcLit
}

// isMethodCallArgument checks if this lambda is an argument to a method call
// that might need special parameter type handling by list_methods_transform
func (t *LambdaTransform) isMethodCallArgument(lambda *syntax.LambdaExpr) bool {
	// This is a simple heuristic - in practice, we can't easily determine parent context
	// from the centralized visitor. For now, assume lambdas with simple parameter
	// names like 'u', 'x', 'item' are likely method call arguments
	if lambda.ParamList != nil && len(lambda.ParamList) > 0 {
		param := lambda.ParamList[0]
		if param.Name != nil {
			paramName := param.Name.Value
			// Common lambda parameter names used in method calls
			methodArgNames := []string{"u", "x", "item", "elem", "e", "obj", "o"}
			for _, name := range methodArgNames {
				if paramName == name {
					return true
				}
			}
		}
	}
	return false
}

func init() {
	RegisterTransformer(&LambdaTransform{})
}