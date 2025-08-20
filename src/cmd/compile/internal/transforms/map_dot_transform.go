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
	visitor := &mapDotVisitor{ctx: ctx}
	
	// Use syntax.Walk to traverse the entire AST
	syntax.Walk(file, visitor)
	
	return visitor.changed
}

// mapDotVisitor implements the visitor pattern for map dot notation transformation
type mapDotVisitor struct {
	ctx     *TransformContext
	changed bool
}

// Visit implements syntax.Visitor interface
func (v *mapDotVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}
	
	debug("map_dot: visiting %T\n", node)
	
	// Transform nodes that contain expressions that might have map dot access
	switch n := node.(type) {
	case *syntax.AssignStmt:
		// Infer map type from assignments like: x := units.Available()
		var lhsList []syntax.Expr
		var rhsList []syntax.Expr
		if l, ok := n.Lhs.(*syntax.ListExpr); ok {
			lhsList = l.ElemList
		} else {
			lhsList = []syntax.Expr{n.Lhs}
		}
		if r, ok := n.Rhs.(*syntax.ListExpr); ok {
			rhsList = r.ElemList
		} else {
			rhsList = []syntax.Expr{n.Rhs}
		}
		if len(lhsList) == len(rhsList) {
			for i := range lhsList {
				if lhsName, ok := lhsList[i].(*syntax.Name); ok {
					if call, ok := rhsList[i].(*syntax.CallExpr); ok {
						if v.isMapReturningFunction(call) {
							v.ctx.Types[lhsName.Value] = "map[string]any"
						}
					}
				}
			}
		}
		// Also transform both sides where applicable
		if transformed := v.transformExpr(n.Lhs); transformed != n.Lhs {
			n.Lhs = transformed
			v.changed = true
		}
		if transformed := v.transformExpr(n.Rhs); transformed != n.Rhs {
			n.Rhs = transformed
			v.changed = true
		}
	case *syntax.VarDecl:
		if n.Values != nil {
			if transformed := v.transformExpr(n.Values); transformed != n.Values {
				n.Values = transformed
				v.changed = true
			}
		}
	case *syntax.ExprStmt:
		if transformed := v.transformExpr(n.X); transformed != n.X {
			n.X = transformed
			v.changed = true
		}
	case *syntax.ReturnStmt:
		if n.Results != nil {
			if transformed := v.transformExpr(n.Results); transformed != n.Results {
				n.Results = transformed
				v.changed = true
			}
		}
	case *syntax.IfStmt:
		if n.Cond != nil {
			if transformed := v.transformExpr(n.Cond); transformed != n.Cond {
				n.Cond = transformed
				v.changed = true
			}
		}
	case *syntax.ForStmt:
		if n.Cond != nil {
			if transformed := v.transformExpr(n.Cond); transformed != n.Cond {
				n.Cond = transformed
				v.changed = true
			}
		}
	case *syntax.CallExpr:
		// Transform arguments
		for i, arg := range n.ArgList {
			if transformed := v.transformExpr(arg); transformed != arg {
				n.ArgList[i] = transformed
				v.changed = true
			}
		}
	case *syntax.CheckStmt:
		if n.Cond != nil {
			if transformed := v.transformExpr(n.Cond); transformed != n.Cond {
				n.Cond = transformed
				v.changed = true
			}
		}
	}
	
	return v
}

// transformExpr recursively transforms expressions
func (v *mapDotVisitor) transformExpr(expr syntax.Expr) syntax.Expr {
	if expr == nil {
		return expr
	}
	
    // Special-case selector: first recurse into base, then attempt transform
    if sel, ok := expr.(*syntax.SelectorExpr); ok {
        // Recurse into base first to enable chained transformations
        sel.X = v.transformExpr(sel.X)
        if newExpr := v.transformSelector(sel); newExpr != nil {
            return newExpr
        }
        return sel
    }
    
    // Recursively transform other sub-expressions
    switch e := expr.(type) {
    case *syntax.CallExpr:
        // Transform function and arguments
        e.Fun = v.transformExpr(e.Fun)
        for i, arg := range e.ArgList {
            e.ArgList[i] = v.transformExpr(arg)
        }
    case *syntax.ParenExpr:
        e.X = v.transformExpr(e.X)
    case *syntax.Operation:
        e.X = v.transformExpr(e.X)
        if e.Y != nil {
            e.Y = v.transformExpr(e.Y)
        }
    case *syntax.IndexExpr:
        e.X = v.transformExpr(e.X)
        e.Index = v.transformExpr(e.Index)
    case *syntax.ListExpr:
        for i := range e.ElemList {
            e.ElemList[i] = v.transformExpr(e.ElemList[i])
        }
    }
	
	return expr
}

// Helper to convert expressions to strings for debugging
func (v *mapDotVisitor) exprToString(expr syntax.Expr) string {
	switch e := expr.(type) {
	case *syntax.Name:
		return e.Value
	case *syntax.CallExpr:
		return "call"
	case *syntax.SelectorExpr:
		return v.exprToString(e.X) + "." + e.Sel.Value
	default:
		return "unknown"
	}
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
	debug("map_dot: transformSelector %s.%s\n", v.exprToString(sel.X), sel.Sel.Value)
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

	// Handle cases where the base is a CallExpr that returns a map
	// e.g., units.Available().area -> units.Available()["area"]
	if callExpr, ok := sel.X.(*syntax.CallExpr); ok {
		if v.isMapReturningFunction(callExpr) {
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

// isMapReturningFunction checks if a function call returns a map with string keys
func (v *mapDotVisitor) isMapReturningFunction(callExpr *syntax.CallExpr) bool {
	// Check for specific known functions that return maps
	if selectorExpr, ok := callExpr.Fun.(*syntax.SelectorExpr); ok {
		// Handle method calls like units.Available()
		if pkg, ok := selectorExpr.X.(*syntax.Name); ok {
			pkgName := pkg.Value
			methodName := selectorExpr.Sel.Value
			
			// Known map-returning functions
			if pkgName == "units" && methodName == "Available" {
				return true
			}
			
			// Add more known functions as needed
		}
	}
	
	// Check for direct function calls with common map-returning names
	if name, ok := callExpr.Fun.(*syntax.Name); ok {
		functionName := name.Value
		mapReturningFunctions := []string{
			"getConfig", "getSettings", "getOptions", "getFlags", "getParams",
			"config", "settings", "options", "flags", "params",
			"Available", "available", "getData", "getMeta", "getProperties",
		}
		
		for _, mapFunc := range mapReturningFunctions {
			if functionName == mapFunc {
				return true
			}
		}
	}
	
	return false
}

func init() {
	RegisterTransformer(&MapDotTransform{})
}
