//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
	"strings"
)

// ArrayIndexTransform handles negative indexing for arrays/slices
// Transforms arr#-1 to arr[len(arr)-1]
type ArrayIndexTransform struct{}

type arrayIndexVisitor struct {
	transform *ArrayIndexTransform
	ctx       *TransformContext
	file      *syntax.File
	changed   bool
}

func (t *ArrayIndexTransform) Name() string {
	return "array_index_transform"
}

func (t *ArrayIndexTransform) Priority() int {
	return 80 // Run before string_index_transform (85)
}

func (t *ArrayIndexTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &arrayIndexVisitor{transform: t, ctx: ctx, file: file}
	syntax.Walk(file, visitor)
	return visitor.changed
}

// Visit implements syntax.Visitor interface
func (v *arrayIndexVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}

	if indexExpr, ok := node.(*syntax.IndexExpr); ok {
		if v.transform.isArrayIndex(indexExpr, v.ctx) {
			if v.transform.hasNegativeIndex(indexExpr.Index) {
				v.transform.convertNegativeArrayIndex(indexExpr, v.ctx)
				v.changed = true
			}
		}
	}

	return v
}

// isArrayIndex checks if this is an array/slice being indexed (not a string)
func (t *ArrayIndexTransform) isArrayIndex(indexExpr *syntax.IndexExpr, ctx *TransformContext) bool {
	// Exclude string literals
	if lit, ok := indexExpr.X.(*syntax.BasicLit); ok {
		return lit.Kind != syntax.StringLit
	}
	
	// Check variable types - exclude strings
	if name, ok := indexExpr.X.(*syntax.Name); ok {
		if ctx != nil && ctx.Types != nil {
			if varType, exists := ctx.Types[name.Value]; exists {
				// Exclude strings, include arrays/slices
				return varType != "string" && !strings.HasPrefix(varType, "map[")
			}
		}
	}
	
	// Include composite literals (arrays)
	if comp, ok := indexExpr.X.(*syntax.CompositeLit); ok {
		// Check if it's an array/slice type (not map)
		if comp.Type == nil {
			return true // Inferred array/slice
		}
		if _, ok := comp.Type.(*syntax.SliceType); ok {
			return true
		}
		if _, ok := comp.Type.(*syntax.ArrayType); ok {
			return true
		}
	}
	
	return false
}

// hasNegativeIndex checks if the index expression contains negative indexing
func (t *ArrayIndexTransform) hasNegativeIndex(index syntax.Expr) bool {
	// Pattern 1: Hash-generated: (-N) - 1
	if op, ok := index.(*syntax.Operation); ok && op.Op == syntax.Sub {
		if lit, ok := op.Y.(*syntax.BasicLit); ok && lit.Kind == syntax.IntLit && lit.Value == "1" {
			if unary, ok := op.X.(*syntax.Operation); ok {
				return unary.Op == syntax.Sub && unary.Y == nil
			}
		}
	}
	
	// Pattern 2: Direct negative: -N
	if unary, ok := index.(*syntax.Operation); ok && unary.Op == syntax.Sub && unary.Y == nil {
		return true
	}
	
	return false
}

// convertNegativeArrayIndex converts arr[(-N)-1] to arr[len(arr)-N] 
func (t *ArrayIndexTransform) convertNegativeArrayIndex(indexExpr *syntax.IndexExpr, ctx *TransformContext) {
	pos := indexExpr.Pos()
	index := indexExpr.Index
	
	var negValue syntax.Expr
	
	// Extract the negative value N from (-N)-1 or -N
	if op, ok := index.(*syntax.Operation); ok && op.Op == syntax.Sub {
		if lit, ok := op.Y.(*syntax.BasicLit); ok && lit.Kind == syntax.IntLit && lit.Value == "1" {
			// Pattern: (-N) - 1
			if unary, ok := op.X.(*syntax.Operation); ok && unary.Op == syntax.Sub && unary.Y == nil {
				negValue = unary.X
			}
		} else if op.Y == nil {
			// Pattern: -N  
			negValue = op.X
		}
	}
	
	if negValue != nil {
		// Create len(arr) - N
		lenCall := &syntax.CallExpr{
			Fun: &syntax.Name{Value: "len"},
			ArgList: []syntax.Expr{indexExpr.X},
		}
		lenCall.SetPos(pos)
		
		newIndex := &syntax.Operation{
			Op: syntax.Sub,
			X:  lenCall,
			Y:  negValue,
		}
		newIndex.SetPos(pos)
		
		indexExpr.Index = newIndex
	}
}

func init() {
	RegisterTransformer(&ArrayIndexTransform{})
}