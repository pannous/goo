// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transforms

import (
	"cmd/compile/internal/syntax"
	"fmt"
)

// Transformer represents a syntax tree transformation stage.
type Transformer interface {
	Transform(file *syntax.File) bool
	Name() string
}

// TransformRegistry holds all registered transformers.
var TransformRegistry []Transformer

// RegisterTransformer adds a transformer to the global registry.
func RegisterTransformer(t Transformer) {
	TransformRegistry = append(TransformRegistry, t)
}

// ApplyTransformations runs all registered transformers on the syntax tree.
func ApplyTransformations(files []*syntax.File) {
	for _, transformer := range TransformRegistry {
		for _, file := range files {
			if transformer.Transform(file) {
				// Optional: add debug logging here
			}
		}
	}
}

// StringConcatTransformer handles automatic string conversion in concatenation.
// It transforms expressions like "result:" + z to "result:" + strconv.Itoa(z)
// when z is an integer type.
type StringConcatTransformer struct{}

func (t *StringConcatTransformer) Name() string {
	return "string_concat"
}

func (t *StringConcatTransformer) Transform(file *syntax.File) bool {
	// First, check if we need to add strconv import
	needsStrconv := t.hasStringConcat(file)
	if needsStrconv {
		t.addStrconvImport(file)
	}
	needsFmtImport := !t.hasImport(file, "fmt") && t.hasStringConcat(file)
	if needsFmtImport {
		t.addFmtImport(file)
	}

	// Transform expressions
	changed := false
	for _, decl := range file.DeclList {
		if funcDecl, ok := decl.(*syntax.FuncDecl); ok {
			if t.transformFuncBody(funcDecl.Body) {
				changed = true
			}
		}
	}
	return changed
}

func (t *StringConcatTransformer) transformFuncBody(stmt syntax.Stmt) bool {
	if stmt == nil {
		return false
	}

	// Only process if it's a BlockStmt (function body)
	if _, ok := stmt.(*syntax.BlockStmt); !ok {
		return false
	}

	return t.walkStmt(stmt)
}

// walkStmt walks a statement and transforms any string concatenations
func (t *StringConcatTransformer) walkStmt(stmt syntax.Stmt) bool {
	if stmt == nil {
		return false
	}

	changed := false
	switch s := stmt.(type) {
	case *syntax.BlockStmt:
		for _, stmt := range s.List {
			if t.walkStmt(stmt) {
				changed = true
			}
		}
	case *syntax.ExprStmt:
		if t.walkExpr(s.X) {
			changed = true
		}
	case *syntax.AssignStmt:
		if s.Lhs != nil && t.walkExpr(s.Lhs) {
			changed = true
		}
		if s.Rhs != nil && t.walkExpr(s.Rhs) {
			changed = true
		}
	case *syntax.IfStmt:
		if s.Init != nil && t.walkStmt(s.Init) {
			changed = true
		}
		if t.walkExpr(s.Cond) {
			changed = true
		}
		if t.walkStmt(s.Then) {
			changed = true
		}
		if s.Else != nil && t.walkStmt(s.Else) {
			changed = true
		}
	case *syntax.ForStmt:
		if s.Init != nil && t.walkStmt(s.Init) {
			changed = true
		}
		if s.Cond != nil && t.walkExpr(s.Cond) {
			changed = true
		}
		if s.Post != nil && t.walkStmt(s.Post) {
			changed = true
		}
		if t.walkStmt(s.Body) {
			changed = true
		}
	case *syntax.ReturnStmt:
		if s.Results != nil && t.walkExpr(s.Results) {
			changed = true
		}
	}
	return changed
}

// walkExpr walks an expression and transforms any string concatenations
func (t *StringConcatTransformer) walkExpr(expr syntax.Expr) bool {
	if expr == nil {
		return false
	}

	changed := false
	switch e := expr.(type) {
	case *syntax.Operation:
		if t.walkExpr(e.X) {
			changed = true
		}
		if e.Y != nil && t.walkExpr(e.Y) {
			changed = true
		}
		// Check if this operation needs transformation
		if transformed := t.transformConcatOperation(e); transformed != nil {
			// Copy the transformed expression back
			if newOp, ok := transformed.(*syntax.Operation); ok {
				e.X = newOp.X
				e.Y = newOp.Y
				changed = true
			}
		}
	case *syntax.CallExpr:
		if t.walkExpr(e.Fun) {
			changed = true
		}
		if e.ArgList != nil {
			for _, arg := range e.ArgList {
				if t.walkExpr(arg) {
					changed = true
				}
			}
		}
	case *syntax.SelectorExpr:
		if t.walkExpr(e.X) {
			changed = true
		}
	case *syntax.IndexExpr:
		if t.walkExpr(e.X) {
			changed = true
		}
		if t.walkExpr(e.Index) {
			changed = true
		}
	case *syntax.ListExpr:
		for _, elem := range e.ElemList {
			if t.walkExpr(elem) {
				changed = true
			}
		}
	}
	return changed
}

