// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

// only include in cmd/compile build with transforms enabled!

package transforms

import (
	"cmd/compile/internal/syntax"
	"fmt"
)

// StringConcatTransform handles automatic string conversion in concatenation.
// It transforms expressions like
// "result:" + z --> "result:" + strconv.Itoa(z)  // deprecated vs:
// "result:" + z --> "result:" + fmt.Sprintf("%v", z) // works!
// "result:" + z --> "result:" + z.String()  // NOT for int!

type StringConcatTransform struct{
	needsFmtImport bool
}

// concatVisitor implements syntax.Visitor to transform string concatenations
type concatVisitor struct {
	transform      *StringConcatTransform
	ctx            *TransformContext //ctx.Types[name.Value] // e.g., "int", "string" guessed in transform.go
	changed        bool
	needsFmtImport bool
}

func (t *StringConcatTransform) Name() string {
	return "string_concat_transform"
}

func (t *StringConcatTransform) Priority() int {
	return 100 // Default priority - between list methods (50) and lambda (200)
}

// NodeTransformer interface implementation
func (t *StringConcatTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
	// Only handle ADD operations directly - the central visitor will find them
	if op, ok := node.(*syntax.Operation); ok {
		return op.Op == syntax.Add
	}
	return false
}

func (t *StringConcatTransform) TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node {
	if op, ok := node.(*syntax.Operation); ok && op.Op == syntax.Add {
		// Check if it's string interpolation first
		parts := t.extractInterpolationParts(op)
		if len(parts) >= 3 && t.isStringInterpolationPattern(parts, ctx) {
			t.needsFmtImport = true
			return t.buildInterpolationChain(parts, ctx)
		}
		
		// Otherwise check for regular concatenation
		if transformed := t.transformConcatOperation(op, ctx); transformed != nil {
			t.needsFmtImport = true
			return transformed
		}
	}
	
	return nil
}

func (t *StringConcatTransform) PostProcess(file *syntax.File, ctx *TransformContext) bool {
	// Add fmt import if needed
	if t.needsFmtImport && !t.hasImport(file, "fmt") {
		t.addFmtImport(file)
		t.needsFmtImport = false
		return true
	}
	return false
}

func (t *StringConcatTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	//println("*** CUSTOM_COMPILER_BUILD_VERIFICATION_2025_07_27 ***")

	// Single pass: walk the entire AST once using syntax.Walk
	visitor := &concatVisitor{transform: t, ctx: ctx}
	syntax.Walk(file, visitor)

	// Add fmt import if needed and transformations were made
	if visitor.needsFmtImport && !t.hasImport(file, "fmt") {
		println("Adding fmt import")
		t.addFmtImport(file)
	}

	return visitor.changed
}

// Visit implements syntax.Visitor interface
func (v *concatVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}

	// Check for string interpolation patterns in expressions
	if expr, ok := node.(*syntax.ExprStmt); ok {
		if transformed := v.transform.transformStringInterpolation(expr.X, v.ctx); transformed != nil {
			println("TRANSFORMING INTERPOLATION:", syntax.String(expr.X), "->", syntax.String(transformed))
			expr.X = transformed
			v.changed = true
			v.needsFmtImport = true
		}
	}

	// Check for string interpolation in assignments
	if assign, ok := node.(*syntax.AssignStmt); ok && assign.Rhs != nil {
		if transformed := v.transform.transformStringInterpolation(assign.Rhs, v.ctx); transformed != nil {
			println("TRANSFORMING ASSIGNMENT INTERPOLATION:", syntax.String(assign.Rhs), "->", syntax.String(transformed))
			assign.Rhs = transformed
			v.changed = true
			v.needsFmtImport = true
		}
	}

	// Check for string concatenation operations (but skip if it's already an interpolation pattern)
	if op, ok := node.(*syntax.Operation); ok && op.Op == syntax.Add {
		// Don't apply regular concatenation to operations that are part of interpolation patterns
		parts := v.transform.extractInterpolationParts(op)
		if len(parts) >= 3 && v.transform.isStringInterpolationPattern(parts, v.ctx) {
			// This is already handled by string interpolation, skip regular concatenation
			return v
		}
		
		if transformed := v.transform.transformConcatOperation(op, v.ctx); transformed != nil {
			if newOp, ok := transformed.(*syntax.Operation); ok {
				op.X = newOp.X
				op.Y = newOp.Y
				v.changed = true
				v.needsFmtImport = true
			}
		}
	}

	return v // Continue walking
}

