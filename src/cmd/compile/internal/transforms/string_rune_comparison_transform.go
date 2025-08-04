// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// StringRuneComparisonTransform handles comparison between strings and runes
// Transforms "你" == '你' into "你" == string('你')
type StringRuneComparisonTransform struct {
	ctx *TransformContext
}

func (t *StringRuneComparisonTransform) Name() string {
	return "string_rune_comparison_transform"
}

func (t *StringRuneComparisonTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	t.ctx = ctx // Store context for use in other methods
	visitor := &stringRuneVisitor{transform: t, ctx: ctx}
	syntax.Walk(file, visitor)
	return visitor.changed
}

func (t *StringRuneComparisonTransform) transformDecl(decl syntax.Decl, ctx *TransformContext) syntax.Decl {
	switch d := decl.(type) {
	case *syntax.FuncDecl:
		if newBody := t.transformStmt(d.Body, ctx); newBody != d.Body {
			newDecl := *d
			if blockStmt, ok := newBody.(*syntax.BlockStmt); ok {
				newDecl.Body = blockStmt
			}
			return &newDecl
		}
	case *syntax.VarDecl:
		if d.Values != nil {
			if newValues := t.transformExpr(d.Values, ctx); newValues != d.Values {
				newDecl := *d
				newDecl.Values = newValues
				return &newDecl
			}
		}
	}
	return decl
}

func (t *StringRuneComparisonTransform) transformStmt(stmt syntax.Stmt, ctx *TransformContext) syntax.Stmt {
	if stmt == nil {
		return nil
	}

	switch s := stmt.(type) {
	case *syntax.BlockStmt:
		changed := false
		newList := make([]syntax.Stmt, len(s.List))
		for i, stmt := range s.List {
			newStmt := t.transformStmt(stmt, ctx)
			newList[i] = newStmt
			if newStmt != stmt {
				changed = true
			}
		}
		if changed {
			newBlock := *s
			newBlock.List = newList
			return &newBlock
		}
	case *syntax.ExprStmt:
		if newExpr := t.transformExpr(s.X, ctx); newExpr != s.X {
			newStmt := *s
			newStmt.X = newExpr
			return &newStmt
		}
	case *syntax.AssignStmt:
		lhsChanged := false
		rhsChanged := false
		newLhs := t.transformExpr(s.Lhs, ctx)
		newRhs := t.transformExpr(s.Rhs, ctx)
		if newLhs != s.Lhs {
			lhsChanged = true
		}
		if newRhs != s.Rhs {
			rhsChanged = true
		}
		if lhsChanged || rhsChanged {
			newStmt := *s
			newStmt.Lhs = newLhs
			newStmt.Rhs = newRhs
			return &newStmt
		}
	case *syntax.ReturnStmt:
		if s.Results != nil {
			if newResults := t.transformExpr(s.Results, ctx); newResults != s.Results {
				newStmt := *s
				newStmt.Results = newResults
				return &newStmt
			}
		}
	case *syntax.IfStmt:
		condChanged := false
		thenChanged := false
		elseChanged := false

		newCond := t.transformExpr(s.Cond, ctx)
		if newCond != s.Cond {
			condChanged = true
		}

		newThen := t.transformStmt(s.Then, ctx)
		if newThen != s.Then {
			thenChanged = true
		}

		var newElse syntax.Stmt
		if s.Else != nil {
			newElse = t.transformStmt(s.Else, ctx)
			if newElse != s.Else {
				elseChanged = true
			}
		}

		if condChanged || thenChanged || elseChanged {
			newStmt := *s
			newStmt.Cond = newCond
			if blockStmt, ok := newThen.(*syntax.BlockStmt); ok {
				newStmt.Then = blockStmt
			}
			newStmt.Else = newElse
			return &newStmt
		}
	}
	return stmt
}

func (t *StringRuneComparisonTransform) transformExpr(expr syntax.Expr, ctx *TransformContext) syntax.Expr {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *syntax.Operation:
		// Check for equality/inequality operations
		if e.Op == syntax.Eql || e.Op == syntax.Neq {
			newX := t.transformExpr(e.X, ctx)
			newY := t.transformExpr(e.Y, ctx)

			// Check if we have string vs rune comparison
			transformed := t.transformStringRuneComparison(newX, newY, e.Op)
			if transformed != nil {
				return transformed
			}

			// If no special transformation, check if operands changed
			if newX != e.X || newY != e.Y {
				newOp := *e
				newOp.X = newX
				newOp.Y = newY
				return &newOp
			}
		} else {
			// For non-comparison operations, transform operands
			newX := t.transformExpr(e.X, ctx)
			var newY syntax.Expr
			if e.Y != nil {
				newY = t.transformExpr(e.Y, ctx)
			}

			if newX != e.X || (e.Y != nil && newY != e.Y) {
				newOp := *e
				newOp.X = newX
				newOp.Y = newY
				return &newOp
			}
		}
	case *syntax.CallExpr:
		// Transform function and arguments
		funChanged := false
		argsChanged := false
		newFun := t.transformExpr(e.Fun, ctx)
		if newFun != e.Fun {
			funChanged = true
		}
		var newArgList []syntax.Expr
		if e.ArgList != nil {
			newArgList = make([]syntax.Expr, len(e.ArgList))
			for i, arg := range e.ArgList {
				newArg := t.transformExpr(arg, ctx)
				newArgList[i] = newArg
				if newArg != arg {
					argsChanged = true
				}
			}
		}
		if funChanged || argsChanged {
			newCall := *e
			newCall.Fun = newFun
			newCall.ArgList = newArgList
			return &newCall
		}
	case *syntax.ParenExpr:
		if newX := t.transformExpr(e.X, ctx); newX != e.X {
			newParen := *e
			newParen.X = newX
			return &newParen
		}
	}
	return expr
}

