// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build transforms

// danger, conflict with src of vendor packages

package transforms

import (
	"cmd/compile/internal/syntax"
	"strings"
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
	
	// First, handle AsCastExpr directly if this node is one
	if asCast, ok := node.(*syntax.AsCastExpr); ok {
		// Replace the AsCastExpr directly - this approach requires modifying the parent
		// Since we can't modify the parent from here, we'll handle specific parent types below
		_ = asCast // Just to note we found one
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
		// Handle both simple assignments and list assignments
		if asCast, ok := n.Rhs.(*syntax.AsCastExpr); ok {
			if newExpr := v.transform.convertAsCastToAssert(asCast, v); newExpr != asCast {
				n.Rhs = newExpr
				v.changed = true
			}
		}
		// Check if RHS is a list expression containing AsCastExpr
		if listExpr, ok := n.Rhs.(*syntax.ListExpr); ok {
			for i, elem := range listExpr.ElemList {
				if asCast, ok := elem.(*syntax.AsCastExpr); ok {
					if newExpr := v.transform.convertAsCastToAssert(asCast, v); newExpr != asCast {
						listExpr.ElemList[i] = newExpr
						v.changed = true
					}
				}
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
	case *syntax.ParenExpr:
		// Handle AsCastExpr inside parentheses: (expr as T)
		if asCast, ok := n.X.(*syntax.AsCastExpr); ok {
			if newExpr := v.transform.convertAsCastToAssert(asCast, v); newExpr != asCast {
				n.X = newExpr
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
	sourceType := t.inferExprType(expr, visitor.ctx)
	
	// Handle specific conversion patterns
	switch {
	// Numeric to string: 1 as string -> strconv.Itoa(1)
	case targetType == "string" && (sourceType == "int" || sourceType == "int_literal"):
		visitor.needsStrconvImport = true
		return t.createStrconvCall("Itoa", expr, pos)
		
	// Float to string: 3.14 as string -> strconv.FormatFloat(3.14, 'g', -1, 64)
	case targetType == "string" && (sourceType == "float64" || sourceType == "float_literal"):
		visitor.needsStrconvImport = true
		return t.createFloatToStringCall(expr, pos)
		
	// Non-base types to string: obj as string -> obj.String()
	case targetType == "string" && !t.isBaseType(sourceType):
		return t.createStringMethodCall(expr, pos)
		
	// String to int: "1" as int -> runtime stringtoint function
	case (targetType == "int" || targetType == "int32" || targetType == "int64") && 
		 (sourceType == "string" || sourceType == "string_literal"):
		return t.createStringToIntCall(expr, pos)
		
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
func (t *AsCastTransform) inferExprType(expr syntax.Expr, ctx *TransformContext) string {
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
		// Look up the variable's declared type in the context if available
		if ctx != nil && ctx.Types != nil {
			if declaredType, exists := ctx.Types[e.Value]; exists {
				return declaredType
			}
		}
		// For named variables without context, assume they are custom types
		// This allows String() method conversion to trigger
		return "custom_type"
	case *syntax.IndexExpr:
		// For slice/array indexing like arr[i], try to infer the element type
		// If we can't determine the specific element type, assume it could be any interface value
		// For any_list[1], the element type should be 'any'
		if baseName, ok := e.X.(*syntax.Name); ok {
			if ctx != nil && ctx.Types != nil {
				if declaredType, exists := ctx.Types[baseName.Value]; exists {
					// Handle specific known slice types
					switch declaredType {
					case "[]any":
						return "any"
					case "[]int":
						return "int"
					case "[]string":
						return "string"
					case "[]float64":
						return "float64"
					// Add more specific cases as needed
					}
				}
			}
		}
		// Default for unknown slice indexing - assume interface value
		return "any"
	case *syntax.Operation:
		// For operations, try to infer from operands
		return "custom_type"
	}
	return "custom_type" // Default to custom type to enable String() method calls
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
	// Use a simplified must pattern for single-value contexts
	if funcName == "Atoi" {
		return t.createMustAtoi(callExpr, pos)
	}
	
	return callExpr
}

// createFloatToStringCall creates strconv.FormatFloat call for float to string conversion
func (t *AsCastTransform) createFloatToStringCall(expr syntax.Expr, pos syntax.Pos) syntax.Expr {
	// Create: strconv.FormatFloat(expr, 'g', -1, 64)
	// 'g' format, -1 precision (shortest), 64-bit size
	
	// Create strconv.FormatFloat
	strconvName := &syntax.Name{Value: "strconv"}
	strconvName.SetPos(pos)
	
	formatFloatName := &syntax.Name{Value: "FormatFloat"}
	formatFloatName.SetPos(pos)
	
	selectorExpr := &syntax.SelectorExpr{
		X:   strconvName,
		Sel: formatFloatName,
	}
	selectorExpr.SetPos(pos)
	
	// Create 'g' argument (format)
	formatArg := &syntax.BasicLit{
		Value: "'g'",
		Kind:  syntax.RuneLit,
	}
	formatArg.SetPos(pos)
	
	// Create -1 argument (precision)
	precisionArg := &syntax.Operation{
		Op: syntax.Sub,
		X:  &syntax.BasicLit{Value: "0", Kind: syntax.IntLit},
		Y:  &syntax.BasicLit{Value: "1", Kind: syntax.IntLit},
	}
	precisionArg.SetPos(pos)
	
	// Create 64 argument (bitsize)
	bitsizeArg := &syntax.BasicLit{
		Value: "64",
		Kind:  syntax.IntLit,
	}
	bitsizeArg.SetPos(pos)
	
	// Create the function call
	callExpr := &syntax.CallExpr{
		Fun:     selectorExpr,
		ArgList: []syntax.Expr{expr, formatArg, precisionArg, bitsizeArg},
	}
	callExpr.SetPos(pos)
	
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

// createFloatLiteralToIntConversion converts untyped float literals to int with truncation
func (t *AsCastTransform) createFloatLiteralToIntConversion(expr syntax.Expr, pos syntax.Pos) syntax.Expr {
	// For semantic casting like 3.14 as int == 3, we want truncation behavior
	// Transform: 3.14 as int -> int(3.14) using truncation
	// Since Go doesn't allow int(3.14) directly, we'll extract the integer part
	
	// For simple cases like 3.14, we can evaluate at compile time
	if basicLit, ok := expr.(*syntax.BasicLit); ok && basicLit.Kind == syntax.FloatLit {
		// Extract the integer part from the float literal
		val := basicLit.Value
		// Find the decimal point and take everything before it
		if dotIndex := strings.Index(val, "."); dotIndex != -1 {
			intPart := val[:dotIndex]
			// Create new integer literal with the truncated value
			truncatedLit := &syntax.BasicLit{
				Value: intPart,
				Kind:  syntax.IntLit,
			}
			truncatedLit.SetPos(pos)
			return truncatedLit
		}
	}
	
	// For complex expressions, fall back to regular conversion (may fail)
	return t.createBasicTypeConversion(expr, "int", pos)
}

// createSimpleAtoi creates a very simple pattern to call strconv.Atoi and get just the value
func (t *AsCastTransform) createSimpleAtoi(expr syntax.Expr, pos syntax.Pos) syntax.Expr {
	// Create strconv.Atoi(expr) call first
	strconvName := &syntax.Name{Value: "strconv"}
	strconvName.SetPos(pos)
	
	atoiName := &syntax.Name{Value: "Atoi"}
	atoiName.SetPos(pos)
	
	selectorExpr := &syntax.SelectorExpr{
		X:   strconvName,
		Sel: atoiName,
	}
	selectorExpr.SetPos(pos)
	
	atoiCall := &syntax.CallExpr{
		Fun:     selectorExpr,
		ArgList: []syntax.Expr{expr},
	}
	atoiCall.SetPos(pos)
	
	// Create simple wrapper that ignores error and just returns value
	// Pattern: (func() int { v, _ := strconv.Atoi(expr); return v })()
	return t.createValueOnlyWrapper(atoiCall, pos)
}

// createValueOnlyWrapper creates a minimal wrapper to extract just the value from a multi-return function
func (t *AsCastTransform) createValueOnlyWrapper(call syntax.Expr, pos syntax.Pos) syntax.Expr {
	// Create: (func() int { v, _ := call; return v })()
	// But use extremely simple approach to avoid position issues
	
	// Use the call position instead of the passed position to avoid issues
	callPos := call.Pos()
	
	// v variable
	vVar := &syntax.Name{Value: "v"}
	vVar.SetPos(callPos)
	
	// _ variable  
	blankVar := &syntax.Name{Value: "_"}
	blankVar.SetPos(callPos)
	
	// v, _ list
	lhs := &syntax.ListExpr{ElemList: []syntax.Expr{vVar, blankVar}}
	lhs.SetPos(callPos)
	
	// v, _ := call
	assign := &syntax.AssignStmt{
		Op:  syntax.Def,
		Lhs: lhs,
		Rhs: call,
	}
	assign.SetPos(callPos)
	
	// return v
	ret := &syntax.ReturnStmt{Results: vVar}
	ret.SetPos(callPos)
	
	// function body
	body := &syntax.BlockStmt{List: []syntax.Stmt{assign, ret}}
	body.SetPos(callPos)
	
	// int type
	intType := &syntax.Name{Value: "int"}
	intType.SetPos(callPos)
	
	// func() int
	funcType := &syntax.FuncType{
		ResultList: []*syntax.Field{{Type: intType}},
	}
	funcType.SetPos(callPos)
	
	// func() int { ... }
	funcLit := &syntax.FuncLit{
		Type: funcType,
		Body: body,
	}
	funcLit.SetPos(callPos)
	
	// (func() int { ... })()
	result := &syntax.CallExpr{Fun: funcLit}
	result.SetPos(callPos)
	
	return result
}

// createMustAtoi creates a simple wrapper that extracts the value from strconv.Atoi
func (t *AsCastTransform) createMustAtoi(atoiCall syntax.Expr, pos syntax.Pos) syntax.Expr {
	// Use the simplest possible approach that works with Go's syntax
	// Generate: (func() int { v, _ := call(); return v })()
	// But using only the most basic syntax nodes to avoid position issues
	
	// Create a minimal function literal that calls strconv.Atoi and returns the value
	// This is much simpler than the previous complex version
	
	// Create result variable: v
	vVar := &syntax.Name{Value: "v"}
	vVar.SetPos(pos)
	
	// Create blank variable: _
	blank := &syntax.Name{Value: "_"}
	blank.SetPos(pos)
	
	// Create list for LHS: v, _
	lhsList := &syntax.ListExpr{ElemList: []syntax.Expr{vVar, blank}}
	lhsList.SetPos(pos)
	
	// Create assignment: v, _ := atoiCall
	assign := &syntax.AssignStmt{
		Op:  syntax.Def,
		Lhs: lhsList,
		Rhs: atoiCall,
	}
	assign.SetPos(pos)
	
	// Create return: return v
	ret := &syntax.ReturnStmt{Results: vVar}
	ret.SetPos(pos)
	
	// Create function body: { v, _ := atoiCall; return v }
	body := &syntax.BlockStmt{List: []syntax.Stmt{assign, ret}}
	body.SetPos(pos)
	
	// Create function type: func() int
	intType := &syntax.Name{Value: "int"}
	intType.SetPos(pos)
	funcType := &syntax.FuncType{
		ResultList: []*syntax.Field{{Type: intType}},
	}
	funcType.SetPos(pos)
	
	// Create function literal
	funcLit := &syntax.FuncLit{Type: funcType, Body: body}
	funcLit.SetPos(pos)
	
	// Create function call: (func()...)()
	callExpr := &syntax.CallExpr{Fun: funcLit}
	callExpr.SetPos(pos)
	return callExpr
}

// createSimpleWrapper creates a simple wrapper for multi-value returns
func (t *AsCastTransform) createSimpleWrapper(call syntax.Expr, returnType string, pos syntax.Pos) syntax.Expr {
	// Use a very simple pattern that Go can handle:
	// Create an immediately invoked function expression (IIFE) with minimal complexity
	// Pattern: (func() int { v, _ := strconv.Atoi(s); return v })()
	
	// Create: func() returnType { v, _ := call; return v }
	
	// Use the position from the call expression instead
	callPos := call.Pos()
	
	// Variable for result
	resultVar := &syntax.Name{Value: "v"}
	resultVar.SetPos(callPos)
	
	// Blank identifier for error
	blankVar := &syntax.Name{Value: "_"}
	blankVar.SetPos(callPos)
	
	// Assignment LHS: v, _
	assignLHS := &syntax.ListExpr{
		ElemList: []syntax.Expr{resultVar, blankVar},
	}
	assignLHS.SetPos(callPos)
	
	// Assignment statement: v, _ := call
	assignment := &syntax.AssignStmt{
		Op:  syntax.Def,
		Lhs: assignLHS,
		Rhs: call,
	}
	assignment.SetPos(callPos)
	
	// Return statement: return v
	returnStmt := &syntax.ReturnStmt{
		Results: resultVar,
	}
	returnStmt.SetPos(callPos)
	
	// Function body: { v, _ := call; return v }
	body := &syntax.BlockStmt{
		List: []syntax.Stmt{assignment, returnStmt},
	}
	body.SetPos(callPos)
	
	// Return type for function
	retType := &syntax.Name{Value: returnType}
	retType.SetPos(callPos)
	
	// Function type: func() returnType
	funcType := &syntax.FuncType{
		ResultList: []*syntax.Field{{Type: retType}},
	}
	funcType.SetPos(callPos)
	
	// Function literal: func() returnType { ... }
	funcLit := &syntax.FuncLit{
		Type: funcType,
		Body: body,
	}
	funcLit.SetPos(callPos)
	
	// Function call: (func() returnType { ... })()
	funcCall := &syntax.CallExpr{
		Fun: funcLit,
	}
	funcCall.SetPos(callPos)
	
	return funcCall
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

// isBaseType checks if a type is a Go base type that has built-in string conversion
func (t *AsCastTransform) isBaseType(sourceType string) bool {
	baseTypes := map[string]bool{
		"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true, "float": true,
		"byte": true, "rune": true, "bool": true,
		"string": true,
		// Interface types should use type assertions, not method calls
		"any": true, "interface{}": true,
		// Literal types
		"int_literal": true, "float_literal": true, "string_literal": true, "rune_literal": true,
	}
	return baseTypes[sourceType]
}

// createStringToIntCall creates a function literal that converts string to int inline
func (t *AsCastTransform) createStringToIntCall(expr syntax.Expr, pos syntax.Pos) syntax.Expr {
	// Follow the pattern of string_methods_transform: create a complete function literal
	// Pattern: func() int { /* inline conversion */ }()
	
	// Use the same naming pattern as string methods transform: stringToInt
	funcName := &syntax.Name{Value: "stringToInt"}
	funcName.SetPos(pos)
	
	callExpr := &syntax.CallExpr{
		Fun:     funcName,
		ArgList: []syntax.Expr{expr},
	}
	callExpr.SetPos(pos)
	
	return callExpr
}

// createStringMethodCall creates obj.String() method call
func (t *AsCastTransform) createStringMethodCall(expr syntax.Expr, pos syntax.Pos) syntax.Expr {
	// Create method name: String
	methodName := &syntax.Name{Value: "String"}
	methodName.SetPos(pos)
	
	// Create selector: expr.String
	selectorExpr := &syntax.SelectorExpr{
		X:   expr,
		Sel: methodName,
	}
	selectorExpr.SetPos(pos)
	
	// Create method call: expr.String()
	callExpr := &syntax.CallExpr{
		Fun: selectorExpr,
	}
	callExpr.SetPos(pos)
	
	return callExpr
}

func init() {
	RegisterTransformer(&AsCastTransform{})
}
