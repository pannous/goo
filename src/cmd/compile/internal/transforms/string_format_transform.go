// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// StringFormatTransform handles string formatting with % operator
// Transforms expressions like "my %v modulo" % "cool" to fmt.Sprintf("my %v modulo", "cool")
type StringFormatTransform struct{}

type formatVisitor struct {
	transform      *StringFormatTransform
	ctx            *TransformContext
	changed        bool
	needsFmtImport bool
}

func (t *StringFormatTransform) Name() string {
	return "string_format_transform"
}

func (t *StringFormatTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &formatVisitor{transform: t, ctx: ctx}
	
	// Walk all declarations and transform them
	for _, decl := range file.DeclList {
		t.walkAndTransform(decl, visitor)
	}
	
	// Add fmt import if we made changes and it's needed
	if visitor.needsFmtImport && !t.hasImport(file, "fmt") {
		t.addFmtImport(file)
		visitor.changed = true
	}
	
	return visitor.changed
}

// walkAndTransform walks the AST and transforms format operations
func (t *StringFormatTransform) walkAndTransform(node syntax.Node, visitor *formatVisitor) {
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
func (t *StringFormatTransform) transformStmtList(stmts []syntax.Stmt, visitor *formatVisitor) {
	for _, stmt := range stmts {
		t.transformStmt(stmt, visitor)
	}
}

// transformStmt transforms a single statement
func (t *StringFormatTransform) transformStmt(stmt syntax.Stmt, visitor *formatVisitor) {
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

// transformExprList transforms a list of expressions
func (t *StringFormatTransform) transformExprList(exprs []syntax.Expr, visitor *formatVisitor) {
	for i, expr := range exprs {
		exprs[i] = t.transformExpr(expr, visitor)
	}
}

// transformExpr transforms a single expression
func (t *StringFormatTransform) transformExpr(expr syntax.Expr, visitor *formatVisitor) syntax.Expr {
	if expr == nil {
		return expr
	}
	
	// Check for % operations that could be string formatting
	if op, ok := expr.(*syntax.Operation); ok {
		if op.Op == syntax.Rem && visitor.isStringFormatOperation(op) {
			if transformed := visitor.convertFormatOperation(op); transformed != nil {
				visitor.changed = true
				visitor.needsFmtImport = true
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
		t.transformExprList(e.ArgList, visitor)
	case *syntax.ParenExpr:
		e.X = t.transformExpr(e.X, visitor)
	case *syntax.ListExpr:
		t.transformExprList(e.ElemList, visitor)
	}
	
	return expr
}


// isStringFormatOperation checks if this % operation is string formatting
func (v *formatVisitor) isStringFormatOperation(op *syntax.Operation) bool {
	// Check if this is part of a format chain by extracting the leftmost operand
	formatString, _ := v.extractFormatChain(op)
	return formatString != nil
}

// convertFormatOperation converts "format" % value to fmt.Sprintf("format", value)
// Also handles chained operations like "format" % val1 % val2
func (v *formatVisitor) convertFormatOperation(op *syntax.Operation) syntax.Expr {
	// Extract the format string and all values from the chain
	formatString, values := v.extractFormatChain(op)
	if formatString == nil {
		return nil
	}
	
	pos := op.Pos()
	
	// Create fmt identifier
	fmtName := &syntax.Name{Value: "fmt"}
	fmtName.SetPos(pos)
	
	// Create Sprintf identifier
	sprintfName := &syntax.Name{Value: "Sprintf"}
	sprintfName.SetPos(pos)
	
	// Create fmt.Sprintf selector
	fmtSprintf := &syntax.SelectorExpr{
		X:   fmtName,
		Sel: sprintfName,
	}
	fmtSprintf.SetPos(pos)
	
	// Build argument list: format string + all values
	argList := make([]syntax.Expr, 0, len(values)+1)
	argList = append(argList, formatString)
	argList = append(argList, values...)
	
	// Create fmt.Sprintf call with format string and all values
	call := &syntax.CallExpr{
		Fun:     fmtSprintf,
		ArgList: argList,
	}
	call.SetPos(pos)
	
	return call
}

// extractFormatChain extracts format string and values from a chain of % operations
func (v *formatVisitor) extractFormatChain(op *syntax.Operation) (syntax.Expr, []syntax.Expr) {
	if op.Op != syntax.Rem {
		return nil, nil
	}
	
	var values []syntax.Expr
	
	// Walk left side to find the format string and collect values
	current := op
	for {
		// Add the right operand (value) to the beginning of values slice
		values = append([]syntax.Expr{current.Y}, values...)
		
		// Check if left side is another % operation
		if leftOp, ok := current.X.(*syntax.Operation); ok && leftOp.Op == syntax.Rem {
			current = leftOp
		} else {
			// Found the format string (leftmost operand)
			if v.transform.isStringExpression(current.X, v.ctx) {
				return current.X, values
			}
			// If leftmost isn't a string, this isn't a format operation
			return nil, nil
		}
	}
}

// isStringExpression returns true if the expression is definitely a string
func (t *StringFormatTransform) isStringExpression(expr syntax.Expr, ctx *TransformContext) bool {
	// Check if it's a string literal
	if basic, ok := expr.(*syntax.BasicLit); ok {
		return basic.Kind == syntax.StringLit
	}

	// Check if it's a string variable with known type
	if name, ok := expr.(*syntax.Name); ok {
		if ctx != nil && ctx.Types != nil {
			varType := ctx.Types[name.Value]
			return varType == "string"
		}
	}

	return false
}

// hasImport checks if the file already imports the specified package
func (t *StringFormatTransform) hasImport(file *syntax.File, pkgName string) bool {
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

// addFmtImport adds the fmt package import
func (t *StringFormatTransform) addFmtImport(file *syntax.File) {
	fmtImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: "\"fmt\"",
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
	newDeclList = append(newDeclList, fmtImport)
	newDeclList = append(newDeclList, file.DeclList[insertPos:]...)
	file.DeclList = newDeclList
}

func init() {
	RegisterTransformer(&StringFormatTransform{})
}