// transformStringRuneComparison handles string vs rune comparisons
func (t *StringRuneComparisonTransform) transformStringRuneComparison(left, right syntax.Expr, op syntax.Operator) syntax.Expr {
	leftIsString := t.isStringType(left)
	rightIsString := t.isStringType(right)
	leftIsRune := t.isRuneType(left)
	rightIsRune := t.isRuneType(right)

	var transformedExpr syntax.Expr

	// Transform when we have string vs rune comparison (literals or variables)
	if leftIsString && rightIsRune {
		// string_var == rune_var -> string_var == string(rune_var)
		stringConversion := t.createStringConversion(right)
		transformedExpr = &syntax.Operation{
			Op: op,
			X:  left,
			Y:  stringConversion,
		}
	} else if leftIsRune && rightIsString {
		// rune_var == string_var -> string(rune_var) == string_var
		stringConversion := t.createStringConversion(left)
		transformedExpr = &syntax.Operation{
			Op: op,
			X:  stringConversion,
			Y:  right,
		}
	}

	if transformedExpr != nil {
		transformedExpr.SetPos(left.Pos())
		return transformedExpr
	}

	return nil
}

// isStringType checks if an expression is of string type (literal or variable)
func (t *StringRuneComparisonTransform) isStringType(expr syntax.Expr) bool {
	// Check for string literal
	if lit, ok := expr.(*syntax.BasicLit); ok {
		return lit.Kind == syntax.StringLit
	}
	// Check for variable with string type
	if name, ok := expr.(*syntax.Name); ok && t.ctx != nil && t.ctx.Types != nil {
		varType := t.ctx.Types[name.Value]
		return varType == "string"
	}
	return false
}

// isRuneType checks if an expression is of rune type (literal or variable)
func (t *StringRuneComparisonTransform) isRuneType(expr syntax.Expr) bool {
	// Check for rune literal
	if lit, ok := expr.(*syntax.BasicLit); ok {
		return lit.Kind == syntax.RuneLit
	}
	// Check for variable with rune type
	if name, ok := expr.(*syntax.Name); ok && t.ctx != nil && t.ctx.Types != nil {
		varType := t.ctx.Types[name.Value]
		return varType == "rune" || varType == "int32" // rune is alias for int32
	}
	return false
}

// isStringLiteral checks if an expression is a string literal
func (t *StringRuneComparisonTransform) isStringLiteral(expr syntax.Expr) bool {
	if lit, ok := expr.(*syntax.BasicLit); ok {
		return lit.Kind == syntax.StringLit
	}
	return false
}

// isRuneLiteral checks if an expression is a rune literal
func (t *StringRuneComparisonTransform) isRuneLiteral(expr syntax.Expr) bool {
	if lit, ok := expr.(*syntax.BasicLit); ok {
		return lit.Kind == syntax.RuneLit
	}
	return false
}

// createStringConversion creates string(rune) conversion
func (t *StringRuneComparisonTransform) createStringConversion(runeExpr syntax.Expr) syntax.Expr {
	pos := runeExpr.Pos()

	stringName := &syntax.Name{Value: "string"}
	stringName.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     stringName,
		ArgList: []syntax.Expr{runeExpr},
	}
	call.SetPos(pos)

	return call
}

type stringRuneVisitor struct {
	transform *StringRuneComparisonTransform
	ctx       *TransformContext
	changed   bool
}

func (v *stringRuneVisitor) Visit(node syntax.Node) syntax.Visitor {
	switch n := node.(type) {
	case *syntax.Operation:
		// Check for equality/inequality operations
		if n.Op == syntax.Eql || n.Op == syntax.Neq {
			// Check if we have string vs rune comparison
			transformed := v.transform.transformStringRuneComparison(n.X, n.Y, n.Op)
			if transformed != nil {
				// Replace the operation in place
				*n = *transformed.(*syntax.Operation)
				v.changed = true
			}
		}
	}
	return v
}

func init() {
	RegisterTransformer(&StringRuneComparisonTransform{})
}
