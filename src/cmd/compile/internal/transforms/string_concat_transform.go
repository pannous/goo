// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

// only include in cmd/compile build with transforms enabled!

package transforms

import (
	"cmd/compile/internal/syntax"
)

// StringConcatTransform handles automatic string conversion in concatenation.
// It transforms expressions like
// "result:" + z --> "result:" + strconv.Itoa(z)  // deprecated vs:
// "result:" + z --> "result:" + fmt.Sprintf("%v", z) // works!
// "result:" + z --> "result:" + z.String()  // NOT for int!

type StringConcatTransform struct{}

// concatVisitor implements syntax.Visitor to transform string concatenations
type concatVisitor struct {
	transform      *StringConcatTransform
	ctx            *TransformContext //ctx.Types[name.Value] // e.g., "int", "string" guessed in transform.go
	changed        bool
	needsFmtImport bool
}

func (t *StringConcatTransform) Name() string {
	return "string_concat"
}

func (t *StringConcatTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	println("StringConcatTransform.Transform called")

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

	// Check for string concatenation operations
	if op, ok := node.(*syntax.Operation); ok && op.Op == syntax.Add {
		println("Found ADD operation:", syntax.String(op))
		if transformed := v.transform.transformConcatOperation(op, v.ctx); transformed != nil {
			println("TRANSFORMING:", syntax.String(op), "->", syntax.String(transformed))
			if newOp, ok := transformed.(*syntax.Operation); ok {
				op.X = newOp.X
				op.Y = newOp.Y
				v.changed = true
				v.needsFmtImport = true
			}
		} else {
			println("NOT transforming:", syntax.String(op))
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

	leftIsString := t.isStringExpression(op.X, ctx)
	rightIsString := t.isStringExpression(op.Y, ctx)
	println("  leftIsString:", leftIsString, "rightIsString:", rightIsString)

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
			println("mightBeNumeric fallback for", name.Value)
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
		println("mightBeNumeric fallback for", name.Value)
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
		println("Registering string concat transformer!") // reactivate this when //go:build transforms is there
		RegisterTransformer(&StringConcatTransform{})     // per context?
	} else {
		//println("NOT Registering string concat transformer")
	}
	msg_shown = true
}
