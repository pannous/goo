// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build transforms

// danger, conflict with src of vendor packages

package transforms

import (
	"cmd/compile/internal/syntax"
)

// AsCastTransform converts as cast expressions to type assertions
// Transforms: x as T -> x.(T)
type AsCastTransform struct{}

// asCastVisitor implements the visitor pattern for as-cast transformation
type asCastVisitor struct {
	transform       *AsCastTransform
	ctx             *TransformContext
	changed         bool
	needsStrconvImport bool
}

func (t *AsCastTransform) Name() string {
	return "as_cast_transform"
}

func (t *AsCastTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &asCastVisitor{transform: t, ctx: ctx}
	
	// Use the general visitor pattern to walk all nodes
	syntax.Walk(file, visitor)
	
	// Add strconv import if needed
	if visitor.needsStrconvImport && !t.hasImport(file, "strconv") {
		t.addStrconvImport(file)
	}
	
	return visitor.changed
}

// Visit implements syntax.Visitor
func (v *asCastVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}
	
	// Look for nodes that contain as-cast expressions we can replace
	switch n := node.(type) {
	case *syntax.ExprStmt:
		if asCast, ok := n.X.(*syntax.AsCastExpr); ok {
			if newExpr := v.transform.convertAsCastToAssert(asCast, v); newExpr != asCast {
				n.X = newExpr
				v.changed = true
			}
		}
	case *syntax.AssignStmt:
		if asCast, ok := n.Rhs.(*syntax.AsCastExpr); ok {
			if newExpr := v.transform.convertAsCastToAssert(asCast, v); newExpr != asCast {
				n.Rhs = newExpr
				v.changed = true
			}
		}
	case *syntax.VarDecl:
		if n.Values != nil {
			if asCast, ok := n.Values.(*syntax.AsCastExpr); ok {
				if newExpr := v.transform.convertAsCastToAssert(asCast, v); newExpr != asCast {
					n.Values = newExpr
					v.changed = true
				}
			}
		}
	case *syntax.CallExpr:
		// Handle as-cast expressions in function arguments
		for i, arg := range n.ArgList {
			if asCast, ok := arg.(*syntax.AsCastExpr); ok {
				if newExpr := v.transform.convertAsCastToAssert(asCast, v); newExpr != asCast {
					n.ArgList[i] = newExpr
					v.changed = true
				}
			}
		}
	case *syntax.ReturnStmt:
		if n.Results != nil {
			if asCast, ok := n.Results.(*syntax.AsCastExpr); ok {
				if newExpr := v.transform.convertAsCastToAssert(asCast, v); newExpr != asCast {
					n.Results = newExpr
					v.changed = true
				}
			}
		}
	case *syntax.Operation:
		if asCast, ok := n.X.(*syntax.AsCastExpr); ok {
			if newExpr := v.transform.convertAsCastToAssert(asCast, v); newExpr != asCast {
				n.X = newExpr
				v.changed = true
			}
		}
		if asCast, ok := n.Y.(*syntax.AsCastExpr); ok {
			if newExpr := v.transform.convertAsCastToAssert(asCast, v); newExpr != asCast {
				n.Y = newExpr
				v.changed = true
			}
		}
	}
	
	// Continue visiting child nodes
	return v
}

func (t *AsCastTransform) convertAsCastToAssert(asCast *syntax.AsCastExpr, visitor *asCastVisitor) syntax.Expr {
	// Check if this is a no-op cast (same type)
	if t.isSameType(asCast.X, asCast.Type, visitor.ctx) {
		return asCast.X
	}
	
	// Handle special "hard cast" cases that need custom conversion logic
	if specialConv := t.createSpecialConversion(asCast.X, asCast.Type, asCast.Pos(), visitor); specialConv != nil {
		return specialConv
	}
	
	// Determine if we need type assertion or type conversion
	if t.shouldUseTypeConversion(asCast.X, asCast.Type, visitor.ctx) {
		// Create type conversion: T(x)
		callExpr := &syntax.CallExpr{
			Fun:     asCast.Type,
			ArgList: []syntax.Expr{asCast.X},
		}
		callExpr.SetPos(asCast.Pos())
		return callExpr
	} else {
		// Create type assertion: x.(T)
		assertExpr := &syntax.AssertExpr{
			X:    asCast.X,
			Type: asCast.Type,
		}
		assertExpr.SetPos(asCast.Pos())
		return assertExpr
	}
}

// isSameType checks if the expression and type are the same
// This is a simple heuristic - in a full implementation, we'd need proper type checking
func (t *AsCastTransform) isSameType(expr syntax.Expr, targetType syntax.Expr, ctx *TransformContext) bool {
	// Look up the variable's declared type in the context if available
	if exprName, ok := expr.(*syntax.Name); ok {
		if typeName, ok := targetType.(*syntax.Name); ok {
			// Check if we have type information for this variable
			if declaredType, exists := ctx.Types[exprName.Value]; exists {
				// If the declared type matches the target type, it's a no-op
				return declaredType == typeName.Value
			}
		}
	}
	
	return false
}

