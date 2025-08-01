// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// InOperatorTransform handles the 'in' operator for strings and collections
// Transforms expressions like "hello" in str to strings.Contains(str, "hello")
// and item in slice to slices.Contains(slice, item)
type InOperatorTransform struct{}

type inVisitor struct {
	transform           *InOperatorTransform
	ctx                 *TransformContext
	changed             bool
	needsStringsImport  bool
	needsSlicesImport   bool
}

func (t *InOperatorTransform) Name() string {
	return "in_operator_transform"
}

func (t *InOperatorTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &inVisitor{transform: t, ctx: ctx}
	
	// Walk all declarations and transform them
	for _, decl := range file.DeclList {
		t.walkAndTransform(decl, visitor)
	}
	
	// Add imports if needed
	if visitor.needsStringsImport && !t.hasImport(file, "strings") {
		t.addImport(file, "strings")
		visitor.changed = true
	}
	if visitor.needsSlicesImport && !t.hasImport(file, "slices") {
		t.addImport(file, "slices")
		visitor.changed = true
	}
	
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
			if transformed := t.convertInOperation(op, visitor); transformed != nil {
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
func (t *InOperatorTransform) convertInOperation(op *syntax.Operation, visitor *inVisitor) syntax.Expr {
	pos := op.Pos()
	
	// For now, assume string containment: "substr" in "string" -> strings.Contains("string", "substr")
	// Later we can add type checking to determine if it's a slice/map/etc.
	
	// Create strings.Contains call
	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)
	
	containsName := &syntax.Name{Value: "Contains"}
	containsName.SetPos(pos)
	
	stringsContains := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: containsName,
	}
	stringsContains.SetPos(pos)
	
	// strings.Contains(haystack, needle) - note the argument order is swapped
	call := &syntax.CallExpr{
		Fun:     stringsContains,
		ArgList: []syntax.Expr{op.Y, op.X}, // Y is the container, X is the item
	}
	call.SetPos(pos)
	
	visitor.needsStringsImport = true
	return call
}

// hasImport checks if the file already imports the specified package
func (t *InOperatorTransform) hasImport(file *syntax.File, pkgName string) bool {
	quotedName := "\"" + pkgName + "\""
	for _, decl := range file.DeclList {
		if importDecl, ok := decl.(*syntax.ImportDecl); ok {
			if importDecl.Path != nil && importDecl.Path.Value == quotedName {
				return true
			}
		}
	}
	return false
}

// addImport adds a package import
func (t *InOperatorTransform) addImport(file *syntax.File, pkgName string) {
	newImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: "\"" + pkgName + "\"",
			Kind:  syntax.StringLit,
		},
	}

	// Find where to insert the import (after existing imports)
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
	newDeclList = append(newDeclList, newImport)
	newDeclList = append(newDeclList, file.DeclList[insertPos:]...)
	file.DeclList = newDeclList
}

func init() {
	RegisterTransformer(&InOperatorTransform{})
}