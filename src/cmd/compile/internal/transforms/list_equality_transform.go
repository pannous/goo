// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// ListEqualityTransform converts list/slice equality comparisons to slices.Equal calls
// Transforms: a == b (where a,b are slices) -> slices.Equal(a, b)
// Transforms: a != b (where a,b are slices) -> !slices.Equal(a, b)
type ListEqualityTransform struct{}

func (t *ListEqualityTransform) Name() string {
	return "list_equality_transform"
}

func (t *ListEqualityTransform) Priority() int {
	return 120 // Run before check_transform (150)
}

func (t *ListEqualityTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	changed := false

	// Handle top-level statements
	if len(file.TopLevelStmts) > 0 {
		for i, stmt := range file.TopLevelStmts {
			if newStmt := t.transformStmt(stmt); newStmt != stmt {
				file.TopLevelStmts[i] = newStmt
				changed = true
			}
		}
	}

	// Handle function declarations
	for _, decl := range file.DeclList {
		if funcDecl, ok := decl.(*syntax.FuncDecl); ok && funcDecl.Body != nil {
			if t.transformBlock(funcDecl.Body) {
				changed = true
			}
		}
	}

	// Request slices import via ImportManager if we made any transformations
	if changed {
		RequestSlicesImport()
	}

	return changed
}

func (t *ListEqualityTransform) transformBlock(block *syntax.BlockStmt) bool {
	changed := false
	for i, stmt := range block.List {
		if newStmt := t.transformStmt(stmt); newStmt != stmt {
			block.List[i] = newStmt
			changed = true
		}
	}
	return changed
}

func (t *ListEqualityTransform) transformStmt(stmt syntax.Stmt) syntax.Stmt {
	if stmt == nil {
		return nil
	}

	switch s := stmt.(type) {
	case *syntax.IfStmt:
		newCond := t.transformExpr(s.Cond)
		var newThen *syntax.BlockStmt
		if s.Then != nil {
			t.transformBlock(s.Then)
			newThen = s.Then
		}
		var newElse syntax.Stmt
		if s.Else != nil {
			newElse = t.transformStmt(s.Else)
		}
		if newCond != s.Cond || newElse != s.Else {
			newIf := *s
			newIf.Cond = newCond
			newIf.Then = newThen
			newIf.Else = newElse
			return &newIf
		}

	case *syntax.BlockStmt:
		t.transformBlock(s)

	case *syntax.CheckStmt:
		newCond := t.transformExpr(s.Cond)
		if newCond != s.Cond {
			newCheck := *s
			newCheck.Cond = newCond
			return &newCheck
		}

	case *syntax.ExprStmt:
		newExpr := t.transformExpr(s.X)
		if newExpr != s.X {
			newStmt := *s
			newStmt.X = newExpr
			return &newStmt
		}

	case *syntax.AssignStmt:
		newRhs := t.transformExpr(s.Rhs)
		if newRhs != s.Rhs {
			newAssign := *s
			newAssign.Rhs = newRhs
			return &newAssign
		}

	case *syntax.DeclStmt:
		for _, decl := range s.DeclList {
			if varDecl, ok := decl.(*syntax.VarDecl); ok && varDecl.Values != nil {
				newValues := t.transformExpr(varDecl.Values)
				if newValues != varDecl.Values {
					varDecl.Values = newValues
				}
			}
		}
	}

	return stmt
}

func (t *ListEqualityTransform) transformExpr(expr syntax.Expr) syntax.Expr {
	if expr == nil {
		return nil
	}
	
	switch e := expr.(type) {
	case *syntax.Operation:
		// First recursively transform operands
		if e.X != nil {
			if newX := t.transformExpr(e.X); newX != e.X {
				e.X = newX
			}
		}
		if e.Y != nil {
			if newY := t.transformExpr(e.Y); newY != e.Y {
				e.Y = newY
			}
		}

		// Then check if this is a slice comparison
		if (e.Op == syntax.Eql || e.Op == syntax.Neq) && t.looksLikeSliceComparison(e.X, e.Y) {
			return t.createSlicesEqualCall(e)
		}
		
	case *syntax.ParenExpr:
		if newX := t.transformExpr(e.X); newX != e.X {
			e.X = newX
		}
		
	case *syntax.ListExpr:
		for i, elem := range e.ElemList {
			if newElem := t.transformExpr(elem); newElem != elem {
				e.ElemList[i] = newElem
			}
		}

	case *syntax.CallExpr:
		// Transform arguments in function calls (e.g., printf(..., a == b))
		for i, arg := range e.ArgList {
			if newArg := t.transformExpr(arg); newArg != arg {
				e.ArgList[i] = newArg
			}
		}
	}

	return expr
}

// looksLikeSliceComparison checks if an expression looks like it might be a slice
func (t *ListEqualityTransform) looksLikeSliceComparison(x, y syntax.Expr) bool {
	// Transform if at least one side is definitely a list/slice literal
	if t.isListLiteral(x) || t.isListLiteral(y) {
		return true
	}

	// Also transform if both sides are variables (could be slices)
	// This is aggressive but necessary since we don't have full type info here
	_, xIsName := x.(*syntax.Name)
	_, yIsName := y.(*syntax.Name)
	if xIsName && yIsName {
		return true
	}

	return false
}

// isListLiteral checks if an expression is a list literal
func (t *ListEqualityTransform) isListLiteral(expr syntax.Expr) bool {
	switch e := expr.(type) {
	case *syntax.CompositeLit:
		if e.Type != nil {
			// Try all possible type representations
			switch e.Type.(type) {
			case *syntax.IndexExpr:
				return true
			case *syntax.ArrayType:
				return true
			case *syntax.Operation:
				// Might be []type represented as an operation
				return true
			case *syntax.SliceType:
				return true
			default:
				// Be aggressive - assume composite literals with unknown types are slices
				// This handles []int{...}, [...]int{...}, etc.
				return true
			}
		}
		return false
	case *syntax.ListExpr:
		return true
	default:
		return false
	}
}

// createSlicesEqualCall creates slices.Equal(a, b) or !slices.Equal(a, b)
func (t *ListEqualityTransform) createSlicesEqualCall(op *syntax.Operation) syntax.Expr {
	pos := op.Pos()
	
	// Create slices.Equal(a, b) call
	slicesName := &syntax.Name{Value: "slices"}
	slicesName.SetPos(pos)
	
	equalName := &syntax.Name{Value: "Equal"}
	equalName.SetPos(pos)
	
	slicesEqualSel := &syntax.SelectorExpr{
		X:   slicesName,
		Sel: equalName,
	}
	slicesEqualSel.SetPos(pos)
	
	equalCall := &syntax.CallExpr{
		Fun:     slicesEqualSel,
		ArgList: []syntax.Expr{op.X, op.Y},
	}
	equalCall.SetPos(pos)
	
	if op.Op == syntax.Eql {
		// a == b -> slices.Equal(a, b)
		return equalCall
	} else {
		// a != b -> !slices.Equal(a, b)
		notOp := &syntax.Operation{
			Op: syntax.Not,
			X:  equalCall,
		}
		notOp.SetPos(pos)
		return notOp
	}
}

func init() {
	RegisterTransformer(&ListEqualityTransform{})
}
