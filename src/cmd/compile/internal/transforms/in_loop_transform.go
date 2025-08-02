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
	
	// Walk the file manually to ensure proper order
	visitor.walkFile(file)
	
	return visitor.changed
}

// walkFile manually walks the file to handle InClause transformation
func (v *inLoopVisitor) walkFile(file *syntax.File) {
	for _, decl := range file.DeclList {
		v.walkDecl(decl)
	}
}

// walkDecl walks declarations
func (v *inLoopVisitor) walkDecl(decl syntax.Decl) {
	switch d := decl.(type) {
	case *syntax.FuncDecl:
		if d.Body != nil {
			v.walkBlockStmt(d.Body)
		}
	// Note: BlockStmt is not a Decl, so we don't need this case
	}
}

// walkBlockStmt walks statements in a block
func (v *inLoopVisitor) walkBlockStmt(block *syntax.BlockStmt) {
	for _, stmt := range block.List {
		v.walkStmt(stmt)
	}
}

// walkStmt walks individual statements
func (v *inLoopVisitor) walkStmt(stmt syntax.Stmt) {
	switch s := stmt.(type) {
	case *syntax.ForStmt:
		// Handle InClause transformation
		if s.Init != nil {
			if inClause, ok := s.Init.(*syntax.InClause); ok {
				rangeClause := v.transform.convertInClauseToRange(inClause, v.ctx)
				if rangeClause != nil {
					s.Init = rangeClause
					v.changed = true
				}
			}
		}
		// Walk the body
		if s.Body != nil {
			v.walkBlockStmt(s.Body)
		}
	case *syntax.BlockStmt:
		v.walkBlockStmt(s)
	case *syntax.IfStmt:
		if s.Then != nil {
			v.walkBlockStmt(s.Then)
		}
		if s.Else != nil {
			v.walkStmt(s.Else)
		}
	}
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
	
	// Also check for InClause nodes directly (in case they appear elsewhere)
	if inClause, ok := node.(*syntax.InClause); ok {
		// This shouldn't happen if ForStmt handling works, but adding as fallback
		rangeClause := v.transform.convertInClauseToRange(inClause, v.ctx)
		if rangeClause != nil {
			// Note: we can't replace the node directly here since Visit doesn't allow that
			// The ForStmt case above should handle this
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
	rangeClause.Def = true     // Always use := for new variables
	
	// Handle different LHS patterns based on collection type
	if inClause.Lhs == nil {
		// "for in collection" -> "for range collection"
		rangeClause.Lhs = nil
	} else {
		// "for x in collection" -> "for _, x := range collection" (for slices/arrays/strings)
		// For now, assume slice/array/string pattern: for _, x := range collection
		rangeClause.Lhs = t.createRangeLhs(inClause.Lhs, pos)
	}
	
	return rangeClause
}

// createRangeLhs creates the appropriate left-hand side for the range clause
// For slices/arrays/strings: "x" becomes "_, x"
// For maps: "x" stays "x" (keys only)
func (t *InLoopTransform) createRangeLhs(originalLhs syntax.Expr, pos syntax.Pos) syntax.Expr {
	// For now, implement the slice/array/string pattern: _, x
	// This assumes we want the values, not the indices
	
	// Create "_, originalLhs"
	underscore := &syntax.Name{Value: "_"}
	underscore.SetPos(pos)
	
	// Ensure originalLhs has proper position
	if originalLhs.Pos() == (syntax.Pos{}) {
		originalLhs.SetPos(pos)
	}
	
	// Create a list expression with _, originalLhs
	listExpr := &syntax.ListExpr{}
	listExpr.SetPos(pos)
	listExpr.ElemList = []syntax.Expr{underscore, originalLhs}
	
	return listExpr
}

func init() {
	RegisterTransformer(&InLoopTransform{})
}