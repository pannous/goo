// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

// only include in cmd/go build with transforms enabled!

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
type StringConcatTransform struct{}

func (t *StringConcatTransform) Name() string {
	return "string_concat"
}

func (t *StringConcatTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	println("StringConcatTransform.Transform called")
	//return false // no Transform today:)
	needsFmtImport := !t.hasImport(file, "fmt") && t.hasStringConcat(file)
	if needsFmtImport {
		println("Adding fmt import")
		t.addFmtImport(file)
	}

	// Transform expressions
	changed := false
	for _, decl := range file.DeclList {
		if funcDecl, ok := decl.(*syntax.FuncDecl); ok {
			if t.transformFuncBody(funcDecl.Body, ctx) {
				changed = true
			}
		}
	}
	return changed
}

func (t *StringConcatTransform) transformFuncBody(stmt syntax.Stmt, ctx *TransformContext) bool {
	if stmt == nil {
		return false
	}

	// Only process if it's a BlockStmt (function body) WHY?
	if _, ok := stmt.(*syntax.BlockStmt); !ok {
		return false
	}

	return t.walkStmt(stmt, ctx)
}

// walkStmt walks a statement and transforms any string concatenations
func (t *StringConcatTransform) walkStmt(stmt syntax.Stmt, ctx *TransformContext) bool {
	if stmt == nil {
		return false
	}

	changed := false
	switch s := stmt.(type) {
	case *syntax.BlockStmt:
		for _, stmt := range s.List {
			if t.walkStmt(stmt, ctx) {
				changed = true
			}
		}
	case *syntax.ExprStmt:
		if t.walkExpr(s.X, ctx) {
			changed = true
		}
	case *syntax.AssignStmt:
		if s.Lhs != nil && t.walkExpr(s.Lhs, ctx) {
			changed = true
		}
		if s.Rhs != nil && t.walkExpr(s.Rhs, ctx) {
			changed = true
		}
	case *syntax.IfStmt:
		if s.Init != nil && t.walkStmt(s.Init, ctx) {
			changed = true
		}
		if t.walkExpr(s.Cond, ctx) {
			changed = true
		}
		if t.walkStmt(s.Then, ctx) {
			changed = true
		}
		if s.Else != nil && t.walkStmt(s.Else, ctx) {
			changed = true
		}
	case *syntax.ForStmt:
		if s.Init != nil && t.walkStmt(s.Init, ctx) {
			changed = true
		}
		if s.Cond != nil && t.walkExpr(s.Cond, ctx) {
			changed = true
		}
		if s.Post != nil && t.walkStmt(s.Post, ctx) {
			changed = true
		}
		if t.walkStmt(s.Body, ctx) {
			changed = true
		}
	case *syntax.ReturnStmt:
		if s.Results != nil && t.walkExpr(s.Results, ctx) {
			changed = true
		}
	// Add more cases for other statement types as needed
	case *syntax.CheckStmt:
		changed = t.walkExpr(s.Cond, ctx)
	//case *syntax.DeclStmt:
	default:
		changed = false
		fmt.Printf("Unhandled stmt node type: %T\n", s)
	}
	if changed {
		//fmt.Printf("Transformed statement: %s\n", syntax.String(stmt))
	} else {
		//print("UN Transformed statement: \n")
	}
	return changed
}

//func (p *printer) stmt(n syntax.Stmt) {
//    switch n := n.(type) {
//    case *syntax.CheckStmt:
//        p.print("check ")
//        p.expr(n.X)
//    }
//}

