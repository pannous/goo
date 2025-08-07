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
	return 200 // Low priority - run after list methods and other transforms
}

// NodeTransformer interface implementation
func (t *LambdaTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
	// Only handle LambdaExpr nodes directly
	if _, ok := node.(*syntax.LambdaExpr); ok {
		return true
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
	
	// Create function type with parameters
	funcType := &syntax.FuncType{
		ParamList: lambda.ParamList,
		// Result type will be inferred from the body expression
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

func init() {
	RegisterTransformer(&LambdaTransform{})
}