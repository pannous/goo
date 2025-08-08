// Copyright 2025 The Goo Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
	"fmt"
)

// InOperatorTransform handles the 'in' operator for strings and collections
// Transforms expressions like "hello" in str to strings.Contains(str, "hello")
// and item in slice to slices.Contains(slice, item)
type InOperatorTransform struct{}

type inOperatorVisitor struct {
	transform          *InOperatorTransform
	ctx                *TransformContext
	changed            bool
	needsStringsImport bool
	needsSlicesImport  bool
}

func (t *InOperatorTransform) Name() string {
	return "in_operator_transform"
}

func (t *InOperatorTransform) Priority() int {
	return 100 // Default priority - between list methods (50) and lambda (200)
}

// Transform using EXACT same pattern as StringMethodsTransform
func (t *InOperatorTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	fmt.Printf("InOperatorTransform.Transform called for package: %s\n", file.PkgName.Value)

	visitor := &inOperatorVisitor{transform: t, ctx: ctx}
	syntax.Walk(file, visitor)

	if visitor.needsStringsImport && !t.hasImport(file, "strings") {
		println("Adding strings import")
		t.addStringsImport(file)
	}
	if visitor.needsSlicesImport && !t.hasImport(file, "slices") {
		println("Adding slices import")
		t.addSlicesImport(file)
	}

	return visitor.changed
}

// Visit implements syntax.Visitor interface - EXACT same pattern as StringMethodsTransform
func (v *inOperatorVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}

	switch n := node.(type) {
	case *syntax.Operation:
		if n.Op == syntax.In {
			fmt.Printf("FOUND IN OPERATION: Converting '%s in %s'\n",
				nodeToString(n.X), nodeToString(n.Y))

			// Convert the IN operation
			newExpr := v.transform.convertInOperation(n, v.ctx)
			if newExpr != nil {
				*n = *newExpr.(*syntax.Operation)
				v.changed = true

				// Determine which import is needed
				containerType := v.transform.inferContainerType(n.Y, v.ctx)
				switch containerType {
				case "string":
					v.needsStringsImport = true
				case "slice":
					v.needsSlicesImport = true
				}
			}
		}
	}
	return v
}

// NodeTransformer interface - disabled to use legacy Transform approach
func (t *InOperatorTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
	return false // Use legacy Transform method instead
}

func (t *InOperatorTransform) TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node {
	return nil // Use legacy Transform method instead
}

func (t *InOperatorTransform) PostProcess(file *syntax.File, ctx *TransformContext) bool {
	return false // No post-processing needed
}

// convertInOperation converts "for item in collection" to appropriate Go code
func (t *InOperatorTransform) convertInOperation(op *syntax.Operation, ctx *TransformContext) syntax.Expr {
	pos := op.Pos()
	fmt.Printf("Converting 'in' operation at position %v\n", pos)

	// Handle rune in string: convert rune to string
	if t.isRuneLiteral(op.X) && t.inferContainerType(op.Y, ctx) == "string" {
		fmt.Printf("Converting rune to string for 'in' operation\n")
		op.X = &syntax.CallExpr{
			Fun:     &syntax.Name{Value: "string"},
			ArgList: []syntax.Expr{op.X},
		}
		op.X.SetPos(pos)
	}

	// Determine the type of operation based on the container (op.Y)
	containerType := t.inferContainerType(op.Y, ctx)
	fmt.Printf("Container type detected: %s\n", containerType)

	switch containerType {
	case "string":
		return t.createStringContainsCall(op, pos)
	case "slice":
		return t.createSliceContainsCall(op, pos)
	case "map":
		return t.createMapContainsCall(op, pos)
	default:
		// Default to string
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
			if len(varType) > 2 && varType[:2] == "[]" {
				return "slice"
			}
			if len(varType) > 4 && varType[:4] == "map[" {
				return "map"
			}
			if varType == "string" {
				return "string"
			}
		}
	}

	return "string" // Default to string for unknown types
}

// createStringContainsCall creates strings.Contains(container, item)
func (t *InOperatorTransform) createStringContainsCall(op *syntax.Operation, pos syntax.Pos) syntax.Expr {
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

func (t *InOperatorTransform) createSliceContainsCall(op *syntax.Operation, pos syntax.Pos) syntax.Expr {
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
	// Create: (func() bool { _, ok := container[item]; return ok })(

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

func (t *InOperatorTransform) isRuneLiteral(expr syntax.Expr) bool {
	if lit, ok := expr.(*syntax.BasicLit); ok && lit.Kind == syntax.RuneLit {
		return true
	}
	return false
}

// EXACT same import handling as StringMethodsTransform
func (t *InOperatorTransform) hasImport(file *syntax.File, pkg string) bool {
	for _, decl := range file.DeclList {
		if importDecl, ok := decl.(*syntax.ImportDecl); ok {
			if importDecl.Path != nil && importDecl.Path.Value == "\""+pkg+"\"" {
				return true
			}
		}
	}
	return false
}

func (t *InOperatorTransform) addStringsImport(file *syntax.File) {
	if t.hasImport(file, "strings") {
		return
	}

	stringsImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: "\"strings\"",
			Kind:  syntax.StringLit,
		},
	}
	fmt.Printf("DEBUG in_operator: Creating import with Value='\"strings\"', Kind=%d - file has %d declarations before insert\n", syntax.StringLit, len(file.DeclList))
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

	fmt.Printf("DEBUG in_operator: Added strings import, file now has %d declarations\n", len(file.DeclList))
}

func (t *InOperatorTransform) addSlicesImport(file *syntax.File) {
	if t.hasImport(file, "slices") {
		return
	}

	slicesImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: "\"slices\"",
			Kind:  syntax.StringLit,
		},
	}
	slicesImport.SetPos(syntax.Pos{})

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
	newDeclList = append(newDeclList, slicesImport)
	newDeclList = append(newDeclList, file.DeclList[insertPos:]...)
	file.DeclList = newDeclList
}

// Helper function to convert node to string for debugging
func nodeToString(node syntax.Expr) string {
	if node == nil {
		return "<nil>"
	}

	switch n := node.(type) {
	case *syntax.BasicLit:
		return n.Value
	case *syntax.Name:
		return n.Value
	default:
		return fmt.Sprintf("<%T>", node)
	}
}

func init() {
	RegisterTransformer(&InOperatorTransform{})
}
