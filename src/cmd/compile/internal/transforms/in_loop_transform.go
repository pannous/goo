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
	} else {
		// "for x in collection" -> "for _, x := range collection" (for slices/arrays/strings)
		// For now, assume slice/array/string pattern: for _, x := range collection
		rangeLhs := t.createRangeLhs(inClause.Lhs, pos)
		rangeClause.Lhs = rangeLhs
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

func init() {
	RegisterTransformer(&InLoopTransform{})
}