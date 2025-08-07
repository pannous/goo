//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
	"strconv"
)

// RangeSyntaxTransform handles the 'start…end' syntax in for loops
// Transforms expressions like "for i in 0…5" to proper Go for loops
// This simplified version handles Range operations directly.
type RangeSyntaxTransform struct{}

func (t *RangeSyntaxTransform) Name() string {
	return "range_syntax_transform"
}

func (t *RangeSyntaxTransform) Priority() int {
	return 50 // Run before in_loop_transform (100)
}

// NodeTransformer interface implementation
func (t *RangeSyntaxTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
	// Only handle Operation nodes with Range operator
	if op, ok := node.(*syntax.Operation); ok {
		return op.Op == syntax.Range
	}
	return false
}

func (t *RangeSyntaxTransform) TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node {
	if op, ok := node.(*syntax.Operation); ok && op.Op == syntax.Range {
		return t.convertRangeToSlice(op)
	}
	return nil
}

func (t *RangeSyntaxTransform) PostProcess(file *syntax.File, ctx *TransformContext) bool {
	// No post-processing needed for range syntax transform
	return false
}

// Legacy Transform method for backward compatibility - not used in new architecture
func (t *RangeSyntaxTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	// This method is kept for interface compatibility but not used
	// The new NodeTransformer interface methods are used instead
	// NOTE: The full range syntax transformation requires more complex
	// for loop restructuring which may benefit from the old interface
	return false
}

// convertRangeToSlice converts a range operation to a slice of integers
// This is a simplified approach: 0…5 becomes []int{0, 1, 2, 3, 4}
func (t *RangeSyntaxTransform) convertRangeToSlice(op *syntax.Operation) syntax.Expr {
	pos := op.Pos()
	
	// Extract start and end values
	start := t.extractIntValue(op.X)
	end := t.extractIntValue(op.Y)
	
	if start < 0 || end < 0 || end <= start {
		// If we can't determine valid range, return original
		return op
	}
	
	// Create slice literal with range values
	var elements []syntax.Expr
	for i := start; i < end; i++ {
		elem := &syntax.BasicLit{
			Kind:  syntax.IntLit,
			Value: strconv.Itoa(i),
		}
		elem.SetPos(pos)
		elements = append(elements, elem)
	}
	
	// Create int slice type
	intType := &syntax.Name{Value: "int"}
	intType.SetPos(pos)
	
	sliceType := &syntax.SliceType{Elem: intType}
	sliceType.SetPos(pos)
	
	// Create composite literal
	compLit := &syntax.CompositeLit{
		Type:     sliceType,
		ElemList: elements,
	}
	compLit.SetPos(pos)
	
	return compLit
}

// extractIntValue attempts to extract integer value from an expression
func (t *RangeSyntaxTransform) extractIntValue(expr syntax.Expr) int {
	if lit, ok := expr.(*syntax.BasicLit); ok && lit.Kind == syntax.IntLit {
		if val, err := strconv.Atoi(lit.Value); err == nil {
			return val
		}
	}
	return -1 // Invalid value
}

func init() {
	RegisterTransformer(&RangeSyntaxTransform{})
}