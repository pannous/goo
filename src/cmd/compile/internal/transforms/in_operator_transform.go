// Copyright 2025 The Goo Authors. All rights reserved.

package transforms

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"strings"
)

// InOperatorTransform handles the 'in' operator for strings and collections
// Transforms expressions like "hello" in str to strings.Contains(str, "hello")
// and item in slice to slices.Contains(slice, item)
type InOperatorTransform struct{}

type inVisitor struct {
	transform           *InOperatorTransform
	ctx                 *TransformContext
	file                *syntax.File
	changed             bool
	needsStringsImport  bool
	needsSlicesImport   bool
}

func (t *InOperatorTransform) Name() string {
	return "in_operator_transform"
}

func (t *InOperatorTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &inVisitor{transform: t, ctx: ctx, file: file}
	
	// Walk all declarations and transform them
	for _, decl := range file.DeclList {
		t.walkAndTransform(decl, visitor)
	}
	
	// Add imports if needed - let automatic import resolver handle it
	// if visitor.needsStringsImport && !t.hasImport(file, "strings") {
	//	println("Adding strings import")
	//	t.addStringsImport(file)
	// }
	// if visitor.needsSlicesImport && !t.hasImport(file, "slices") {
	//	println("Adding slices import")
	//	t.addSlicesImport(file)
	// }
	
	return visitor.changed
}

// walkAndTransform walks the AST and transforms in operations
func (t *InOperatorTransform) walkAndTransform(node syntax.Node, visitor *inVisitor) {
	if node == nil {
		return
	}
	
	switch n := node.(type) {
	case *syntax.FuncDecl:
		if n.Body != nil {
			t.transformStmtList(n.Body.List, visitor)
		}
	case *syntax.VarDecl:
		if n.Values != nil {
			n.Values = t.transformExpr(n.Values, visitor)
		}
	}
}

// transformStmtList transforms a list of statements
func (t *InOperatorTransform) transformStmtList(stmts []syntax.Stmt, visitor *inVisitor) {
	for _, stmt := range stmts {
		t.transformStmt(stmt, visitor)
	}
}

// transformStmt transforms a single statement
func (t *InOperatorTransform) transformStmt(stmt syntax.Stmt, visitor *inVisitor) {
	if stmt == nil {
		return
	}
	
	switch s := stmt.(type) {
	case *syntax.ExprStmt:
		s.X = t.transformExpr(s.X, visitor)
	case *syntax.AssignStmt:
		if s.Rhs != nil {
			s.Rhs = t.transformExpr(s.Rhs, visitor)
		}
	case *syntax.CheckStmt:
		if s.Cond != nil {
			s.Cond = t.transformExpr(s.Cond, visitor)
		}
	case *syntax.BlockStmt:
		t.transformStmtList(s.List, visitor)
	case *syntax.IfStmt:
		if s.Cond != nil {
			s.Cond = t.transformExpr(s.Cond, visitor)
		}
		t.transformStmt(s.Then, visitor)
		if s.Else != nil {
			t.transformStmt(s.Else, visitor)
		}
	case *syntax.ForStmt:
		if s.Cond != nil {
			s.Cond = t.transformExpr(s.Cond, visitor)
		}
		if s.Init != nil {
			t.transformStmt(s.Init, visitor)
		}
		if s.Post != nil {
			t.transformStmt(s.Post, visitor)
		}
		t.transformStmt(s.Body, visitor)
	}
}

