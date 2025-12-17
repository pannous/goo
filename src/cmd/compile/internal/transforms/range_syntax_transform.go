//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// RangeSyntaxTransform handles the 'start…end' syntax in for loops
// Transforms expressions like "for i in 0…5" to proper Go for loops
type RangeSyntaxTransform struct{}

type rangeSyntaxVisitor struct {
	transform *RangeSyntaxTransform
	ctx       *TransformContext
	file      *syntax.File
	changed   bool
}

func (t *RangeSyntaxTransform) Name() string {
	return "range_syntax_transform"
}

func (t *RangeSyntaxTransform) Priority() int {
	return 50 // Run before in_loop_transform (100)
}

func (t *RangeSyntaxTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &rangeSyntaxVisitor{transform: t, ctx: ctx, file: file}
	syntax.Walk(file, visitor)
	return visitor.changed
}

// Visit implements syntax.Visitor interface
func (v *rangeSyntaxVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}

	// Look for InClause with range operations in for loops
	if forStmt, ok := node.(*syntax.ForStmt); ok {
		if forStmt.Init != nil {
			if inClause, ok := forStmt.Init.(*syntax.InClause); ok {
				// Check if the X expression is a range operation (either .. or ... or …)
				if op, ok := inClause.X.(*syntax.Operation); ok && (op.Op == syntax.Range || op.Op == syntax.RangeExclusive) {
					// Convert entire for loop
					v.transform.convertRangeForLoop(forStmt, inClause, op, v.ctx)
					v.changed = true
				}
			}
		}
	}

	// Handle Range operations in general expressions (not just for loops)
	if op, ok := node.(*syntax.Operation); ok && (op.Op == syntax.Range || op.Op == syntax.RangeExclusive) {
		// Transform a..b or a...b into array literal
		v.transform.convertRangeExpression(op, v.ctx)
		v.changed = true
	}

	return v
}

// convertRangeForLoop converts entire for loop:
// "for i in start..end" to "for i := start; i < end; i++" (exclusive)
// "for i in start…end" to "for i := start; i <= end; i++" (inclusive)
func (t *RangeSyntaxTransform) convertRangeForLoop(forStmt *syntax.ForStmt, inClause *syntax.InClause, rangeOp *syntax.Operation, ctx *TransformContext) {
	pos := inClause.Pos()

	// Extract the loop variable, start, and end
	loopVar := inClause.Lhs
	start := rangeOp.X
	end := rangeOp.Y

	// Create "i := start" initialization
	initStmt := &syntax.AssignStmt{
		Op:  syntax.Def, // := operator
		Lhs: loopVar,
		Rhs: start,
	}
	initStmt.SetPos(pos)

	// Create condition: "i < end" for exclusive (..), "i <= end" for inclusive (... or …)
	var compareOp syntax.Operator
	if rangeOp.Op == syntax.RangeExclusive {
		compareOp = syntax.Lss // < operator (exclusive)
	} else {
		compareOp = syntax.Leq // <= operator (inclusive)
	}

	conditionOp := &syntax.Operation{
		Op: compareOp,
		X:  loopVar, // i
		Y:  end,     // end
	}
	conditionOp.SetPos(pos)
	
	// Create "i++" increment (Rhs == nil means increment)
	incOp := &syntax.AssignStmt{
		Op:  syntax.Add, // + operator
		Lhs: loopVar,    // i
		Rhs: nil,        // nil means increment (i++)
	}
	incOp.SetPos(pos)
	
	// Update the ForStmt
	forStmt.Init = initStmt
	forStmt.Cond = conditionOp
	forStmt.Post = incOp
}

// convertRangeExpression converts a…b into array literal [a, a+1, a+2, ..., b]
func (t *RangeSyntaxTransform) convertRangeExpression(rangeOp *syntax.Operation, ctx *TransformContext) {
	pos := rangeOp.Pos()
	start := rangeOp.X
	end := rangeOp.Y
	
	// For now, create a simple call to a runtime range function
	// We'll use makeRange(start, end) which we'll implement
	funcName := &syntax.Name{Value: "makeRange"}
	funcName.SetPos(pos)
	
	call := &syntax.CallExpr{
		Fun:     funcName,
		ArgList: []syntax.Expr{start, end},
	}
	call.SetPos(pos)
	
	// Replace the operation with the call (convert to Addition as placeholder)
	*rangeOp = syntax.Operation{
		Op: syntax.Add, // Convert to addition to prevent compiler errors
		X:  call,
		Y:  &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"},
	}
	rangeOp.SetPos(pos)
}

func init() {
	RegisterTransformer(&RangeSyntaxTransform{})
}