// transformConcatOperation checks if this is a string concatenation with a non-string operand
// and wraps the non-string operand with fmt.Sprintf if it's an integer.
func (t *StringConcatTransform) transformConcatOperation(op *syntax.Operation, ctx *TransformContext) syntax.Expr {
	if op.Op != syntax.Add {
		return nil
	}

	// First, check if this is part of a chained concatenation
	if chainResult := t.transformConcatChain(op, ctx); chainResult != nil {
		return chainResult
	}

	// Fall back to individual operation handling
	leftIsString := t.isStringExpression(op.X, ctx)
	rightIsString := t.isStringExpression(op.Y, ctx)

	if leftIsString && !rightIsString {
		if t.mightBeNumericVariable(op.Y, ctx) {
			return &syntax.Operation{
				Op: op.Op,
				X:  op.X,
				Y:  t.createSprintfCall(op.Y),
			}
		}
	} else if rightIsString && !leftIsString {
		if t.mightBeNumericVariable(op.X, ctx) {
			return &syntax.Operation{
				Op: op.Op,
				X:  t.createSprintfCall(op.X),
				Y:  op.Y,
			}
		}
	}

	return nil
}

// transformConcatChain handles chained concatenation operations like "prefix" + 1 + 2 + "suffix"
func (t *StringConcatTransform) transformConcatChain(op *syntax.Operation, ctx *TransformContext) syntax.Expr {
	// Extract all parts of the concatenation chain
	parts := t.extractConcatenationParts(op)
	if len(parts) < 2 {
		return nil
	}

	// Check if this chain involves any string concatenation with non-strings
	hasStringConcatWithNonString := false
	for i := 0; i < len(parts)-1; i++ {
		leftIsString := t.isStringExpression(parts[i], ctx) || t.containsString(parts[i], ctx)
		rightIsString := t.isStringExpression(parts[i+1], ctx)

		if (leftIsString && !rightIsString && t.mightBeNumericVariable(parts[i+1], ctx)) ||
			(!leftIsString && rightIsString && t.mightBeNumericVariable(parts[i], ctx)) {
			hasStringConcatWithNonString = true
			break
		}
	}

	if !hasStringConcatWithNonString {
		return nil
	}

	// Transform the chain: wrap non-string operands when they're being concatenated with strings
	var result syntax.Expr = parts[0]

	for i := 1; i < len(parts); i++ {
		leftIsStringish := t.isStringExpression(result, ctx) || t.containsString(result, ctx)
		rightIsString := t.isStringExpression(parts[i], ctx)

		var rightSide syntax.Expr = parts[i]

		// If left side is string-ish and right side is not string but could be numeric, wrap it
		if leftIsStringish && !rightIsString && t.mightBeNumericVariable(parts[i], ctx) {
			rightSide = t.createSprintfCall(parts[i])
		}

		result = &syntax.Operation{
			Op: syntax.Add,
			X:  result,
			Y:  rightSide,
		}
		result.SetPos(op.Pos())
	}

	return result
}

// extractConcatenationParts extracts all parts from a left-associative concatenation chain
func (t *StringConcatTransform) extractConcatenationParts(expr syntax.Expr) []syntax.Expr {
	var parts []syntax.Expr

	// Handle left-associative chaining: ((a + b) + c) + d
	var collect func(syntax.Expr)
	collect = func(e syntax.Expr) {
		if op, ok := e.(*syntax.Operation); ok && op.Op == syntax.Add {
			// Recursively collect from left side first
			collect(op.X)
			// Then add the right side
			parts = append(parts, op.Y)
		} else {
			// Base case: not an operation, add as first part
			parts = append(parts, e)
		}
	}

	collect(expr)
	return parts
}

