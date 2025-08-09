// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// StringIndexTransform handles ONLY string literal character access with # operator
// Transforms "abc"#1 to []rune("abc")[0] to return rune instead of byte
// VERY conservative - only handles string literals to avoid breaking array/slice indexing
type StringIndexTransform struct{}

func (t *StringIndexTransform) Name() string {
	return "string_index_transform"
}

func (t *StringIndexTransform) Priority() int {
	return 85 // Run before most transforms but after basic operators
}

func (t *StringIndexTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &stringIndexVisitor{transform: t, ctx: ctx}
	syntax.Walk(file, visitor)
	return visitor.changed
}

type stringIndexVisitor struct {
	transform *StringIndexTransform
	ctx       *TransformContext
	changed   bool
}

func (v *stringIndexVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}

	// Look for IndexExpr nodes that need transformation
	if indexExpr, ok := node.(*syntax.IndexExpr); ok {
		if v.transform.isStringLiteralIndex(indexExpr) {
			// Transform in place
			newExpr := v.transform.transformStringIndex(indexExpr)
			*indexExpr = *newExpr.(*syntax.IndexExpr)
			v.changed = true
		}
	}

	return v
}

// isStringLiteralIndex checks if this is ONLY a string literal being indexed
func (t *StringIndexTransform) isStringLiteralIndex(indexExpr *syntax.IndexExpr) bool {
	// ONLY handle string literals - extremely conservative
	if lit, ok := indexExpr.X.(*syntax.BasicLit); ok {
		return lit.Kind == syntax.StringLit
	}
	return false
}

// transformStringIndex transforms string[index] to []rune(string)[index]
func (t *StringIndexTransform) transformStringIndex(indexExpr *syntax.IndexExpr) syntax.Expr {
	pos := indexExpr.Pos()
	
	// Create []rune type
	runeType := &syntax.Name{Value: "rune"}
	runeType.SetPos(pos)
	
	sliceType := &syntax.SliceType{Elem: runeType}
	sliceType.SetPos(pos)
	
	// Create []rune(string_literal)
	runeConversion := &syntax.CallExpr{
		Fun:     sliceType,
		ArgList: []syntax.Expr{indexExpr.X},
	}
	runeConversion.SetPos(pos)
	
	// Create new IndexExpr with rune slice
	result := &syntax.IndexExpr{
		X:     runeConversion,
		Index: indexExpr.Index, // Keep same index (parser already handles 1-based to 0-based)
	}
	result.SetPos(pos)
	
	return result
}

func init() {
	RegisterTransformer(&StringIndexTransform{})
}