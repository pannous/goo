// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
	"strings"
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
	usesSlices := false
	usesMaps := false
	debug("LIST_EQUALITY_TRANSFORM: Transform called")

	// Use SyntaxWalker with VisitExpr - it walks EVERYTHING including function arguments!
	walker := &SyntaxWalker{
		VisitExpr: func(expr syntax.Expr) syntax.Expr {
			// Check if this is a slice/map equality/inequality operation
			if op, ok := expr.(*syntax.Operation); ok {
				if (op.Op == syntax.Eql || op.Op == syntax.Neq) && t.looksLikeSliceComparison(op.X, op.Y, ctx) {
					pkgName := t.determineComparisonType(op.X, op.Y, ctx)
					if pkgName == "maps" {
						usesMaps = true
					} else {
						usesSlices = true
					}
					changed = true
					return t.createSlicesEqualCall(op, ctx)
				}
			}
			return expr
		},
	}

	walker.WalkFile(file)

	// Also handle top-level statements (implicit main)
	if len(file.TopLevelStmts) > 0 {
		for i, stmt := range file.TopLevelStmts {
			oldChanged := changed
			file.TopLevelStmts[i] = t.transformTopLevelStmt(stmt, &changed, ctx)
			// Track if we added maps in top level too
			if changed && !oldChanged {
				// Assume slices for now; proper detection would require passing back info
				usesSlices = true
			}
		}
	}

	// Request imports via ImportManager if we made any transformations
	if usesSlices {
		RequestSlicesImport()
	}
	if usesMaps {
		RequestMapsImport()
	}

	return changed
}

func (t *ListEqualityTransform) transformTopLevelStmt(stmt syntax.Stmt, changed *bool, ctx *TransformContext) syntax.Stmt {
	walker := &SyntaxWalker{
		VisitExpr: func(expr syntax.Expr) syntax.Expr {
			if op, ok := expr.(*syntax.Operation); ok {
				if (op.Op == syntax.Eql || op.Op == syntax.Neq) && t.looksLikeSliceComparison(op.X, op.Y, ctx) {
					*changed = true
					return t.createSlicesEqualCall(op, ctx)
				}
			}
			return expr
		},
	}
	walker.walkStmt(stmt)
	return stmt
}

// looksLikeSliceComparison checks if an expression looks like it might be a slice or map
func (t *ListEqualityTransform) looksLikeSliceComparison(x, y syntax.Expr, ctx *TransformContext) bool {
	// Transform if at least one side is definitely a list/slice literal
	if t.isListLiteral(x) || t.isListLiteral(y) {
		return true
	}

	// Also transform if we have type information indicating slices
	if t.isSliceVariable(x, ctx) || t.isSliceVariable(y, ctx) {
		return true
	}

	// Also transform if we have type information indicating maps
	if t.isMapVariable(x, ctx) || t.isMapVariable(y, ctx) {
		return true
	}

	return false
}

// isSliceVariable checks if an expression is a variable with a slice type
func (t *ListEqualityTransform) isSliceVariable(expr syntax.Expr, ctx *TransformContext) bool {
	if ctx == nil || ctx.Types == nil {
		return false
	}

	name, ok := expr.(*syntax.Name)
	if !ok {
		return false
	}

	varType, exists := ctx.Types[name.Value]
	if !exists {
		return false
	}

	// Exclude arrays (marked with "[_]" prefix) - they use native == comparison
	if strings.HasPrefix(varType, "[_]") {
		return false
	}

	// Check if type is a slice (starts with "[]" or is "slice")
	return strings.HasPrefix(varType, "[]") || varType == "slice"
}

// isMapVariable checks if an expression is a variable with a map type
func (t *ListEqualityTransform) isMapVariable(expr syntax.Expr, ctx *TransformContext) bool {
	if ctx == nil || ctx.Types == nil {
		return false
	}

	name, ok := expr.(*syntax.Name)
	if !ok {
		return false
	}

	varType, exists := ctx.Types[name.Value]
	if !exists {
		return false
	}

	// Check if type is a map (starts with "map[")
	return strings.HasPrefix(varType, "map[")
}

// isListLiteral checks if an expression is a list literal
func (t *ListEqualityTransform) isListLiteral(expr syntax.Expr) bool {
	switch e := expr.(type) {
	case *syntax.CompositeLit:
		if e.Type != nil {
			// Only transform slice types, not arrays
			switch e.Type.(type) {
			case *syntax.SliceType:
				// []int{...} - this is a slice
				return true
			case *syntax.ArrayType:
				// [2]int{...} or [...]int{...} - these are arrays, use native ==
				return false
			case *syntax.IndexExpr:
				// Generic slice type like []T[K] - treat as slice
				return true
			default:
				// Unknown type - be conservative, don't transform
				return false
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

// determineComparisonType checks if we're comparing slices or maps
func (t *ListEqualityTransform) determineComparisonType(x, y syntax.Expr, ctx *TransformContext) string {
	// Check if either side is a map
	if t.isMapVariable(x, ctx) || t.isMapVariable(y, ctx) {
		return "maps"
	}
	// Default to slices
	return "slices"
}

// createSlicesEqualCall creates slices.Equal[T](a, b) or maps.Equal(a, b) or negated versions
func (t *ListEqualityTransform) createSlicesEqualCall(op *syntax.Operation, ctx *TransformContext) syntax.Expr {
	pos := op.Pos()

	// Determine if we're comparing slices or maps
	pkgName := t.determineComparisonType(op.X, op.Y, ctx)

	// Create pkg.Equal(a, b) call
	slicesName := &syntax.Name{Value: pkgName}
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