// containsString checks if an expression contains string operations (for complex expressions)
func (t *StringConcatTransform) containsString(expr syntax.Expr, ctx *TransformContext) bool {
	// Check if the expression itself is a string
	if t.isStringExpression(expr, ctx) {
		return true
	}

	// For operations, check if any operand is a string
	if op, ok := expr.(*syntax.Operation); ok && op.Op == syntax.Add {
		return t.containsString(op.X, ctx) || t.containsString(op.Y, ctx)
	}

	return false
}

// transformStringInterpolation handles string interpolation patterns like "str" value "str"
// and transforms them to "str" + fmt.Sprintf("%v", value) + "str"
func (t *StringConcatTransform) transformStringInterpolation(expr syntax.Expr, ctx *TransformContext) syntax.Expr {
	// Look for operations that could be string interpolation
	if op, ok := expr.(*syntax.Operation); ok {
		// Check if this looks like string interpolation (consecutive operations)
		parts := t.extractInterpolationParts(op)
		if len(parts) >= 3 && t.isStringInterpolationPattern(parts, ctx) {
			println("Found string interpolation pattern with", len(parts), "parts")
			return t.buildInterpolationChain(parts, ctx)
		}
	}
	return nil
}

// extractInterpolationParts extracts all consecutive parts from nested operations
func (t *StringConcatTransform) extractInterpolationParts(expr syntax.Expr) []syntax.Expr {
	var parts []syntax.Expr

	// Handle nested operations by flattening them
	var flatten func(syntax.Expr)
	flatten = func(e syntax.Expr) {
		if op, ok := e.(*syntax.Operation); ok && op.Op == syntax.Add {
			// For operations, process left and right recursively
			flatten(op.X)
			flatten(op.Y)
		} else {
			// For non-operations, add as a part
			parts = append(parts, e)
		}
	}

	flatten(expr)
	return parts
}

// isStringInterpolationPattern checks if the parts form a valid string interpolation pattern
func (t *StringConcatTransform) isStringInterpolationPattern(parts []syntax.Expr, ctx *TransformContext) bool {
	if len(parts) < 3 {
		return false
	}

	// Check if pattern alternates: string literal, any value, string literal, ...
	// The key is that even positions (0, 2, 4, ...) should be string literals
	// and odd positions (1, 3, 5, ...) can be any values (including string variables)
	for i, part := range parts {
		if i%2 == 0 {
			// Even positions should be string literals (not just any string expression)
			if !t.isStringLiteral(part) {
				return false
			}
		}
		// Odd positions can be anything (numbers, variables, expressions, etc.)
	}

	return true
}

// buildInterpolationChain builds a chain of + operations with fmt.Sprintf calls
func (t *StringConcatTransform) buildInterpolationChain(parts []syntax.Expr, ctx *TransformContext) syntax.Expr {
	if len(parts) == 0 {
		return nil
	}

	var result syntax.Expr = parts[0]

	for i := 1; i < len(parts); i++ {
		var rightSide syntax.Expr

		// For interpolated values (not string literals), add spacing
		if !t.isStringLiteral(parts[i]) {
			// This includes both non-string values and string variables
			rightSide = t.createSprintfCallWithSpacing(parts[i])
		} else {
			// Only string literals don't get spacing
			rightSide = parts[i]
		}

		result = &syntax.Operation{
			Op: syntax.Add,
			X:  result,
			Y:  rightSide,
		}
	}

	return result
}

// isStringLiteral returns true if the expression is a string literal.
func (t *StringConcatTransform) isStringLiteral(expr syntax.Expr) bool {
	if basic, ok := expr.(*syntax.BasicLit); ok {
		return basic.Kind == syntax.StringLit
	}
	return false
}