// transformConcatOperation checks if this is a string concatenation with a non-string operand
// and wraps the non-string operand with strconv.Itoa if it's an integer.
func (t *StringConcatTransformer) transformConcatOperation(op *syntax.Operation) syntax.Expr {
	if op.Op != syntax.Add {
		return nil
	}

	// Check if either operand is a string literal
	leftIsString := t.isStringLiteral(op.X)
	rightIsString := t.isStringLiteral(op.Y)

	// Only transform if exactly one operand is a string and the other might be an integer
	if leftIsString && !rightIsString {
		if t.mightBeIntegerVariable(op.Y) {
			// Transform: "string" + var => "string" + strconv.Itoa(var)
			op.Y = t.createItoacCall(op.Y)
			return op
		}
	} else if rightIsString && !leftIsString {
		if t.mightBeIntegerVariable(op.X) {
			// Transform: var + "string" => strconv.Itoa(var) + "string"
			op.X = t.createItoacCall(op.X)
			return op
		}
	}

	return nil
}

// isStringLiteral returns true if the expression is a string literal.
func (t *StringConcatTransformer) isStringLiteral(expr syntax.Expr) bool {
	if basic, ok := expr.(*syntax.BasicLit); ok {
		return basic.Kind == syntax.StringLit
	}
	return false
}

// mightBeIntegerVariable returns true if the expression could be an integer variable.
// For now, we'll be conservative and only handle simple identifiers.
func (t *StringConcatTransformer) mightBeIntegerVariable(expr syntax.Expr) bool {
	_, ok := expr.(*syntax.Name)
	return ok
}

// createItoacCall creates a syntax tree for strconv.Itoa(expr).
func (t *StringConcatTransformer) createItoacCall(expr syntax.Expr) syntax.Expr {
	// Create strconv.Itoa(expr)
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X: &syntax.Name{
				Value: "strconv",
			},
			Sel: &syntax.Name{
				Value: "Itoa",
			},
		},
		ArgList: []syntax.Expr{expr},
	}
}

// hasStringConcat checks if the file contains string + int concatenations
func (t *StringConcatTransformer) hasStringConcat(file *syntax.File) bool {
	// todo: instead of parsing the whole file we can mark it in parser.go
	for _, decl := range file.DeclList {
		if funcDecl, ok := decl.(*syntax.FuncDecl); ok {
			if t.bodyHasStringConcat(funcDecl.Body) {
				return true
			}
		}
	}
	return false
}

// bodyHasStringConcat checks if the function body contains string + int concatenations
func (t *StringConcatTransformer) bodyHasStringConcat(stmt syntax.Stmt) bool {
	if stmt == nil {
		return false
	}

	// Handle the case where stmt is not a BlockStmt
	if blockStmt, ok := stmt.(*syntax.BlockStmt); ok {
		return t.checkForStringConcat(blockStmt)
	}

	return t.checkForStringConcat(stmt)
}

// checkForStringConcat recursively checks statements for string + int concatenations
func (t *StringConcatTransformer) checkForStringConcat(stmt syntax.Stmt) bool {
	if stmt == nil {
		return false
	}

	switch s := stmt.(type) {
	case *syntax.BlockStmt:
		for _, stmt := range s.List {
			if t.checkForStringConcat(stmt) {
				return true
			}
		}
	case *syntax.ExprStmt:
		return t.checkExprForStringConcat(s.X)
	case *syntax.AssignStmt:
		return (s.Lhs != nil && t.checkExprForStringConcat(s.Lhs)) ||
			(s.Rhs != nil && t.checkExprForStringConcat(s.Rhs))
	case *syntax.IfStmt:
		if s.Init != nil && t.checkForStringConcat(s.Init) {
			return true
		}
		if t.checkExprForStringConcat(s.Cond) {
			return true
		}
		if t.checkForStringConcat(s.Then) {
			return true
		}
		if s.Else != nil && t.checkForStringConcat(s.Else) {
			return true
		}
	}
	return false
}

// checkExprForStringConcat recursively checks expressions for string + int concatenations
func (t *StringConcatTransformer) checkExprForStringConcat(expr syntax.Expr) bool {
	if expr == nil {
		return false
	}

	switch e := expr.(type) {
	case *syntax.Operation:
		if e.Op == syntax.Add {
			leftIsString := t.isStringLiteral(e.X)
			rightIsString := t.isStringLiteral(e.Y)
			if (leftIsString && t.mightBeIntegerVariable(e.Y)) ||
				(rightIsString && t.mightBeIntegerVariable(e.X)) {
				return true
			}
		}
		return t.checkExprForStringConcat(e.X) || (e.Y != nil && t.checkExprForStringConcat(e.Y))
	case *syntax.CallExpr:
		if t.checkExprForStringConcat(e.Fun) {
			return true
		}
		if e.ArgList != nil {
			for _, arg := range e.ArgList {
				if t.checkExprForStringConcat(arg) {
					return true
				}
			}
		}
	}
	return false
}

func (t *StringConcatTransformer) addFmtImport(file *syntax.File) {
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
func (t *StringConcatTransformer) addStrconvImport(file *syntax.File) {
	// Check if strconv is already imported
	if t.hasImport(file, "strconv") {
		return
	}

	// Add strconv import
	strconvImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: "\"strconv\"",
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
	newDeclList = append(newDeclList, strconvImport)
	newDeclList = append(newDeclList, file.DeclList[insertPos:]...)
	file.DeclList = newDeclList
}

func (t *StringConcatTransformer) hasImport(file *syntax.File, name string) bool {
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

// Register the transformer during package initialization
func init() {
	//do_register := false
	do_register := true
	if do_register {
		fmt.Println("Registering string concat transformer")
		RegisterTransformer(&StringConcatTransformer{})
	} else {
		fmt.Println("NOT Registering string concat transformer")
	}
}
