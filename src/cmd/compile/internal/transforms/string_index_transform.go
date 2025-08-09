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
		if v.transform.isStringLiteralIndex(indexExpr, v.ctx) {
			// Transform - might return CallExpr or IndexExpr
			newExpr := v.transform.transformStringIndex(indexExpr)
			if newIndex, ok := newExpr.(*syntax.IndexExpr); ok {
				// Rune slice indexing - replace in place
				*indexExpr = *newIndex
			} else {
				// Character search call - need to replace the parent node
				// For now, convert back to index expr format (this is a limitation)
			}
			v.changed = true
		}
	}

	return v
}

// isStringLiteralIndex checks if this is a string being indexed
func (t *StringIndexTransform) isStringLiteralIndex(indexExpr *syntax.IndexExpr, ctx *TransformContext) bool {
	// Handle string literals
	if lit, ok := indexExpr.X.(*syntax.BasicLit); ok {
		return lit.Kind == syntax.StringLit
	}
	
	// Handle string variables by checking type context
	if name, ok := indexExpr.X.(*syntax.Name); ok {
		if ctx != nil && ctx.Types != nil {
			if varType, exists := ctx.Types[name.Value]; exists {
				return varType == "string"
			}
		}
	}
	
	return false
}

// isRuneIndex checks if the index is a rune literal for character-based indexing
func (t *StringIndexTransform) isRuneIndex(index syntax.Expr) bool {
	// Direct rune literal
	if lit, ok := index.(*syntax.BasicLit); ok {
		return lit.Kind == syntax.RuneLit
	}
	
	// Parser-generated pattern: (rune_literal) - 1 from hash syntax
	if op, ok := index.(*syntax.Operation); ok && op.Op == syntax.Sub {
		if lit, ok := op.Y.(*syntax.BasicLit); ok && lit.Kind == syntax.IntLit && lit.Value == "1" {
			if runeLit, ok := op.X.(*syntax.BasicLit); ok && runeLit.Kind == syntax.RuneLit {
				return true
			}
		}
	}
	
	return false
}

// createCharacterIndexCall creates strings.IndexByte(receiver, char) for character-based indexing
func (t *StringIndexTransform) createCharacterIndexCall(receiver, index syntax.Expr, pos syntax.Pos) syntax.Expr {
	// Extract the actual rune literal from parser pattern or use directly
	var runeLit syntax.Expr
	if op, ok := index.(*syntax.Operation); ok && op.Op == syntax.Sub {
		// Parser pattern: (rune_literal) - 1
		if lit, ok := op.Y.(*syntax.BasicLit); ok && lit.Kind == syntax.IntLit && lit.Value == "1" {
			if rune, ok := op.X.(*syntax.BasicLit); ok && rune.Kind == syntax.RuneLit {
				runeLit = rune
			}
		}
	} else {
		// Direct rune literal
		runeLit = index
	}
	
	// Create strings.IndexByte call
	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)
	
	indexByteName := &syntax.Name{Value: "IndexByte"}
	indexByteName.SetPos(pos)
	
	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: indexByteName,
	}
	selector.SetPos(pos)
	
	// Create function call
	result := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, runeLit},
	}
	result.SetPos(pos)
	
	return result
}

// transformStringIndex transforms string[index] to []rune(string)[adjustedIndex] or strings.IndexByte
func (t *StringIndexTransform) transformStringIndex(indexExpr *syntax.IndexExpr) syntax.Expr {
	pos := indexExpr.Pos()
	
	
	// TODO: Character-based indexing "abc"#'b' not yet implemented
	// Would need more sophisticated node replacement to return CallExpr from IndexExpr context
	
	// Regular numeric indexing: convert to rune slice access
	// Create []rune type
	runeType := &syntax.Name{Value: "rune"}
	runeType.SetPos(pos)
	
	sliceType := &syntax.SliceType{Elem: runeType}
	sliceType.SetPos(pos)
	
	// Create []rune(string_expr)
	runeConversion := &syntax.CallExpr{
		Fun:     sliceType,
		ArgList: []syntax.Expr{indexExpr.X},
	}
	runeConversion.SetPos(pos)
	
	// Handle different indexing patterns
	adjustedIndex := t.handleIndexConversion(indexExpr.Index, runeConversion, pos)
	
	// Create new IndexExpr with rune slice
	result := &syntax.IndexExpr{
		X:     runeConversion,
		Index: adjustedIndex,
	}
	result.SetPos(pos)
	
	return result
}

// handleIndexConversion handles both hash and bracket indexing patterns
func (t *StringIndexTransform) handleIndexConversion(index syntax.Expr, runeSlice syntax.Expr, pos syntax.Pos) syntax.Expr {
	// Pattern 1: Hash-generated negative indexing: (-N) - 1 → len(runes) - N
	if op, ok := index.(*syntax.Operation); ok && op.Op == syntax.Sub {
		// Check if Y is literal 1 (from parser's 1-based conversion)
		if lit, ok := op.Y.(*syntax.BasicLit); ok && lit.Kind == syntax.IntLit && lit.Value == "1" {
			// Check if X is a unary minus operation  
			if unary, ok := op.X.(*syntax.Operation); ok {
				if unary.Op == syntax.Sub && unary.Y == nil {
					// This is (-N) - 1, convert to len(runes) - N
					if negLit, ok := unary.X.(*syntax.BasicLit); ok && negLit.Kind == syntax.IntLit {
						return t.createNegativeIndex(runeSlice, negLit, pos)
					}
				}
			}
		}
	}
	
	// Pattern 2: Regular bracket negative indexing: -N → len(runes) - N  
	if unary, ok := index.(*syntax.Operation); ok && unary.Op == syntax.Sub && unary.Y == nil {
		if negLit, ok := unary.X.(*syntax.BasicLit); ok && negLit.Kind == syntax.IntLit {
			return t.createNegativeIndex(runeSlice, negLit, pos)
		}
	}
	
	// For non-negative indices, return as-is
	return index
}

// createNegativeIndex creates len(runes) - N for negative indexing
func (t *StringIndexTransform) createNegativeIndex(runeSlice syntax.Expr, negLit *syntax.BasicLit, pos syntax.Pos) syntax.Expr {
	// Create len(runeSlice)
	lenCall := &syntax.CallExpr{
		Fun:     &syntax.Name{Value: "len"},
		ArgList: []syntax.Expr{runeSlice},
	}
	lenCall.SetPos(pos)
	lenCall.Fun.SetPos(pos)
	
	// Create len(runes) - N (where N is the positive value)
	result := &syntax.Operation{
		Op: syntax.Sub,
		X:  lenCall,
		Y:  negLit,
	}
	result.SetPos(pos)
	
	return result
}

func init() {
	RegisterTransformer(&StringIndexTransform{})
}