// isStringExpression returns true if the expression is definitely a string
func (t *StringConcatTransform) isStringExpression(expr syntax.Expr, ctx *TransformContext) bool {
	// Check if it's a string literal - this is definitive
	if t.isStringLiteral(expr) {
		return true
	}

	// Check if it's a string variable with known type
	if name, ok := expr.(*syntax.Name); ok {
		if ctx != nil && ctx.Types != nil {
			varType := ctx.Types[name.Value]
			return varType == "string"
		}
		// Without type context, be conservative and assume NOT string
		return false
	}

	// For all other cases (function calls, selectors, etc.), be conservative
	// and assume NOT string unless we have definitive proof
	return false
}

// mightBeNumericVariable returns true if the expression could be a numeric variable.
// Enhanced to handle complex expressions including operations, array access, struct fields, etc.
func (t *StringConcatTransform) mightBeNumericVariable(expr syntax.Expr, ctx *TransformContext) bool {
	// Handle literal numbers and booleans
	if basic, ok := expr.(*syntax.BasicLit); ok {
		return basic.Kind == syntax.IntLit || basic.Kind == syntax.FloatLit || basic.Kind == syntax.ImagLit
	}

	// Handle boolean literals (true/false are represented as Names)
	if name, ok := expr.(*syntax.Name); ok {
		if name.Value == "true" || name.Value == "false" {
			//println("mightBeNumeric fallback for", name.Value)
			return true
		}
	}

	// Handle variables with known types
	if name, ok := expr.(*syntax.Name); ok {
		if ctx != nil && ctx.Types != nil {
			varType := ctx.Types[name.Value]
			return varType == "int" || varType == "float64" || varType == "float32" ||
				varType == "int32" || varType == "int64" || varType == "int8" || varType == "int16" ||
				varType == "uint" || varType == "uint8" || varType == "uint16" || varType == "uint32" || varType == "uint64" ||
				varType == "bool" || varType == "complex64" || varType == "complex128" || varType == "byte" || varType == "rune"
		}
		// Conservative fallback - assume common variable names might be numeric
		//println("mightBeNumeric fallback for", name.Value)
		return true
	}

	// Handle parenthesized expressions
	if paren, ok := expr.(*syntax.ParenExpr); ok {
		return t.mightBeNumericVariable(paren.X, ctx)
	}

	// Handle arithmetic operations (likely to be numeric)
	if op, ok := expr.(*syntax.Operation); ok {
		switch op.Op {
		case syntax.Add, syntax.Sub, syntax.Mul, syntax.Div, syntax.Rem:
			return true // Arithmetic operations are numeric
		case syntax.And, syntax.Or, syntax.Eql, syntax.Neq, syntax.Lss, syntax.Leq, syntax.Gtr, syntax.Geq:
			return true // Boolean operations
		}
	}

	// Handle array/slice indexing
	if _, ok := expr.(*syntax.IndexExpr); ok {
		return true // Assume array elements might be numeric
	}

	// Handle struct field access
	if _, ok := expr.(*syntax.SelectorExpr); ok {
		return true // Assume struct fields might be numeric
	}

	// Handle function calls
	if _, ok := expr.(*syntax.CallExpr); ok {
		return true // Assume function results might be numeric
	}

	// Handle unary operations
	if unary, ok := expr.(*syntax.Operation); ok && unary.Y == nil {
		switch unary.Op {
		case syntax.Add, syntax.Sub, syntax.Not:
			return true // +x, -x, !x
		case syntax.Mul: // *ptr (pointer dereference)
			return true
		}
	}

	return false
}

// createItoacCall creates a syntax tree for strconv.Itoa(expr).
//func (t *StringConcatTransform) createItoacCall(expr syntax.Expr) syntax.Expr {
//	// Create strconv.Itoa(expr)
//	return &syntax.CallExpr{
//		Fun: &syntax.SelectorExpr{
//			X: &syntax.Name{
//				Value: "strconv",
//			},
//			Sel: &syntax.Name{
//				Value: "Itoa",
//			},
//		},
//		ArgList: []syntax.Expr{expr},
//	}
//}