// walkExpr walks an expression and transforms any string concatenations
func (t *StringConcatTransform) walkExpr(expr syntax.Expr, ctx *TransformContext) bool {
	if expr == nil {
		return false
	}

	changed := false
	switch e := expr.(type) {
	case *syntax.Operation:
		if t.walkExpr(e.X, ctx) {
			changed = true
		}
		if e.Y != nil && t.walkExpr(e.Y, ctx) {
			changed = true
		}
		// Check if this operation needs transformation
		if transformed := t.transformConcatOperation(e, ctx); transformed != nil {
			// Copy the transformed expression back
			if newOp, ok := transformed.(*syntax.Operation); ok {
				e.X = newOp.X
				e.Y = newOp.Y
				changed = true
			}
		}
	case *syntax.CallExpr:
		if t.walkExpr(e.Fun, ctx) {
			changed = true
		}
		if e.ArgList != nil {
			for _, arg := range e.ArgList {
				if t.walkExpr(arg, ctx) {
					changed = true
				}
			}
		}
	case *syntax.SelectorExpr:
		if t.walkExpr(e.X, ctx) {
			changed = true
		}
	case *syntax.IndexExpr:
		if t.walkExpr(e.X, ctx) {
			changed = true
		}
		if t.walkExpr(e.Index, ctx) {
			changed = true
		}
	case *syntax.Name:
		changed = false
	case *syntax.BasicLit:
		changed = e.Kind != syntax.StringLit
	case *syntax.ListExpr:
		for _, elem := range e.ElemList {
			if t.walkExpr(elem, ctx) {
				changed = true
			}
		}
	default:
		fmt.Printf("Unhandled expr node type: %T\n", e)
	}
	if changed {
		fmt.Printf("Transformed expression: %s\n", syntax.String(expr))
	}
	return changed
}

// transformConcatOperation checks if this is a string concatenation with a non-string operand
// and wraps the non-string operand with fmt.Sprintf if it's an integer.
func (t *StringConcatTransform) transformConcatOperation(op *syntax.Operation, ctx *TransformContext) syntax.Expr {
	if op.Op != syntax.Add {
		return nil
	}

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

// isStringLiteral returns true if the expression is a string literal.
func (t *StringConcatTransform) isStringLiteral(expr syntax.Expr) bool {
	if basic, ok := expr.(*syntax.BasicLit); ok {
		return basic.Kind == syntax.StringLit
	}
	return false
}

// isStringExpression returns true if the expression could be a string (literal or variable)
func (t *StringConcatTransform) isStringExpression(expr syntax.Expr, ctx *TransformContext) bool {
	// Check if it's a string literal
	if t.isStringLiteral(expr) {
		return true
	}

	// Check if it's a string variable (based on context)
	if name, ok := expr.(*syntax.Name); ok {
		if ctx != nil && ctx.Types != nil {
			varType := ctx.Types[name.Value]
			return varType == "string"
		}
		println("UNKNOWN variable type for", name.Value)
		// Conservative: assume variables might be strings
		return true
	}

	// String expressions like function calls returning strings, etc.
	if _, ok := expr.(*syntax.CallExpr); ok {
		return true // Function might return string
	}

	if _, ok := expr.(*syntax.SelectorExpr); ok {
		return true // Field might be string
	}

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
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X: &syntax.Name{
				Value: "fmt",
			},
			Sel: &syntax.Name{
				Value: "Sprintf",
			},
		},
		ArgList: []syntax.Expr{
			&syntax.BasicLit{
				Kind:  syntax.StringLit,
				Value: "\"%v\"",
			},
			expr,
		},
	}
}

// hasStringConcat checks if the file contains string + int concatenations
func (t *StringConcatTransform) hasStringConcat(file *syntax.File) bool {
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
func (t *StringConcatTransform) bodyHasStringConcat(stmt syntax.Stmt) bool {
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
func (t *StringConcatTransform) checkForStringConcat(stmt syntax.Stmt) bool {
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
func (t *StringConcatTransform) checkExprForStringConcat(expr syntax.Expr) bool {
	if expr == nil {
		return false
	}

	switch e := expr.(type) {
	case *syntax.Operation:
		if e.Op == syntax.Add {
			leftIsString := t.isStringExpression(e.X, nil)
			rightIsString := t.isStringExpression(e.Y, nil)
			if (leftIsString && t.mightBeNumericVariable(e.Y, nil)) ||
				(rightIsString && t.mightBeNumericVariable(e.X, nil)) {
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

// Register the transformer during package initialization
var msg_shown = false

func init() {
	//do_register := false
	do_register := true
	//do_register := !msg_shown // Only register if not already shown
	if do_register {
		println("Registering string concat transformer!")
		RegisterTransformer(&StringConcatTransform{}) // per context?
	} else {
		//println("NOT Registering string concat transformer")
	}
	msg_shown = true
}
