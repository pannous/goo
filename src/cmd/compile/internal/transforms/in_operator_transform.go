// Copyright 2025 The Goo Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"strings"
)

// InOperatorTransform handles the 'in' operator for strings and collections
// Transforms expressions like "hello" in str to strings.Contains(str, "hello")
// and item in slice to slices.Contains(slice, item)
type InOperatorTransform struct{} // Clean implementation using centralized ImportManager

func (t *InOperatorTransform) Name() string {
	return "in_operator_transform"
}

func (t *InOperatorTransform) Priority() int {
	return 100 // Default priority - between list methods (50) and lambda (200)
}

// NodeTransformer interface implementation
func (t *InOperatorTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
	// Only handle IN operations directly - let the main transformer framework do tree walking
	if op, ok := node.(*syntax.Operation); ok {
		if op.Op == syntax.In {
			print("FOUND IN OPERATION\n")
			return true
		}
	}
	return false
}

func (t *InOperatorTransform) TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node {
	// Only handle direct IN operations - main transformer framework handles tree walking
	if op, ok := node.(*syntax.Operation); ok && op.Op == syntax.In {
		return t.convertInOperation(op, ctx)
	}
	return nil
}

func (t *InOperatorTransform) PostProcess(file *syntax.File, ctx *TransformContext) bool {
	// Let centralized ImportManager handle all imports
	return false
}

// Legacy Transform method for backward compatibility - not used in new architecture
func (t *InOperatorTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	// This method is kept for interface compatibility but not used
	// The new NodeTransformer interface methods are used instead
	return false
}

// convertInOperation converts "for item in collection" to appropriate Go code
func (t *InOperatorTransform) convertInOperation(op *syntax.Operation, ctx *TransformContext) syntax.Expr {
	pos := op.Pos()
	println("Converting rune to string for 'in' operation")

	// Handle rune in string: convert rune to string
	if t.isRuneLiteral(op.X) && t.inferContainerType(op.Y, ctx) == "string" {
		println("Converting rune to string for 'in' operation")
		op.X = &syntax.CallExpr{
			Fun:     &syntax.Name{Value: "string"},
			ArgList: []syntax.Expr{op.X},
		}
		op.X.SetPos(pos)
	}

	// Determine the type of operation based on the container (op.Y)
	containerType := t.inferContainerType(op.Y, ctx)

	switch containerType {
	case "string":
		return t.createStringContainsCall(op, pos)
	case "slice":
		return t.createSliceContainsCall(op, pos)
	case "map":
		return t.createMapContainsCall(op, pos)
	case "iterator":
		return t.createIteratorContainsCall(op, pos)
	default:
		// Try to determine at runtime or fall back to string
		return t.createStringContainsCall(op, pos)
	}
}

// inferContainerType tries to determine if the container is a string, slice, or map
func (t *InOperatorTransform) inferContainerType(container syntax.Expr, ctx *TransformContext) string {
	// Check for string literals
	if basic, ok := container.(*syntax.BasicLit); ok {
		if basic.Kind == syntax.StringLit {
			return "string"
		}
	}

	// Check for iterator function calls
	if t.isIteratorType(container) {
		return "iterator"
	}

	// Check for composite literals (slices/arrays/maps)
	if comp, ok := container.(*syntax.CompositeLit); ok {
		if comp.Type != nil {
			// Check for map types
			if _, isMap := comp.Type.(*syntax.MapType); isMap {
				return "map"
			}
			// Check for slice/array types
			if _, isSlice := comp.Type.(*syntax.SliceType); isSlice {
				return "slice"
			}
			if _, isArray := comp.Type.(*syntax.ArrayType); isArray {
				return "slice"
			}
		}
		// If no explicit type, infer from usage - composite literals are usually slices
		return "slice"
	}

	// Check context for variable types
	if name, ok := container.(*syntax.Name); ok && ctx != nil && ctx.Types != nil {
		if varType, exists := ctx.Types[name.Value]; exists {
			if strings.Contains(varType, "[]") {
				return "slice"
			}
			if strings.Contains(varType, "map[") {
				return "map"
			}
			if varType == "string" {
				return "string"
			}
		}
	}

	return "unknown"
}

// createStringContainsCall creates strings.Contains(container, item)
func (t *InOperatorTransform) createStringContainsCall(op *syntax.Operation, pos syntax.Pos) syntax.Expr {
	// Use centralized import manager (NEW approach)
	RequestStringsImport()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	containsName := &syntax.Name{Value: "Contains"}
	containsName.SetPos(pos)

	selectorExpr := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: containsName,
	}
	selectorExpr.SetPos(pos)

	callExpr := &syntax.CallExpr{
		Fun:     selectorExpr,
		ArgList: []syntax.Expr{op.Y, op.X}, // Note: argument order is reversed for strings.Contains
	}
	callExpr.SetPos(pos)

	return callExpr
}