func (t *StringConcatTransform) createSprintfCall(expr syntax.Expr) syntax.Expr {
	pos := expr.Pos()

	fmtName := &syntax.Name{Value: "fmt"}
	fmtName.SetPos(pos)

	sprintfName := &syntax.Name{Value: "Sprintf"}
	sprintfName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   fmtName,
		Sel: sprintfName,
	}
	selector.SetPos(pos)

	formatLit := &syntax.BasicLit{
		Kind:  syntax.StringLit,
		Value: "\"%v\"",
	}
	formatLit.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{formatLit, expr},
	}
	call.SetPos(pos)

	return call
}

// createSprintfCallWithSpacing creates a fmt.Sprintf call with spaces around the value
func (t *StringConcatTransform) createSprintfCallWithSpacing(expr syntax.Expr) syntax.Expr {
	pos := expr.Pos()

	fmtName := &syntax.Name{Value: "fmt"}
	fmtName.SetPos(pos)

	sprintfName := &syntax.Name{Value: "Sprintf"}
	sprintfName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   fmtName,
		Sel: sprintfName,
	}
	selector.SetPos(pos)

	formatLit := &syntax.BasicLit{
		Kind:  syntax.StringLit,
		Value: "\" %v \"",  // Note the spaces around %v
	}
	formatLit.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{formatLit, expr},
	}
	call.SetPos(pos)

	return call
}

func (t *StringConcatTransform) addFmtImport(file *syntax.File) {
	// Check if fmt is already imported
	if t.hasImport(file, "fmt") {
		return
	}

	// Add fmt import
	fmtImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: "\"fmt\"",
			Kind:  syntax.StringLit,
		},
	}

	// Insert at the beginning or after package declaration
	var insertPos int
	for i, decl := range file.DeclList {
		if _, ok := decl.(*syntax.ImportDecl); ok {
			insertPos = i + 1
		} else {
			break
		}
	}

	// Insert the import
	newDeclList := make([]syntax.Decl, 0, len(file.DeclList)+1)
	newDeclList = append(newDeclList, file.DeclList[:insertPos]...)
	newDeclList = append(newDeclList, fmtImport)
	newDeclList = append(newDeclList, file.DeclList[insertPos:]...)
	file.DeclList = newDeclList
}

// addStrconvImport adds the strconv import to the file
//func (t *StringConcatTransform) addStrconvImport(file *syntax.File) {
//	// Check if strconv is already imported
//	if t.hasImport(file, "strconv") {
//		return
//	}
//
//	// Add strconv import
//	strconvImport := &syntax.ImportDecl{
//		Path: &syntax.BasicLit{
//			Value: "\"strconv\"",
//			Kind:  syntax.StringLit,
//		},
//	}
//
//	// Insert at the beginning or after package declaration
//	var insertPos int
//	for i, decl := range file.DeclList {
//		if _, ok := decl.(*syntax.ImportDecl); ok {
//			insertPos = i + 1
//		} else {
//			break
//		}
//	}
//
//	// Insert the import
//	newDeclList := make([]syntax.Decl, 0, len(file.DeclList)+1)
//	newDeclList = append(newDeclList, file.DeclList[:insertPos]...)
//	newDeclList = append(newDeclList, strconvImport)
//	newDeclList = append(newDeclList, file.DeclList[insertPos:]...)
//	file.DeclList = newDeclList
//}

func (t *StringConcatTransform) hasImport(file *syntax.File, name string) bool {
	if name[0] != '"' { // Ensure the import name is quoted
		name = "\"" + name + "\""
	}
	for _, decl := range file.DeclList {
		if importDecl, ok := decl.(*syntax.ImportDecl); ok {
			if importDecl.Path != nil && importDecl.Path.Value == name {
				return true // Already imported
			}
		}
	}
	return false
}

func init() {
	RegisterTransformer(&StringConcatTransform{}) // per context?
}

// Test comment Sun Jul 27 11:43:35 CEST 2025
