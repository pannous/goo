// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build transforms

// danger, conflict with src of vendor packages

package transforms

import (
	"cmd/compile/internal/syntax"
)

// AsCastTransform converts as cast expressions to type assertions
// Transforms: x as T -> x.(T)
type AsCastTransform struct{}

func (t *AsCastTransform) Name() string {
	return "as_cast_transform"
}

func (t *AsCastTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	changed := false

	// Transform all declarations
	for i, decl := range file.DeclList {
		if newDecl := t.transformDecl(decl); newDecl != decl {
			file.DeclList[i] = newDecl
			changed = true
		}
	}

	return changed
}

func (t *AsCastTransform) transformDecl(decl syntax.Decl) syntax.Decl {
	switch d := decl.(type) {
	case *syntax.FuncDecl:
		if newBody := t.transformStmt(d.Body); newBody != d.Body {
			newDecl := *d
			if blockStmt, ok := newBody.(*syntax.BlockStmt); ok {
				newDecl.Body = blockStmt
			}
			return &newDecl
		}
	case *syntax.VarDecl:
		if d.Values != nil {
			if newValues := t.transformExpr(d.Values); newValues != d.Values {
				newDecl := *d
				newDecl.Values = newValues
				return &newDecl
			}
		}
	}
	return decl
}

func (t *AsCastTransform) transformStmt(stmt syntax.Stmt) syntax.Stmt {
	if stmt == nil {
		return nil
	}

	switch s := stmt.(type) {
	case *syntax.BlockStmt:
		changed := false
		newList := make([]syntax.Stmt, len(s.List))
		for i, stmt := range s.List {
			newStmt := t.transformStmt(stmt)
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
		if newExpr := t.transformExpr(s.X); newExpr != s.X {
			newStmt := *s
			newStmt.X = newExpr
			return &newStmt
		}
	case *syntax.AssignStmt:
		lhsChanged := false
		rhsChanged := false
		newLhs := t.transformExpr(s.Lhs)
		newRhs := t.transformExpr(s.Rhs)
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
			if newResults := t.transformExpr(s.Results); newResults != s.Results {
				newStmt := *s
				newStmt.Results = newResults
				return &newStmt
			}
		}
		// Skip the complex statement transformations for now to avoid type issues
	}
	return stmt
}

func (t *AsCastTransform) transformExpr(expr syntax.Expr) syntax.Expr {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *syntax.AsCastExpr:
		// This is what we're looking for! Convert x as T to x.(T)
		return t.convertAsCastToAssert(e)
	case *syntax.CallExpr:
		funChanged := false
		argsChanged := false
		newFun := t.transformExpr(e.Fun)
		if newFun != e.Fun {
			funChanged = true
		}
		var newArgList []syntax.Expr
		if e.ArgList != nil {
			newArgList = make([]syntax.Expr, len(e.ArgList))
			for i, arg := range e.ArgList {
				newArg := t.transformExpr(arg)
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
	case *syntax.Operation:
		xChanged := false
		yChanged := false
		newX := t.transformExpr(e.X)
		if newX != e.X {
			xChanged = true
		}
		var newY syntax.Expr
		if e.Y != nil {
			newY = t.transformExpr(e.Y)
			if newY != e.Y {
				yChanged = true
			}
		}
		if xChanged || yChanged {
			newOp := *e
			newOp.X = newX
			newOp.Y = newY
			return &newOp
		}
	case *syntax.IndexExpr:
		xChanged := false
		indexChanged := false
		newX := t.transformExpr(e.X)
		if newX != e.X {
			xChanged = true
		}
		newIndex := t.transformExpr(e.Index)
		if newIndex != e.Index {
			indexChanged = true
		}
		if xChanged || indexChanged {
			newIndexExpr := *e
			newIndexExpr.X = newX
			newIndexExpr.Index = newIndex
			return &newIndexExpr
		}
	case *syntax.ParenExpr:
		if newX := t.transformExpr(e.X); newX != e.X {
			newParen := *e
			newParen.X = newX
			return &newParen
		}
	case *syntax.SelectorExpr:
		if newX := t.transformExpr(e.X); newX != e.X {
			newSelector := *e
			newSelector.X = newX
			return &newSelector
		}
	case *syntax.SliceExpr:
		changed := false
		newSlice := *e
		if newX := t.transformExpr(e.X); newX != e.X {
			newSlice.X = newX
			changed = true
		}
		for i, idx := range e.Index {
			if idx != nil {
				if newIdx := t.transformExpr(idx); newIdx != idx {
					newSlice.Index[i] = newIdx
					changed = true
				}
			}
		}
		if changed {
			return &newSlice
		}
	}
	return expr
}

func (t *AsCastTransform) convertAsCastToAssert(asCast *syntax.AsCastExpr) *syntax.AssertExpr {
	// Create type assertion: x.(T)
	assertExpr := &syntax.AssertExpr{
		X:    asCast.X,
		Type: asCast.Type,
	}
	assertExpr.SetPos(asCast.Pos())

	return assertExpr
}

func init() {
	RegisterTransformer(&AsCastTransform{})
}