func (t *InOperatorTransform) createInlineStringContains(op *syntax.Operation, pos syntax.Pos) syntax.Expr {
	// Generate proper inline string containment check without external imports
	// Create: len(container) >= len(item) && (len(item) == 0 || stringIndexOf(container, item) >= 0)

	// For simplicity, create a function literal that does the containment check
	// func() bool {
	//   s, sub := container, item
	//   if len(sub) == 0 { return true }
	//   for i := 0; i <= len(s)-len(sub); i++ {
	//     if s[i:i+len(sub)] == sub { return true }
	//   }
	//   return false
	// }()

	// Create the function body statements
	sParam := &syntax.Name{Value: "s"}
	sParam.SetPos(pos)
	subParam := &syntax.Name{Value: "sub"}
	subParam.SetPos(pos)

	// Assignment: s, sub := container, item
	assignStmt := &syntax.AssignStmt{
		Op:  syntax.Def, // :=
		Lhs: &syntax.ListExpr{ElemList: []syntax.Expr{sParam, subParam}},
		Rhs: &syntax.ListExpr{ElemList: []syntax.Expr{op.Y, op.X}}, // container, item
	}
	assignStmt.SetPos(pos)

	// if len(sub) == 0 { return true }
	lenSubCall := &syntax.CallExpr{
		Fun:     &syntax.Name{Value: "len"},
		ArgList: []syntax.Expr{subParam},
	}
	lenSubCall.SetPos(pos)
	lenSubCall.Fun.SetPos(pos)

	zeroLit := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
	zeroLit.SetPos(pos)

	lenCondition := &syntax.Operation{
		Op: syntax.Eql,
		X:  lenSubCall,
		Y:  zeroLit,
	}
	lenCondition.SetPos(pos)

	trueLit := &syntax.Name{Value: "true"}
	trueLit.SetPos(pos)

	emptyReturnStmt := &syntax.ReturnStmt{Results: trueLit}
	emptyReturnStmt.SetPos(pos)

	ifEmptyBody := &syntax.BlockStmt{List: []syntax.Stmt{emptyReturnStmt}}
	ifEmptyBody.SetPos(pos)

	ifEmptyStmt := &syntax.IfStmt{
		Cond: lenCondition,
		Then: ifEmptyBody,
	}
	ifEmptyStmt.SetPos(pos)

	// Simple loop-free approach: generate multiple substring checks
	// For practicality, we'll create a simpler version that uses string slicing
	// return len(s) >= len(sub) && (len(s) == 0 || s[:len(sub)] == sub || (len(s) > len(sub) && stringContains(s, sub)))

	// Actually, let's use strings.Contains but make sure the import is added
	RequestStringsImport()
	//RegisterImportWithPos("strings", pos)

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	containsName := &syntax.Name{Value: "Contains"}
	containsName.SetPos(pos)

	selectorExpr := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: containsName,
	}
	selectorExpr.SetPos(pos)

	callExpr := &syntax.CallExpr{
		Fun:     selectorExpr,
		ArgList: []syntax.Expr{op.Y, op.X}, // container, item
	}
	callExpr.SetPos(pos)

	return callExpr
}

func (t *InOperatorTransform) createSliceContainsCall(op *syntax.Operation, pos syntax.Pos) syntax.Expr {
	// Request slices import through centralized import manager
	RequestSlicesImport()

	slicesName := &syntax.Name{Value: "slices"}
	slicesName.SetPos(pos)

	containsName := &syntax.Name{Value: "Contains"}
	containsName.SetPos(pos)

	selectorExpr := &syntax.SelectorExpr{
		X:   slicesName,
		Sel: containsName,
	}
	selectorExpr.SetPos(pos)

	callExpr := &syntax.CallExpr{
		Fun:     selectorExpr,
		ArgList: []syntax.Expr{op.Y, op.X}, // Note: argument order is reversed for slices.Contains
	}
	callExpr.SetPos(pos)

	return callExpr
}