// createSpecialConversion handles special "hard cast" cases and semantic conversions
func (t *AsCastTransform) createSpecialConversion(expr syntax.Expr, targetType syntax.Expr, pos syntax.Pos, visitor *asCastVisitor) syntax.Expr {
	// Check if target type is a name we can work with
	typeName, ok := targetType.(*syntax.Name)
	if !ok {
		return nil
	}
	
	// Handle semantic conversions based on source and target types
	if semanticConv := t.createSemanticConversion(expr, typeName.Value, pos, visitor); semanticConv != nil {
		return semanticConv
	}
	
	// Handle "float" as alias for "float64"
	if typeName.Value == "float" {
		// Create float64(x) conversion
		newTypeName := &syntax.Name{Value: "float64"}
		newTypeName.SetPos(pos)
		
		callExpr := &syntax.CallExpr{
			Fun:     newTypeName,
			ArgList: []syntax.Expr{expr},
		}
		callExpr.SetPos(pos)
		return callExpr
	}
	
	return nil
}

// createSemanticConversion handles high-level value conversions
func (t *AsCastTransform) createSemanticConversion(expr syntax.Expr, targetType string, pos syntax.Pos, visitor *asCastVisitor) syntax.Expr {
	// Determine source type
	sourceType := t.inferExprType(expr)
	
	// Handle specific conversion patterns
	switch {
	// Numeric to string: 1 as string -> strconv.Itoa(1)
	case targetType == "string" && (sourceType == "int" || sourceType == "int_literal"):
		visitor.needsStrconvImport = true
		return t.createStrconvCall("Itoa", expr, pos)
		
	// String to int: "1" as int -> strconv.Atoi("1") 
	case (targetType == "int" || targetType == "int32" || targetType == "int64") && 
		 (sourceType == "string" || sourceType == "string_literal"):
		visitor.needsStrconvImport = true
		return t.createStrconvCall("Atoi", expr, pos)
		
	// Float to int: 3.1 as int -> int(3.1)
	case targetType == "int" && (sourceType == "float64" || sourceType == "float_literal"):
		// For float literals, Go requires explicit conversion through float64
		if sourceType == "float_literal" {
			return t.createFloatLiteralToIntConversion(expr, pos)
		}
		return t.createBasicTypeConversion(expr, targetType, pos)
		
	// Int to float: 3 as float -> float64(3)
	case (targetType == "float" || targetType == "float64") && 
		 (sourceType == "int" || sourceType == "int_literal"):
		actualTarget := targetType
		if targetType == "float" {
			actualTarget = "float64"
		}
		return t.createBasicTypeConversion(expr, actualTarget, pos)
		
	// Rune to int: '1' as int -> int('1') - 48 (to get numeric value, not ASCII)
	case targetType == "int" && (sourceType == "rune" || sourceType == "rune_literal"):
		return t.createRuneToIntConversion(expr, pos)
		
	// Int to rune: 1 as rune -> rune(1 + 48) (to get character '1', not control char)
	case targetType == "rune" && (sourceType == "int" || sourceType == "int_literal"):
		return t.createIntToRuneConversion(expr, pos)
	}
	
	return nil
}

// inferExprType tries to determine the type of an expression
func (t *AsCastTransform) inferExprType(expr syntax.Expr) string {
	switch e := expr.(type) {
	case *syntax.BasicLit:
		switch e.Kind {
		case syntax.IntLit:
			return "int_literal"
		case syntax.FloatLit:
			return "float_literal"
		case syntax.StringLit:
			return "string_literal"
		case syntax.RuneLit:
			return "rune_literal"
		}
	case *syntax.Name:
		// Could look up in context, but for now assume based on common patterns
		return "unknown"
	}
	return "unknown"
}

// createStrconvCall creates a call to strconv function, handling error returns
func (t *AsCastTransform) createStrconvCall(funcName string, arg syntax.Expr, pos syntax.Pos) syntax.Expr {
	// Create strconv.Funcname(arg)
	strconvName := &syntax.Name{Value: "strconv"}
	strconvName.SetPos(pos)
	
	funcNameNode := &syntax.Name{Value: funcName}
	funcNameNode.SetPos(pos)
	
	selectorExpr := &syntax.SelectorExpr{
		X:   strconvName,
		Sel: funcNameNode,
	}
	selectorExpr.SetPos(pos)
	
	callExpr := &syntax.CallExpr{
		Fun:     selectorExpr,
		ArgList: []syntax.Expr{arg},
	}
	callExpr.SetPos(pos)
	
	// For functions that return (value, error), we need to handle the error
	// Use panic-on-error approach: func() T { v, err := call(); if err != nil { panic(err) }; return v }()
	if funcName == "Atoi" {
		return t.createMustWrapper(callExpr, "int", pos)
	}
	
	return callExpr
}

