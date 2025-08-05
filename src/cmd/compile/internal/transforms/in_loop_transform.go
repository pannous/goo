// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// InLoopTransform handles the 'for x in collection' syntax
// Transforms expressions like "for x in slice" to "for _, x := range slice"
// and "for x in map" to "for x := range map"
type InLoopTransform struct{}

type inLoopVisitor struct {
	transform *InLoopTransform
	ctx       *TransformContext
	file      *syntax.File
	changed   bool
}

func (t *InLoopTransform) Name() string {
	return "in_loop_transform"
}

func (t *InLoopTransform) Priority() int {
	return 100 // Default priority - between list methods (50) and lambda (200)
}

func (t *InLoopTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &inLoopVisitor{transform: t, ctx: ctx, file: file}
	
	// Use syntax.Walk but with a better visitor implementation
	syntax.Walk(file, visitor)
	
	return visitor.changed
}

// Visit implements syntax.Visitor interface
func (v *inLoopVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}
	
	// Look for ForStmt nodes that contain InClause
	if forStmt, ok := node.(*syntax.ForStmt); ok {
		if forStmt.Init != nil {
			if inClause, ok := forStmt.Init.(*syntax.InClause); ok {
				// Transform the InClause to a RangeClause
				rangeClause := v.transform.convertInClauseToRange(inClause, v.ctx)
				if rangeClause != nil {
					forStmt.Init = rangeClause
					v.changed = true
				}
			}
		}
	}
	
	// Continue visiting child nodes  
	return v
}

// convertInClauseToRange converts "for x in collection" to "for _, x := range collection"
// or appropriate range syntax based on the collection type
func (t *InLoopTransform) convertInClauseToRange(inClause *syntax.InClause, ctx *TransformContext) syntax.SimpleStmt {
	pos := inClause.Pos()
	
	
	// Create a RangeClause to replace the InClause
	rangeClause := &syntax.RangeClause{}
	rangeClause.SetPos(pos)
	rangeClause.X = inClause.X // Same collection expression
	rangeClause.Def = inClause.Def // Use the same definition flag from InClause
	
	// Handle different LHS patterns based on collection type
	if inClause.Lhs == nil {
		// "for in collection" -> "for range collection"
		rangeClause.Lhs = nil
	} else if list, ok := inClause.Lhs.(*syntax.ListExpr); ok && len(list.ElemList) == 2 {
		// "for key, value in collection" -> "for key, value := range collection"
		rangeClause.Lhs = inClause.Lhs // Use key,value as-is
		rangeClause.Def = true
	} else {
		// "for x in collection" -> "for _, x := range collection" (for slices/arrays/strings)
		// or "for x in collection" -> "for x := range collection" (for maps/iterators)
		if t.isIteratorType(inClause.X) {
			// Iterator functions only allow single variable: for x := range iterator()
			rangeClause.Lhs = inClause.Lhs
		} else {
			// Non-iterators use blank identifier for value access
			rangeLhs := t.createRangeLhs(inClause.Lhs, pos)
			rangeClause.Lhs = rangeLhs
		}
		// Ensure we use := for new variable declarations
		rangeClause.Def = true
	}
	
	return rangeClause
}

// createRangeLhs creates the appropriate left-hand side for the range clause
// For slices/arrays/strings: "x" becomes "_, x" (value access)
// For maps: "x" stays "x" (keys)
func (t *InLoopTransform) createRangeLhs(originalLhs syntax.Expr, pos syntax.Pos) syntax.Expr {
	// For Python-like behavior: "for x in collection" -> "for _, x := range collection"
	// This gives value access instead of index access
	
	// Create blank identifier for index
	blankIdent := &syntax.Name{Value: "_"}
	blankIdent.SetPos(pos)
	
	// Ensure originalLhs has proper position
	if originalLhs.Pos() == (syntax.Pos{}) {
		originalLhs.SetPos(pos)
	}
	
	// Create list expression: [_, x]
	listExpr := &syntax.ListExpr{
		ElemList: []syntax.Expr{blankIdent, originalLhs},
	}
	listExpr.SetPos(pos)
	
	return listExpr
}

// isIteratorType attempts to detect if the expression is likely an iterator
// This is a heuristic since we don't have full type information at AST level
func (t *InLoopTransform) isIteratorType(expr syntax.Expr) bool {
	// Check if it's a function call that might return an iterator
	if call, ok := expr.(*syntax.CallExpr); ok {
		// Check if the function name suggests it returns an iterator
		if name, ok := call.Fun.(*syntax.Name); ok {
			// Common iterator function naming patterns
			funcName := name.Value
			// Look for iterator-like function names or functions that end with common patterns
			return t.looksLikeIteratorFunction(funcName)
		}
		
		// Check for selector expressions like somePackage.Iterator()
		if sel, ok := call.Fun.(*syntax.SelectorExpr); ok {
			return t.looksLikeIteratorFunction(sel.Sel.Value)
		}
	}
	
	return false
}

// looksLikeIteratorFunction checks if a function name suggests it returns an iterator
func (t *InLoopTransform) looksLikeIteratorFunction(name string) bool {
	// Common patterns for iterator function names
	iteratorPatterns := []string{
		"Iter", "Iterator", "Items", "Values", "Keys", "Entries", 
		"Numbers", "Range", "Sequence", "Stream", "Generate",
	}
	
	for _, pattern := range iteratorPatterns {
		if name == pattern || 
		   len(name) > len(pattern) && name[len(name)-len(pattern):] == pattern ||
		   len(name) > len(pattern) && name[:len(pattern)] == pattern {
			return true
		}
	}
	
	return false
}

func init() {
	RegisterTransformer(&InLoopTransform{})
}