func (t *InOperatorTransform) createMapContainsCall(op *syntax.Operation, pos syntax.Pos) syntax.Expr {
	// For maps, "key in map" becomes "_, exists := map[key]; exists"
	// Create: (func() bool { _, ok := container[item]; return ok })()

	// Create index expression: container[item]
	indexExpr := &syntax.IndexExpr{
		X:     op.Y,
		Index: op.X,
	}
	indexExpr.SetPos(pos)

	// Create variables: _, ok
	blankVar := &syntax.Name{Value: "_"}
	blankVar.SetPos(pos)

	okVar := &syntax.Name{Value: "ok"}
	okVar.SetPos(pos)

	// Create LHS list: _, ok
	lhsList := &syntax.ListExpr{
		ElemList: []syntax.Expr{blankVar, okVar},
	}
	lhsList.SetPos(pos)

	// Create assignment: _, ok := container[item]
	assignStmt := &syntax.AssignStmt{
		Op:  syntax.Def,
		Lhs: lhsList,
		Rhs: indexExpr,
	}
	assignStmt.SetPos(pos)

	// Create return statement: return ok
	returnStmt := &syntax.ReturnStmt{
		Results: okVar,
	}
	returnStmt.SetPos(pos)

	// Create function body
	body := &syntax.BlockStmt{
		List: []syntax.Stmt{assignStmt, returnStmt},
	}
	body.SetPos(pos)

	// Create function type: func() bool
	boolType := &syntax.Name{Value: "bool"}
	boolType.SetPos(pos)

	funcType := &syntax.FuncType{
		ResultList: []*syntax.Field{{Type: boolType}},
	}
	funcType.SetPos(pos)

	// Create function literal
	funcLit := &syntax.FuncLit{
		Type: funcType,
		Body: body,
	}
	funcLit.SetPos(pos)

	// Create function call: (func() bool { ... })()
	callExpr := &syntax.CallExpr{
		Fun: funcLit,
	}
	callExpr.SetPos(pos)

	return callExpr
}

func (t *InOperatorTransform) createIteratorContainsCall(op *syntax.Operation, pos syntax.Pos) syntax.Expr {
	// For now, treat iterator as slice - this would need more sophisticated handling
	return t.createSliceContainsCall(op, pos)
}

// isIteratorType checks if an expression represents an iterator
func (t *InOperatorTransform) isIteratorType(expr syntax.Expr) bool {
	// Check for function calls that return iterators
	if call, ok := expr.(*syntax.CallExpr); ok {
		if fun, ok := call.Fun.(*syntax.SelectorExpr); ok {
			// Check for methods that typically return iterators
			methodName := fun.Sel.Value
			return methodName == "Iter" || methodName == "Iterator" || methodName == "Range"
		}
	}
	return false
}

// Import handling is now centralized in ImportManager
// No need for individual transformer import methods

func init() {
	RegisterTransformer(&InOperatorTransform{})
}

func (t *InOperatorTransform) isRuneLiteral(expr syntax.Expr) bool {
	if lit, ok := expr.(*syntax.BasicLit); ok && lit.Kind == syntax.RuneLit {
		return true
	}
	return false
}

// hasImport checks if the file already has the given import (copy from working system)
func (t *InOperatorTransform) hasImport(file *syntax.File, importPath string) bool {
	// Check for empty import path to prevent panic
	if len(importPath) == 0 {
		return false
	}

	// Normalize import path (add quotes if missing)
	normalizedPath := importPath
	if normalizedPath[0] != '"' {
		normalizedPath = "\"" + normalizedPath + "\""
	}

	for _, decl := range file.DeclList {
		if importDecl, ok := decl.(*syntax.ImportDecl); ok {
			if importDecl.Path != nil && importDecl.Path.Value == normalizedPath {
				return true
			}
		}
	}
	return false
}

// addStringsImport adds the strings import using EXACT copy of working StringMethodsTransform.addStringsImport
func (t *InOperatorTransform) addStringsImport(file *syntax.File) {
	if t.hasStringsImport(file) {
		return
	}

	stringsImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: "\"strings\"",
			Kind:  syntax.StringLit,
		},
	}
	fmt.Printf("DEBUG InOperatorTransform: Creating STRINGS import with Value='\"strings\"', Kind=%d - file has %d declarations before insert\n", syntax.StringLit, len(file.DeclList))
	stringsImport.SetPos(syntax.Pos{})

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
	newDeclList = append(newDeclList, stringsImport)
	newDeclList = append(newDeclList, file.DeclList[insertPos:]...)
	file.DeclList = newDeclList
	fmt.Printf("DEBUG InOperatorTransform: STRINGS import added - file now has %d declarations\n", len(file.DeclList))

	// Debug: print all import declarations
	fmt.Printf("DEBUG: All imports after adding:\n")
	for i, decl := range file.DeclList {
		if importDecl, ok := decl.(*syntax.ImportDecl); ok {
			fmt.Printf("  [%d] Import: %s (Kind=%d)\n", i, importDecl.Path.Value, importDecl.Path.Kind)
		}
	}
}

// hasStringsImport checks if strings import already exists using EXACT copy of working system 
func (t *InOperatorTransform) hasStringsImport(file *syntax.File) bool {
	for _, decl := range file.DeclList {
		if importDecl, ok := decl.(*syntax.ImportDecl); ok {
			if importDecl.Path != nil && importDecl.Path.Value == "\"strings\"" {
				return true
			}
		}
	}
	return false
}