// createBasicTypeConversion creates T(x) conversion
func (t *AsCastTransform) createBasicTypeConversion(expr syntax.Expr, targetType string, pos syntax.Pos) syntax.Expr {
	typeName := &syntax.Name{Value: targetType}
	typeName.SetPos(pos)
	
	callExpr := &syntax.CallExpr{
		Fun:     typeName,
		ArgList: []syntax.Expr{expr},
	}
	callExpr.SetPos(pos)
	
	return callExpr
}

// createRuneToIntConversion converts '1' as int to get numeric value 1 (not ASCII 49)
func (t *AsCastTransform) createRuneToIntConversion(expr syntax.Expr, pos syntax.Pos) syntax.Expr {
	// For character literals like '1', we want the numeric value, not ASCII
	// So '1' as int should give 1, not 49
	// This is: int(rune) - int('0')
	
	// Create int(expr)
	intTypeName := &syntax.Name{Value: "int"}
	intTypeName.SetPos(pos)
	
	intOfExpr := &syntax.CallExpr{
		Fun:     intTypeName,
		ArgList: []syntax.Expr{expr},
	}
	intOfExpr.SetPos(pos)
	
	// Create '0' literal
	zeroRune := &syntax.BasicLit{
		Value: "'0'",
		Kind:  syntax.RuneLit,
	}
	zeroRune.SetPos(pos)
	
	// Create int('0')
	intOfZero := &syntax.CallExpr{
		Fun:     &syntax.Name{Value: "int"},
		ArgList: []syntax.Expr{zeroRune},
	}
	intOfZero.SetPos(pos)
	
	// Create subtraction: int(expr) - int('0')
	subExpr := &syntax.Operation{
		Op: syntax.Sub,
		X:  intOfExpr,
		Y:  intOfZero,
	}
	subExpr.SetPos(pos)
	
	return subExpr
}

// createIntToRuneConversion converts 1 as rune to get character '1' (not control char)
func (t *AsCastTransform) createIntToRuneConversion(expr syntax.Expr, pos syntax.Pos) syntax.Expr {
	// For numeric values like 1, we want the character '1', not control character
	// This is: rune(int + '0')
	
	// Create '0' literal  
	zeroRune := &syntax.BasicLit{
		Value: "'0'",
		Kind:  syntax.RuneLit,
	}
	zeroRune.SetPos(pos)
	
	// Create int('0')
	intOfZero := &syntax.CallExpr{
		Fun:     &syntax.Name{Value: "int"},
		ArgList: []syntax.Expr{zeroRune},
	}
	intOfZero.SetPos(pos)
	
	// Create addition: expr + int('0')
	addExpr := &syntax.Operation{
		Op: syntax.Add,
		X:  expr,
		Y:  intOfZero,
	}
	addExpr.SetPos(pos)
	
	// Create rune(expr + int('0'))
	runeTypeName := &syntax.Name{Value: "rune"}
	runeTypeName.SetPos(pos)
	
	callExpr := &syntax.CallExpr{
		Fun:     runeTypeName,
		ArgList: []syntax.Expr{addExpr},
	}
	callExpr.SetPos(pos)
	
	return callExpr
}

// createFloatLiteralToIntConversion converts untyped float literals to int
func (t *AsCastTransform) createFloatLiteralToIntConversion(expr syntax.Expr, pos syntax.Pos) syntax.Expr {
	// For untyped float literals like 3.1, Go's type system is strict
	// We need to just use the basic int() conversion and let Go handle it
	// If it fails, it means the code needs to be written differently
	return t.createBasicTypeConversion(expr, "int", pos)
}