// transformExpr transforms a single expression
func (t *InOperatorTransform) transformExpr(expr syntax.Expr, visitor *inVisitor) syntax.Expr {
	if expr == nil {
		return expr
	}
	
	// Check for 'in' operations
	if op, ok := expr.(*syntax.Operation); ok {
		if op.Op == syntax.In {
			if transformed := t.convertInOperation(op, visitor, visitor.file); transformed != nil {
				visitor.changed = true
				return transformed
			}
		}
		// Transform operands recursively
		if op.X != nil {
			op.X = t.transformExpr(op.X, visitor)
		}
		if op.Y != nil {
			op.Y = t.transformExpr(op.Y, visitor)
		}
	}
	
	// Handle other expression types that might contain sub-expressions
	switch e := expr.(type) {
	case *syntax.CallExpr:
		for i, arg := range e.ArgList {
			e.ArgList[i] = t.transformExpr(arg, visitor)
		}
	case *syntax.ParenExpr:
		e.X = t.transformExpr(e.X, visitor)
	case *syntax.ListExpr:
		for i, elem := range e.ElemList {
			e.ElemList[i] = t.transformExpr(elem, visitor)
		}
	}
	
	return expr
}

// convertInOperation converts "item in collection" to appropriate Go code
func (t *InOperatorTransform) convertInOperation(op *syntax.Operation, visitor *inVisitor, file *syntax.File) syntax.Expr {
	pos := op.Pos()
	
	// Determine the type of operation based on the container (op.Y)
	containerType := t.inferContainerType(op.Y, visitor.ctx)
	
	switch containerType {
	case "string":
		return t.createStringContainsCall(op, visitor, pos)
	case "slice":
		return t.createSliceContainsCall(op, visitor, pos)
	case "map":
		return t.createMapContainsCall(op, visitor, pos)
	default:
		// Try to determine at runtime or fall back to string
		return t.createStringContainsCall(op, visitor, pos)
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
func (t *InOperatorTransform) createStringContainsCall(op *syntax.Operation, visitor *inVisitor, pos syntax.Pos) syntax.Expr {
	visitor.needsStringsImport = true
	
	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)
	
	containsName := &syntax.Name{Value: "Contains"}
	containsName.SetPos(pos)
	
	stringsContains := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: containsName,
	}
	stringsContains.SetPos(pos)
	
	// Ensure arguments have proper position info
	if op.Y != nil {
		op.Y.SetPos(pos)
	}
	if op.X != nil {
		op.X.SetPos(pos)
	}
	
	call := &syntax.CallExpr{
		Fun:     stringsContains,
		ArgList: []syntax.Expr{op.Y, op.X}, // Y is container, X is item
	}
	call.SetPos(pos)
	
	return call
}

// createSliceContainsCall creates slices.Contains(container, item)
func (t *InOperatorTransform) createSliceContainsCall(op *syntax.Operation, visitor *inVisitor, pos syntax.Pos) syntax.Expr {
	visitor.needsSlicesImport = true
	
	slicesName := &syntax.Name{Value: "slices"}
	slicesName.SetPos(pos)
	
	containsName := &syntax.Name{Value: "Contains"}
	containsName.SetPos(pos)
	
	slicesContains := &syntax.SelectorExpr{
		X:   slicesName,
		Sel: containsName,
	}
	slicesContains.SetPos(pos)
	
	// Ensure arguments have proper position info
	if op.Y != nil {
		op.Y.SetPos(pos)
	}
	if op.X != nil {
		op.X.SetPos(pos)
	}
	
	call := &syntax.CallExpr{
		Fun:     slicesContains,
		ArgList: []syntax.Expr{op.Y, op.X}, // Y is container, X is item
	}
	call.SetPos(pos)
	
	return call
}

// createMapContainsCall creates map key existence check (simplified for now)
func (t *InOperatorTransform) createMapContainsCall(op *syntax.Operation, visitor *inVisitor, pos syntax.Pos) syntax.Expr {
	// For now, fall back to treating as slice
	// TODO: Implement proper map[key] existence check
	return t.createSliceContainsCall(op, visitor, pos)
}

func (t *InOperatorTransform) hasImport(file *syntax.File, name string) bool {
	if name[0] != '"' {
		name = "\"" + name + "\""
	}
	for _, decl := range file.DeclList {
		if importDecl, ok := decl.(*syntax.ImportDecl); ok {
			if importDecl.Path != nil && importDecl.Path.Value == name {
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

func init() {
	RegisterTransformer(&InOperatorTransform{})
}