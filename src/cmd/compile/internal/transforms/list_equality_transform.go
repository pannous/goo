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
	debug("LIST_EQUALITY_TRANSFORM: Transform called")

	// Use SyntaxWalker with VisitExpr - it walks EVERYTHING including function arguments!
	walker := &SyntaxWalker{
		VisitExpr: func(expr syntax.Expr) syntax.Expr {
			// Check if this is a slice equality/inequality operation
			if op, ok := expr.(*syntax.Operation); ok {
				debug("LIST_EQUALITY checking operation: op=%v", op.Op)
				if (op.Op == syntax.Eql || op.Op == syntax.Neq) && t.looksLikeSliceComparison(op.X, op.Y) {
					debug("LIST_EQUALITY transforming slice comparison!")
					changed = true
					return t.createSlicesEqualCall(op)
				}
			}
			return expr
		},
	}

	walker.WalkFile(file)

	// Also handle top-level statements (implicit main)
	if len(file.TopLevelStmts) > 0 {
		for i, stmt := range file.TopLevelStmts {
			file.TopLevelStmts[i] = t.transformTopLevelStmt(stmt, &changed)
		}
	}

	// Request slices import via ImportManager if we made any transformations
	if changed {
		RequestSlicesImport()
	}

	return changed
}

func (t *ListEqualityTransform) transformTopLevelStmt(stmt syntax.Stmt, changed *bool) syntax.Stmt {
	walker := &SyntaxWalker{
		VisitExpr: func(expr syntax.Expr) syntax.Expr {
			if op, ok := expr.(*syntax.Operation); ok {
				if (op.Op == syntax.Eql || op.Op == syntax.Neq) && t.looksLikeSliceComparison(op.X, op.Y) {
					*changed = true
					return t.createSlicesEqualCall(op)
				}
			}
			return expr
		},
	}
	walker.walkStmt(stmt)
	return stmt
}

// looksLikeSliceComparison checks if an expression looks like it might be a slice
func (t *ListEqualityTransform) looksLikeSliceComparison(x, y syntax.Expr) bool {
	// Transform ONLY if at least one side is definitely a list/slice literal
	// Don't transform variable comparisons - we can't know their types at this stage
	if t.isListLiteral(x) || t.isListLiteral(y) {
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

// extractSliceType tries to extract the slice type from a slice expression
func (t *ListEqualityTransform) extractSliceType(expr syntax.Expr) syntax.Expr {
	if comp, ok := expr.(*syntax.CompositeLit); ok && comp.Type != nil {
		// For []Type{...}, comp.Type is the slice type
		// Return the whole slice type (e.g., []int), not just the element type
		return comp.Type
	}
	return nil
}

// createSlicesEqualCall creates slices.Equal[T](a, b) or !slices.Equal[T](a, b)
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

	// Try to extract slice type for generic instantiation
	var funExpr syntax.Expr = slicesEqualSel
	if sliceType := t.extractSliceType(op.X); sliceType != nil {
		debug("LIST_EQUALITY: Extracted slice type from left side\n")
		// Create slices.Equal[[]T] with type parameter
		typeIndexExpr := &syntax.IndexExpr{
			X:     slicesEqualSel,
			Index: sliceType,
		}
		typeIndexExpr.SetPos(pos)
		funExpr = typeIndexExpr
	} else if sliceType := t.extractSliceType(op.Y); sliceType != nil {
		debug("LIST_EQUALITY: Extracted slice type from right side\n")
		// Try the right side if left didn't work
		typeIndexExpr := &syntax.IndexExpr{
			X:     slicesEqualSel,
			Index: sliceType,
		}
		typeIndexExpr.SetPos(pos)
		funExpr = typeIndexExpr
	} else {
		debug("LIST_EQUALITY: Could not extract slice type, using untyped slices.Equal\n")
	}

	equalCall := &syntax.CallExpr{
		Fun:     funExpr,
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