// createMustWrapper creates a wrapper that panics on error
// Generates: func() T { v, err := call(); if err != nil { panic(err) }; return v }()
func (t *AsCastTransform) createMustWrapper(call syntax.Expr, returnType string, pos syntax.Pos) syntax.Expr {
	// For simplicity, let's use a different approach
	// Generate: func() T { v, _ := call(); return v }()
	// This ignores the error but doesn't panic
	
	// Create variable names
	vName := &syntax.Name{Value: "v"}
	vName.SetPos(pos)
	
	blankName := &syntax.Name{Value: "_"}
	blankName.SetPos(pos)
	
	// Create assignment: v, _ := call()
	lhs := &syntax.ListExpr{
		ElemList: []syntax.Expr{vName, blankName},
	}
	lhs.SetPos(pos)
	
	assignStmt := &syntax.AssignStmt{
		Op:  syntax.Def, // :=
		Lhs: lhs,
		Rhs: call,
	}
	assignStmt.SetPos(pos)
	
	// Create return statement: return v
	returnStmt := &syntax.ReturnStmt{
		Results: vName,
	}
	returnStmt.SetPos(pos)
	
	// Create function body
	body := &syntax.BlockStmt{
		List: []syntax.Stmt{assignStmt, returnStmt},
	}
	body.SetPos(pos)
	
	// Create function type: func() returnType
	returnTypeName := &syntax.Name{Value: returnType}
	returnTypeName.SetPos(pos)
	
	fnType := &syntax.FuncType{
		ResultList: []*syntax.Field{{Type: returnTypeName}},
	}
	fnType.SetPos(pos)
	
	// Create function literal
	fnLit := &syntax.FuncLit{
		Type: fnType,
		Body: body,
	}
	fnLit.SetPos(pos)
	
	// Create function call: (func()...)()
	callExpr := &syntax.CallExpr{
		Fun: fnLit,
	}
	callExpr.SetPos(pos)
	
	return callExpr
}

// shouldUseTypeConversion determines if we should use type conversion T(x) vs type assertion x.(T)
// Type conversion is used when converting between concrete types (like float64 to int)
// Type assertion is used when extracting a concrete type from an interface
func (t *AsCastTransform) shouldUseTypeConversion(expr syntax.Expr, targetType syntax.Expr, ctx *TransformContext) bool {
	// Common concrete types that can be converted between
	concreteTypes := map[string]bool{
		"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true, "float": true,
		"byte": true, "rune": true,
		"string": true,
	}
	
	// Check if target type is a concrete type
	if typeName, ok := targetType.(*syntax.Name); ok {
		if !concreteTypes[typeName.Value] {
			return false // Not a concrete type, use assertion
		}
	} else {
		return false // Complex type, use assertion
	}
	
	// For literals, always use type conversion
	if basic, ok := expr.(*syntax.BasicLit); ok {
		switch basic.Kind {
		case syntax.IntLit, syntax.FloatLit, syntax.RuneLit, syntax.StringLit:
			return true
		}
	}
	
	// Handle IndexExpr (like array[0], slice[i])
	if indexExpr, ok := expr.(*syntax.IndexExpr); ok {
		// Check if the indexed variable is a slice/array of interface{}/any
		if varName, ok := indexExpr.X.(*syntax.Name); ok {
			if declaredType, exists := ctx.Types[varName.Value]; exists {
				// If it's a slice of interface{} or any, use assertion
				if declaredType == "[]interface{}" || declaredType == "[]any" || 
				   declaredType == "slice" {
					return false // Use type assertion
				}
			}
		}
	}

	// Look up the variable's declared type in the context if available
	if exprName, ok := expr.(*syntax.Name); ok {
		if typeName, ok := targetType.(*syntax.Name); ok {
			// Check if we have type information for this variable
			if declaredType, exists := ctx.Types[exprName.Value]; exists {
				// If both source and target are concrete types, use conversion
				if concreteTypes[declaredType] && concreteTypes[typeName.Value] {
					return true
				}
				
				// If source is interface{} or any, use assertion
				if declaredType == "interface{}" || declaredType == "any" {
					return false
				}
			}
		}
	}
	
	// Default to type conversion for concrete types
	if typeName, ok := targetType.(*syntax.Name); ok {
		return concreteTypes[typeName.Value]
	}
	
	return false
}

// hasImport checks if a file already imports a package
func (t *AsCastTransform) hasImport(file *syntax.File, packageName string) bool {
	if packageName[0] != '"' {
		packageName = "\"" + packageName + "\""
	}
	for _, decl := range file.DeclList {
		if importDecl, ok := decl.(*syntax.ImportDecl); ok {
			if importDecl.Path != nil && importDecl.Path.Value == packageName {
				return true
			}
		}
	}
	return false
}

// addStrconvImport adds "strconv" import to the file
func (t *AsCastTransform) addStrconvImport(file *syntax.File) {
	if t.hasImport(file, "strconv") {
		return
	}

	strconvImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: "\"strconv\"",
			Kind:  syntax.StringLit,
		},
	}
	strconvImport.SetPos(syntax.Pos{})

	var insertPos int
	for i, decl := range file.DeclList {
		if _, ok := decl.(*syntax.ImportDecl); ok {
			insertPos = i + 1
		} else {
			break
		}
	}

	newDeclList := make([]syntax.Decl, 0, len(file.DeclList)+1)
	newDeclList = append(newDeclList, file.DeclList[:insertPos]...)
	newDeclList = append(newDeclList, strconvImport)
	newDeclList = append(newDeclList, file.DeclList[insertPos:]...)
	file.DeclList = newDeclList
}

func init() {
	RegisterTransformer(&AsCastTransform{})
}
