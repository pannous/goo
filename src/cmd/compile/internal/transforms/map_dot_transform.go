// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transforms

import (
	"cmd/compile/internal/syntax"
	"strings"
)

// MapDotTransform converts map dot notation (m.key) to index notation (m["key"])
// for maps with string keys.
type MapDotTransform struct{}

func (t *MapDotTransform) Name() string {
	return "map_dot_transform"
}

func (t *MapDotTransform) Priority() int {
	return 100 // Default priority - between list methods (50) and lambda (200)
}

func (t *MapDotTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	anyChanged := false
	
	// Keep transforming until no more changes can be made (for nested cases)
	maxIterations := 10 // Prevent infinite loops
	for i := 0; i < maxIterations; i++ {
		visitor := &mapDotVisitor{ctx: ctx}
		
		// Transform function declarations
		for _, decl := range file.DeclList {
			if funcDecl, ok := decl.(*syntax.FuncDecl); ok && funcDecl.Body != nil {
				visitor.walkBlockStmt(funcDecl.Body)
			}
		}
		
		if visitor.changed {
			anyChanged = true
		} else {
			// No changes in this iteration, we're done
			break
		}
	}
	
	return anyChanged
}

// mapDotVisitor implements the visitor pattern for map dot notation transformation
type mapDotVisitor struct {
	ctx     *TransformContext
	changed bool
}

// walkBlockStmt walks through all statements in a block
func (v *mapDotVisitor) walkBlockStmt(block *syntax.BlockStmt) {
	for _, stmt := range block.List {
		v.walkStmt(stmt)
	}
}

// walkStmt walks a statement and transforms any map dot notation expressions
func (v *mapDotVisitor) walkStmt(stmt syntax.Stmt) {
	switch s := stmt.(type) {
	case *syntax.ExprStmt:
		v.walkExpr(&s.X)
	case *syntax.AssignStmt:
		v.walkExpr(&s.Lhs)
		v.walkExpr(&s.Rhs)
	case *syntax.BlockStmt:
		v.walkBlockStmt(s)
	case *syntax.IfStmt:
		if s.Init != nil {
			v.walkStmt(s.Init)
		}
		v.walkExpr(&s.Cond)
		if s.Then != nil {
			v.walkBlockStmt(s.Then)
		}
		if s.Else != nil {
			v.walkStmt(s.Else)
		}
	case *syntax.ForStmt:
		if s.Init != nil {
			v.walkStmt(s.Init)
		}
		if s.Cond != nil {
			v.walkExpr(&s.Cond)
		}
		if s.Post != nil {
			v.walkStmt(s.Post)
		}
		if s.Body != nil {
			v.walkBlockStmt(s.Body)
		}
	case *syntax.ReturnStmt:
		if s.Results != nil {
			v.walkExpr(&s.Results)
		}
	case *syntax.CheckStmt:
		if s.Cond != nil {
			v.walkExpr(&s.Cond)
		}
	}
}

// walkExpr walks an expression and transforms selectors
func (v *mapDotVisitor) walkExpr(exprPtr *syntax.Expr) {
	if exprPtr == nil || *exprPtr == nil {
		return
	}
	
	expr := *exprPtr
	
	switch e := expr.(type) {
	case *syntax.SelectorExpr:
		// First walk the base expression to transform any nested selectors
		v.walkExpr(&e.X)
		
		// Then check if this selector should be transformed
		if newExpr := v.transformSelector(e); newExpr != nil {
			*exprPtr = newExpr
			v.changed = true
		}
	case *syntax.CallExpr:
		v.walkExpr(&e.Fun)
		if e.ArgList != nil {
			for i := range e.ArgList {
				v.walkExpr(&e.ArgList[i])
			}
		}
	case *syntax.IndexExpr:
		v.walkExpr(&e.X)
		v.walkExpr(&e.Index)
	case *syntax.Operation:
		v.walkExpr(&e.X)
		if e.Y != nil {
			v.walkExpr(&e.Y)
		}
	case *syntax.ListExpr:
		for i := range e.ElemList {
			v.walkExpr(&e.ElemList[i])
		}
	}
}

func (v *mapDotVisitor) transformSelector(sel *syntax.SelectorExpr) syntax.Expr {
	// Check if the base expression is a variable we know is a map with string keys
	if name, ok := sel.X.(*syntax.Name); ok {
		varType, exists := v.ctx.Types[name.Value]
		if exists && strings.HasPrefix(varType, "map[string]") {
			// Transform m.key to m["key"]
			keyName := sel.Sel.Value
			stringLit := &syntax.BasicLit{
				Kind:  syntax.StringLit,
				Value: `"` + keyName + `"`,
			}
			return &syntax.IndexExpr{
				X:     sel.X,
				Index: stringLit,
			}
		}
	}

	// Also handle cases where the base is an IndexExpr (for nested map access)
	// e.g., config["database"].host -> config["database"]["host"]
	if indexExpr, ok := sel.X.(*syntax.IndexExpr); ok {
		// If the base IndexExpr is likely a map access, transform this selector too
		if v.looksLikeMapAccess(indexExpr) {
			keyName := sel.Sel.Value
			stringLit := &syntax.BasicLit{
				Kind:  syntax.StringLit,
				Value: `"` + keyName + `"`,
			}
			stringLit.SetPos(sel.Sel.Pos())
			
			result := &syntax.IndexExpr{
				X:     sel.X,
				Index: stringLit,
			}
			result.SetPos(sel.Pos())
			return result
		}
	}

	// Handle cases where the base is a SelectorExpr that results in a map
	// e.g., settings.flags.debug -> settings.flags["debug"] 
	if selectorExpr, ok := sel.X.(*syntax.SelectorExpr); ok {
		// Check if this selector expression refers to a map field
		if v.isMapFieldAccess(selectorExpr) {
			keyName := sel.Sel.Value
			stringLit := &syntax.BasicLit{
				Kind:  syntax.StringLit,
				Value: `"` + keyName + `"`,
			}
			stringLit.SetPos(sel.Sel.Pos())
			
			result := &syntax.IndexExpr{
				X:     sel.X,
				Index: stringLit,
			}
			result.SetPos(sel.Pos())
			return result
		}
	}
	return nil
}

// looksLikeMapAccess checks if an IndexExpr is likely a map access with string key
func (v *mapDotVisitor) looksLikeMapAccess(indexExpr *syntax.IndexExpr) bool {
	// Check if the index is a string literal (strong indicator of map access)
	if basicLit, ok := indexExpr.Index.(*syntax.BasicLit); ok {
		if basicLit.Kind == syntax.StringLit {
			return true
		}
	}
	
	// Could add more heuristics here if needed:
	// - Check if base variable is known to be a map
	// - Check patterns in usage context
	
	return false
}

// isMapFieldAccess checks if a selector expression accesses a map field
func (v *mapDotVisitor) isMapFieldAccess(selectorExpr *syntax.SelectorExpr) bool {
	// For cases like settings.flags where flags is a map field
	// We need to check if the field being accessed is of map type
	
	// This is a heuristic approach since we don't have full type information
	// We assume that common map field names are likely maps
	fieldName := selectorExpr.Sel.Value
	commonMapFields := []string{"flags", "config", "settings", "options", "params", "data", "meta"}
	
	for _, mapField := range commonMapFields {
		if fieldName == mapField {
			return true
		}
	}
	
	// Could be enhanced with more sophisticated type inference
	return false
}

func init() {
	RegisterTransformer(&MapDotTransform{})
}