//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
	"strconv"
)

// EllipsisRangeTransform handles ellipsis range expressions like 1…3 -> [1, 2, 3]
type EllipsisRangeTransform struct{}

type ellipsisRangeVisitor struct {
	transform *EllipsisRangeTransform
	ctx       *TransformContext
	file      *syntax.File
	changed   bool
}

func (t *EllipsisRangeTransform) Name() string {
	return "ellipsis_range_transform"
}

func (t *EllipsisRangeTransform) Priority() int {
	return 45 // Run before range_syntax_transform (50)
}

func (t *EllipsisRangeTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &ellipsisRangeVisitor{transform: t, ctx: ctx, file: file}
	syntax.Walk(file, visitor)
	return visitor.changed
}

// Visit implements syntax.Visitor interface
func (v *ellipsisRangeVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}

	// Handle Range operations by replacing them in their parent nodes
	switch n := node.(type) {
	case *syntax.ExprStmt:
		if rangeOp, ok := n.X.(*syntax.Operation); ok && rangeOp.Op == syntax.Range {
			if newExpr := v.transform.convertRangeToArray(rangeOp, v.ctx); newExpr != rangeOp {
				n.X = newExpr
				v.changed = true
			}
		}
	case *syntax.AssignStmt:
		if rangeOp, ok := n.Rhs.(*syntax.Operation); ok && rangeOp.Op == syntax.Range {
			if newExpr := v.transform.convertRangeToArray(rangeOp, v.ctx); newExpr != rangeOp {
				n.Rhs = newExpr
				v.changed = true
			}
		}
	case *syntax.Operation:
		// Handle Range in operation's operands
		if rangeOp, ok := n.X.(*syntax.Operation); ok && rangeOp.Op == syntax.Range {
			if newExpr := v.transform.convertRangeToArray(rangeOp, v.ctx); newExpr != rangeOp {
				n.X = newExpr
				v.changed = true
			}
		}
		if rangeOp, ok := n.Y.(*syntax.Operation); ok && rangeOp.Op == syntax.Range {
			if newExpr := v.transform.convertRangeToArray(rangeOp, v.ctx); newExpr != rangeOp {
				n.Y = newExpr
				v.changed = true
			}
		}
	case *syntax.ParenExpr:
		if rangeOp, ok := n.X.(*syntax.Operation); ok && rangeOp.Op == syntax.Range {
			if newExpr := v.transform.convertRangeToArray(rangeOp, v.ctx); newExpr != rangeOp {
				n.X = newExpr
				v.changed = true
			}
		}
	}

	return v
}

// convertRangeToArray converts a…b into array literal [a, a+1, a+2, ..., b]
func (t *EllipsisRangeTransform) convertRangeToArray(rangeOp *syntax.Operation, ctx *TransformContext) syntax.Expr {
	pos := rangeOp.Pos()
	start := rangeOp.X
	end := rangeOp.Y
	
	// Check if this is a simple numeric range we can expand at compile time
	if startLit, ok := start.(*syntax.BasicLit); ok {
		if endLit, ok := end.(*syntax.BasicLit); ok {
			if startLit.Kind == syntax.IntLit && endLit.Kind == syntax.IntLit {
				return t.createNumericRange(startLit, endLit, pos)
			}
			if startLit.Kind == syntax.RuneLit && endLit.Kind == syntax.RuneLit {
				return t.createCharRange(startLit, endLit, pos)
			}
		}
	}
	
	// For complex expressions, use a runtime function call
	return t.createRuntimeRange(start, end, pos)
}

// createNumericRange creates [start, start+1, ..., end] for numeric literals
func (t *EllipsisRangeTransform) createNumericRange(startLit, endLit *syntax.BasicLit, pos syntax.Pos) syntax.Expr {
	// Parse start and end values
	startVal, err := strconv.Atoi(startLit.Value)
	if err != nil {
		return t.createRuntimeRange(startLit, endLit, pos)
	}
	
	endVal, err := strconv.Atoi(endLit.Value)
	if err != nil {
		return t.createRuntimeRange(startLit, endLit, pos)
	}
	
	// Create array elements
	var elements []syntax.Expr
	for i := startVal; i <= endVal; i++ {
		lit := &syntax.BasicLit{
			Kind:  syntax.IntLit,
			Value: strconv.Itoa(i),
		}
		lit.SetPos(pos)
		elements = append(elements, lit)
	}
	
	// Create []int type
	intType := &syntax.Name{Value: "int"}
	intType.SetPos(pos)
	
	sliceType := &syntax.SliceType{Elem: intType}
	sliceType.SetPos(pos)
	
	// Create array literal using CompositeLit
	arrayLit := &syntax.CompositeLit{
		Type:     sliceType, // []int
		ElemList: elements,
	}
	arrayLit.SetPos(pos)
	
	return arrayLit
}

// createCharRange creates ['a', 'b', 'c'] for character literals
func (t *EllipsisRangeTransform) createCharRange(startLit, endLit *syntax.BasicLit, pos syntax.Pos) syntax.Expr {
	// Parse rune values (they're in single quotes like 'a')
	startRune := t.parseRuneLiteral(startLit.Value)
	endRune := t.parseRuneLiteral(endLit.Value)
	
	if startRune == 0 || endRune == 0 {
		return t.createRuntimeRange(startLit, endLit, pos)
	}
	
	// Create array elements
	var elements []syntax.Expr
	for r := startRune; r <= endRune; r++ {
		lit := &syntax.BasicLit{
			Kind:  syntax.RuneLit,
			Value: "'" + string(r) + "'",
		}
		lit.SetPos(pos)
		elements = append(elements, lit)
	}
	
	// Create []rune type
	runeType := &syntax.Name{Value: "rune"}
	runeType.SetPos(pos)
	
	sliceType := &syntax.SliceType{Elem: runeType}
	sliceType.SetPos(pos)
	
	// Create array literal using CompositeLit
	arrayLit := &syntax.CompositeLit{
		Type:     sliceType, // []rune
		ElemList: elements,
	}
	arrayLit.SetPos(pos)
	
	return arrayLit
}

// parseRuneLiteral extracts rune value from 'x' format
func (t *EllipsisRangeTransform) parseRuneLiteral(value string) rune {
	if len(value) >= 3 && value[0] == '\'' && value[len(value)-1] == '\'' {
		inner := value[1 : len(value)-1]
		if len(inner) == 1 {
			return rune(inner[0])
		}
	}
	return 0
}

// createRuntimeRange creates makeRange(start, end) call for complex expressions
func (t *EllipsisRangeTransform) createRuntimeRange(start, end syntax.Expr, pos syntax.Pos) syntax.Expr {
	funcName := &syntax.Name{Value: "makeRange"}
	funcName.SetPos(pos)
	
	call := &syntax.CallExpr{
		Fun:     funcName,
		ArgList: []syntax.Expr{start, end},
	}
	call.SetPos(pos)
	
	return call
}

func init() {
	RegisterTransformer(&EllipsisRangeTransform{})
}