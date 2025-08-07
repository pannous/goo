// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
	"strings"
)

// StringFormatTransform handles string formatting with % operator
// Transforms expressions like "my %v modulo" % "cool" to fmt.Sprintf("my %v modulo", "cool")
type StringFormatTransform struct{
	needsFmtImport bool
}

func (t *StringFormatTransform) Name() string {
	return "string_format_transform"
}

func (t *StringFormatTransform) Priority() int {
	return 100 // Default priority - between list methods (50) and lambda (200)
}

// NodeTransformer interface implementation
func (t *StringFormatTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
	// Only handle Operation nodes with Rem (%) operator for string formatting
	if op, ok := node.(*syntax.Operation); ok {
		return op.Op == syntax.Rem && t.isStringFormatOperation(op)
	}
	return false
}

func (t *StringFormatTransform) TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node {
	if op, ok := node.(*syntax.Operation); ok && op.Op == syntax.Rem {
		if t.isStringFormatOperation(op) {
			t.needsFmtImport = true
			return t.convertFormatOperation(op)
		}
	}
	return nil
}

func (t *StringFormatTransform) PostProcess(file *syntax.File, ctx *TransformContext) bool {
	// Add fmt import if needed
	if t.needsFmtImport && !t.hasImport(file, "fmt") {
		t.addFmtImport(file)
		t.needsFmtImport = false
		return true
	}
	return false
}

// Legacy Transform method for backward compatibility - not used in new architecture
func (t *StringFormatTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	// This method is kept for interface compatibility but not used
	// The new NodeTransformer interface methods are used instead
	return false
}

func (t *StringFormatTransform) isStringFormatOperation(op *syntax.Operation) bool {
	// Check if left operand is a string literal with format placeholders
	if lit, ok := op.X.(*syntax.BasicLit); ok && lit.Kind == syntax.StringLit {
		// Check for format specifiers like %v, %s, %d etc
		return strings.Contains(lit.Value, "%")
	}
	return false
}

func (t *StringFormatTransform) convertFormatOperation(op *syntax.Operation) syntax.Expr {
	pos := op.Pos()
	
	// Extract the format string and arguments
	formatString, args := t.extractFormatChain(op)
	
	// Create fmt.Sprintf call
	fmtName := &syntax.Name{Value: "fmt"}
	fmtName.SetPos(pos)
	
	sprintfName := &syntax.Name{Value: "Sprintf"}
	sprintfName.SetPos(pos)
	
	selectorExpr := &syntax.SelectorExpr{
		X:   fmtName,
		Sel: sprintfName,
	}
	selectorExpr.SetPos(pos)
	
	// Create argument list: format string + args
	argList := make([]syntax.Expr, 0, len(args)+1)
	argList = append(argList, formatString)
	argList = append(argList, args...)
	
	callExpr := &syntax.CallExpr{
		Fun:     selectorExpr,
		ArgList: argList,
	}
	callExpr.SetPos(pos)
	
	return callExpr
}

func (t *StringFormatTransform) extractFormatChain(op *syntax.Operation) (syntax.Expr, []syntax.Expr) {
	if op.Op != syntax.Rem {
		return op, []syntax.Expr{}
	}
	
	var args []syntax.Expr
	var format syntax.Expr = op.X
	current := op
	
	// Collect all arguments from right to left
	for current.Op == syntax.Rem {
		args = append([]syntax.Expr{current.Y}, args...) // prepend to maintain order
		
		// Check if the left operand is also a % operation
		if leftOp, ok := current.X.(*syntax.Operation); ok && leftOp.Op == syntax.Rem {
			current = leftOp
		} else {
			format = current.X
			break
		}
	}
	
	return format, args
}

// hasImport checks if a file already imports a package
func (t *StringFormatTransform) hasImport(file *syntax.File, packageName string) bool {
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

// addFmtImport adds "fmt" import to the file
func (t *StringFormatTransform) addFmtImport(file *syntax.File) {
	if t.hasImport(file, "fmt") {
		return
	}

	fmtImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: "\"fmt\"",
			Kind:  syntax.StringLit,
		},
	}
	fmtImport.SetPos(syntax.Pos{})

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
	newDeclList = append(newDeclList, fmtImport)
	newDeclList = append(newDeclList, file.DeclList[insertPos:]...)
	file.DeclList = newDeclList
}

func init() {
	RegisterTransformer(&StringFormatTransform{})
}