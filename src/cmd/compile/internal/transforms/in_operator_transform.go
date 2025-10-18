// Copyright 2025 The Goo Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transforms

import (
    "cmd/compile/internal/syntax"
    "strings"
)

// InOperatorTransform handles the 'in' operator for strings and collections
// Transforms expressions like "hello" in str to strings.Contains(str, "hello")
// and item in slice to slices.Contains(slice, item)
type InOperatorTransform struct{}

type inVisitor struct {
    transform *InOperatorTransform
    ctx       *TransformContext
    file      *syntax.File
    changed   bool
}

func (t *InOperatorTransform) Name() string {
	return "in_operator_transform"
}

func (t *InOperatorTransform) Priority() int {
	return 100 // Default priority - between list methods (50) and lambda (200)
}

func (t *InOperatorTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
    changed := false
    visitor := &inVisitor{transform: t, ctx: ctx, file: file}
    // Use syntax.Walk to traverse the entire AST
    syntax.Walk(file, visitor)

    // Second pass: statement-level rewrites for map membership to avoid func lits
    // Walk function bodies and transform AssignStmt/CheckStmt/IfStmt with map 'in'
    changed = changed || visitor.changed
    for i, decl := range file.DeclList {
        if f, ok := decl.(*syntax.FuncDecl); ok && f.Body != nil {
            if newBody := t.rewriteMapMembershipInBlock(f.Body, ctx); newBody != f.Body {
                nf := *f
                if bs, ok := newBody.(*syntax.BlockStmt); ok {
                    nf.Body = bs
                    file.DeclList[i] = &nf
                    changed = changed || true
                }
            }
        }
    }

    // Also handle top-level statements for implicit main files (map and slice membership)
    if len(file.TopLevelStmts) > 0 {
        if newList, ok := t.rewriteMapMembershipInList(file.TopLevelStmts, ctx); ok {
            file.TopLevelStmts = newList
            changed = true
        }
    }

    return changed
}

// Visit implements syntax.Visitor interface
func (v *inVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}
	
	// Transform nodes that contain expressions that might have 'in' operations
    switch n := node.(type) {
    case *syntax.VarDecl:
		if n.Values != nil {
			if transformed := v.transform.transformExpr(n.Values, v); transformed != n.Values {
				n.Values = transformed
				v.changed = true
			}
		}
    case *syntax.AssignStmt:
        // First, handle map membership rewrite directly into multi-assign
        if n.Rhs != nil {
            if op, ok := v.transform.isMapInOperation(n.Rhs, v.ctx); ok {
                pos := n.Pos()
                idx := &syntax.IndexExpr{X: op.Y, Index: op.X}
                if pos.IsKnown() { idx.SetPos(pos) }
                blank := &syntax.Name{Value: "_"}
                if pos.IsKnown() { blank.SetPos(pos) }
                lhsList := &syntax.ListExpr{ElemList: []syntax.Expr{blank, n.Lhs}}
                if pos.IsKnown() { lhsList.SetPos(pos) }
                n.Lhs = lhsList
                n.Rhs = idx
                v.changed = true
            } else {
                if transformed := v.transform.transformExpr(n.Rhs, v); transformed != n.Rhs {
                    n.Rhs = transformed
                    v.changed = true
                }
            }
        }
	case *syntax.CheckStmt:
		if n.Cond != nil {
			if transformed := v.transform.transformExpr(n.Cond, v); transformed != n.Cond {
				n.Cond = transformed
				v.changed = true
			}
		}
	case *syntax.ExprStmt:
		if transformed := v.transform.transformExpr(n.X, v); transformed != n.X {
			n.X = transformed
			v.changed = true
		}
	case *syntax.IfStmt:
		if n.Cond != nil {
			if transformed := v.transform.transformExpr(n.Cond, v); transformed != n.Cond {
				n.Cond = transformed
				v.changed = true
			}
		}
    case *syntax.ForStmt:
        if n.Cond != nil {
            if transformed := v.transform.transformExpr(n.Cond, v); transformed != n.Cond {
                n.Cond = transformed
                v.changed = true
            }
        }
    case *syntax.BlockStmt:
        // Rewrite map membership at statement level within this block
        if newBlock := v.transform.rewriteMapMembershipInBlock(n, v.ctx); newBlock != n {
            if nb, ok := newBlock.(*syntax.BlockStmt); ok {
                n.List = nb.List
                v.changed = true
            }
        }
    }
	
	// Continue visiting child nodes
	return v
}

// transformExpr transforms a single expression
func (t *InOperatorTransform) transformExpr(expr syntax.Expr, visitor *inVisitor) syntax.Expr {
	if expr == nil {
		return expr
	}
	
	// Check for 'in' operations
	if op, ok := expr.(*syntax.Operation); ok {
		if op.Op == syntax.In {
			if transformed := t.convertInOperation(op, visitor, visitor.file); transformed != nil {
				visitor.changed = true
				return transformed
			}
		}
		// Transform operands recursively
		if op.X != nil {
			op.X = t.transformExpr(op.X, visitor)
		}
		if op.Y != nil {
			op.Y = t.transformExpr(op.Y, visitor)
		}
	}
	
	// Handle other expression types that might contain sub-expressions
	switch e := expr.(type) {
	case *syntax.CallExpr:
		for i, arg := range e.ArgList {
			e.ArgList[i] = t.transformExpr(arg, visitor)
		}
	case *syntax.ParenExpr:
		e.X = t.transformExpr(e.X, visitor)
	case *syntax.ListExpr:
		for i, elem := range e.ElemList {
			e.ElemList[i] = t.transformExpr(elem, visitor)
		}
	}
	
	return expr
}

// convertInOperation converts "item in collection" to appropriate Go code
func (t *InOperatorTransform) convertInOperation(op *syntax.Operation, visitor *inVisitor, file *syntax.File) syntax.Expr {
	// Determine the type of operation based on the container (op.Y)
	containerType := t.inferContainerType(op.Y, visitor.ctx)
	println("DEBUG: convertInOperation containerType =", containerType)
	
    switch containerType {
    case "string":
        // Defer to string_methods_transform by emitting receiver.contains(arg)
        sel := &syntax.SelectorExpr{X: op.Y, Sel: &syntax.Name{Value: "contains"}}
        call := &syntax.CallExpr{Fun: sel, ArgList: []syntax.Expr{op.X}}
        return call
    case "slice":
        return t.createSliceContainsCall(op, visitor)
    case "map":
        // Defer to statement-level rewrite to avoid fragile func-lits here
        return op
    case "iterator":
        // Defer or handle elsewhere; keep unchanged for now
        return op
    default:
        // Try string fallback
        return t.createStringContainsCall(op, visitor, syntax.Pos{})
    }
}

// ---- Statement-level rewrites for map membership ----

// rewriteMapMembershipInBlock rewrites statements inside a block to eliminate
// expression-level map membership for contexts like assignments, checks, and if-conds.
func (t *InOperatorTransform) rewriteMapMembershipInBlock(block *syntax.BlockStmt, ctx *TransformContext) syntax.Stmt {
    if block == nil { return block }
    changed := false
    newList := make([]syntax.Stmt, 0, len(block.List))
    for _, stmt := range block.List {
        switch s := stmt.(type) {
        case *syntax.AssignStmt:
            if repl, ok := t.rewriteAssignWithMapIn(s, ctx); ok {
                newList = append(newList, repl...)
                changed = true
                continue
            }
            newList = append(newList, s)
        case *syntax.CheckStmt:
            if repl, ok := t.rewriteCheckWithMapIn(s, ctx); ok {
                newList = append(newList, repl)
                changed = true
                continue
            }
            newList = append(newList, s)
        case *syntax.IfStmt:
            if repl, ok := t.rewriteIfWithMapIn(s, ctx); ok {
                newList = append(newList, repl)
                changed = true
                continue
            }
            // Also dive into nested blocks
            if s.Then != nil {
                if nb := t.rewriteMapMembershipInBlock(s.Then, ctx); nb != s.Then {
                    ns := *s; ns.Then = nb.(*syntax.BlockStmt); s = &ns; changed = true
                }
            }
            if s.Else != nil {
                if bs, ok := s.Else.(*syntax.BlockStmt); ok {
                    if nb := t.rewriteMapMembershipInBlock(bs, ctx); nb != bs {
                        ns := *s; ns.Else = nb; s = &ns; changed = true
                    }
                }
            }
            newList = append(newList, s)
        case *syntax.BlockStmt:
            if nb := t.rewriteMapMembershipInBlock(s, ctx); nb != s {
                newList = append(newList, nb)
                changed = true
            } else {
                newList = append(newList, s)
            }
        default:
            newList = append(newList, s)
        }
    }
    if !changed { return block }
    nb := *block
    nb.List = newList
    return &nb
}

// rewriteMapMembershipInList applies map-in rewrites to a list of statements.
func (t *InOperatorTransform) rewriteMapMembershipInList(list []syntax.Stmt, ctx *TransformContext) ([]syntax.Stmt, bool) {
    changed := false
    out := make([]syntax.Stmt, 0, len(list))
    for _, stmt := range list {
        switch s := stmt.(type) {
        case *syntax.AssignStmt:
            if repl, ok := t.rewriteAssignWithMapIn(s, ctx); ok {
                out = append(out, repl...)
                changed = true
                continue
            }
            if repl, ok := t.rewriteAssignWithSliceIn(s, ctx); ok {
                out = append(out, repl...)
                changed = true
                continue
            }
            out = append(out, s)
        case *syntax.CheckStmt:
            if repl, ok := t.rewriteCheckWithMapIn(s, ctx); ok {
                out = append(out, repl)
                changed = true
                continue
            }
            if repl, ok := t.rewriteCheckWithSliceIn(s, ctx); ok {
                out = append(out, repl)
                changed = true
                continue
            }
            out = append(out, s)
        case *syntax.IfStmt:
            if repl, ok := t.rewriteIfWithMapIn(s, ctx); ok {
                out = append(out, repl)
                changed = true
                continue
            }
            // Dive into nested
            if s.Then != nil {
                if nb := t.rewriteMapMembershipInBlock(s.Then, ctx); nb != s.Then {
                    ns := *s; ns.Then = nb.(*syntax.BlockStmt); s = &ns; changed = true
                }
            }
            if s.Else != nil {
                if bs, ok := s.Else.(*syntax.BlockStmt); ok {
                    if nb := t.rewriteMapMembershipInBlock(bs, ctx); nb != bs {
                        ns := *s; ns.Else = nb; s = &ns; changed = true
                    }
                }
            }
            out = append(out, s)
        case *syntax.BlockStmt:
            if nb := t.rewriteMapMembershipInBlock(s, ctx); nb != s {
                out = append(out, nb)
                changed = true
            } else {
                out = append(out, s)
            }
        default:
            out = append(out, s)
        }
    }
    return out, changed
}

func (t *InOperatorTransform) isMapInOperation(expr syntax.Expr, ctx *TransformContext) (*syntax.Operation, bool) {
    // Unwrap parentheses
    for {
        if p, ok := expr.(*syntax.ParenExpr); ok && p.X != nil {
            expr = p.X
        } else {
            break
        }
    }
    op, ok := expr.(*syntax.Operation)
    if !ok || op.Op != syntax.In { return nil, false }
    if t.inferContainerType(op.Y, ctx) == "map" { return op, true }
    return nil, false
}

// rewriteAssignWithMapIn handles: lhs := (key in m)  or  lhs = (key in m)
// Rewrites to:
//   // def: lhs := false
//   _, ok := m[key]
//   lhs = ok   // or lhs := ok for def with no prior init
func (t *InOperatorTransform) rewriteAssignWithMapIn(as *syntax.AssignStmt, ctx *TransformContext) ([]syntax.Stmt, bool) {
    op, ok := t.isMapInOperation(as.Rhs, ctx)
    if !ok { return nil, false }
    pos := as.Pos()

    // Transform into a single multi-assign using comma-ok: _, lhs := m[key]
    idx := &syntax.IndexExpr{X: op.Y, Index: op.X}
    idx.SetPos(pos)
    blank := &syntax.Name{Value: "_"}; blank.SetPos(pos)
    // Prepare LHS list: _, <lhs>
    var lhsVar syntax.Expr = as.Lhs
    // as.Lhs should be a Name or similar; keep as is
    lhsList := &syntax.ListExpr{ElemList: []syntax.Expr{blank, lhsVar}}
    lhsList.SetPos(pos)
    newAssign := &syntax.AssignStmt{Op: as.Op, Lhs: lhsList, Rhs: idx}
    newAssign.SetPos(pos)
    return []syntax.Stmt{newAssign}, true
}

// rewriteAssignWithSliceIn handles: lhs := (item in slice) or lhs = (item in slice)
// Rewrites to: lhs := false; for i := 0; i < len(slice); i++ { if slice[i] == item { lhs = true; break } }
func (t *InOperatorTransform) rewriteAssignWithSliceIn(as *syntax.AssignStmt, ctx *TransformContext) ([]syntax.Stmt, bool) {
    // Check if this is a slice 'in' operation
    op, ok := t.isSliceInOperation(as.Rhs, ctx)
    if !ok { return nil, false }
    
    // Initialize lhs to false
    falseLit := &syntax.Name{Value: "false"}
    initAssign := &syntax.AssignStmt{Op: as.Op, Lhs: as.Lhs, Rhs: falseLit}
    
    // Create loop variables
    iVar := &syntax.Name{Value: "i"}
    zero := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
    one := &syntax.BasicLit{Kind: syntax.IntLit, Value: "1"}
    
    // Create: i := 0
    initI := &syntax.AssignStmt{Op: syntax.Def, Lhs: iVar, Rhs: zero}
    
    // Create: len(slice)
    lenFunc := &syntax.Name{Value: "len"}
    lenCall := &syntax.CallExpr{Fun: lenFunc, ArgList: []syntax.Expr{op.Y}}
    
    // Create: i < len(slice)
    condition := &syntax.Operation{Op: syntax.Lss, X: iVar, Y: lenCall}
    
    // Create: i++
    incI := &syntax.AssignStmt{Op: syntax.Add, Lhs: iVar, Rhs: one}
    
    // Create: slice[i]
    indexExpr := &syntax.IndexExpr{X: op.Y, Index: iVar}
    
    // Create: slice[i] == item
    comparison := &syntax.Operation{Op: syntax.Eql, X: indexExpr, Y: op.X}
    
    // Create: lhs = true
    trueLit := &syntax.Name{Value: "true"}
    setTrue := &syntax.AssignStmt{Op: 0, Lhs: as.Lhs, Rhs: trueLit}
    
    // Create: break
    breakStmt := &syntax.BranchStmt{Tok: syntax.Break}
    
    // Create: if slice[i] == item { lhs = true; break }
    ifBody := &syntax.BlockStmt{List: []syntax.Stmt{setTrue, breakStmt}}
    ifStmt := &syntax.IfStmt{Cond: comparison, Then: ifBody}
    
    // Create loop body
    loopBody := &syntax.BlockStmt{List: []syntax.Stmt{ifStmt}}
    
    // Create: for i := 0; i < len(slice); i++ { ... }
    forStmt := &syntax.ForStmt{Init: initI, Cond: condition, Post: incI, Body: loopBody}
    
    return []syntax.Stmt{initAssign, forStmt}, true
}

// isSliceInOperation checks if an expression is a slice 'in' operation
func (t *InOperatorTransform) isSliceInOperation(expr syntax.Expr, ctx *TransformContext) (*syntax.Operation, bool) {
    // Unwrap parentheses
    for {
        if p, ok := expr.(*syntax.ParenExpr); ok && p.X != nil {
            expr = p.X
        } else { break }
    }
    op, ok := expr.(*syntax.Operation)
    if !ok || op.Op != syntax.In { return nil, false }
    
    containerType := t.inferContainerType(op.Y, ctx)
    
    if containerType == "slice" { return op, true }
    return nil, false
}

// rewriteCheckWithMapIn handles: check (key in m)
// Rewrites to block: { _, ok := m[key]; check ok }
func (t *InOperatorTransform) rewriteCheckWithMapIn(cs *syntax.CheckStmt, ctx *TransformContext) (syntax.Stmt, bool) {
    op, ok := t.isMapInOperation(cs.Cond, ctx)
    if !ok { return nil, false }
    
    pos := cs.Pos()
    idx := &syntax.IndexExpr{X: op.Y, Index: op.X}
    if pos.IsKnown() { idx.SetPos(pos) }
    
    blank := &syntax.Name{Value: "_"}
    if pos.IsKnown() { blank.SetPos(pos) }
    
    okName := &syntax.Name{Value: "ok"}
    if pos.IsKnown() { okName.SetPos(pos) }
    
    lhsList := &syntax.ListExpr{ElemList: []syntax.Expr{blank, okName}}
    if pos.IsKnown() { lhsList.SetPos(pos) }
    
    assignOk := &syntax.AssignStmt{Op: syntax.Def, Lhs: lhsList, Rhs: idx}
    if pos.IsKnown() { assignOk.SetPos(pos) }
    
    newCheck := &syntax.CheckStmt{Cond: okName}
    if pos.IsKnown() { newCheck.SetPos(pos) }
    
    bs := &syntax.BlockStmt{List: []syntax.Stmt{assignOk, newCheck}}
    if pos.IsKnown() { bs.SetPos(pos) }
    
    return bs, true
}

// rewriteCheckWithSliceIn handles: check (item in slice) for variable slices
// Rewrites to: { ok := false; for i := 0; i < len(slice); i++ { if slice[i] == item { ok = true; break } } check ok }
func (t *InOperatorTransform) rewriteCheckWithSliceIn(cs *syntax.CheckStmt, ctx *TransformContext) (syntax.Stmt, bool) {
    // Check if this is a slice 'in' operation
    cond := cs.Cond
    for {
        if p, ok := cond.(*syntax.ParenExpr); ok && p.X != nil {
            cond = p.X
        } else { break }
    }
    op, ok := cond.(*syntax.Operation)
    if !ok || op.Op != syntax.In { return nil, false }
    if t.inferContainerType(op.Y, ctx) != "slice" { return nil, false }
    
    // Create: ok := false
    okName := &syntax.Name{Value: "ok"}
    falseLit := &syntax.Name{Value: "false"}
    okInit := &syntax.AssignStmt{Op: syntax.Def, Lhs: okName, Rhs: falseLit}
    
    // Create loop variables
    iVar := &syntax.Name{Value: "i"}
    zero := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
    one := &syntax.BasicLit{Kind: syntax.IntLit, Value: "1"}
    
    // Create: i := 0
    initI := &syntax.AssignStmt{Op: syntax.Def, Lhs: iVar, Rhs: zero}
    
    // Create: len(slice)
    lenFunc := &syntax.Name{Value: "len"}
    lenCall := &syntax.CallExpr{Fun: lenFunc, ArgList: []syntax.Expr{op.Y}}
    
    // Create: i < len(slice)
    condition := &syntax.Operation{Op: syntax.Lss, X: iVar, Y: lenCall}
    
    // Create: i++
    incI := &syntax.AssignStmt{Op: syntax.Add, Lhs: iVar, Rhs: one}
    
    // Create: slice[i]
    indexExpr := &syntax.IndexExpr{X: op.Y, Index: iVar}
    
    // Create: slice[i] == item
    comparison := &syntax.Operation{Op: syntax.Eql, X: indexExpr, Y: op.X}
    
    // Create: ok = true
    trueLit := &syntax.Name{Value: "true"}
    setOk := &syntax.AssignStmt{Op: 0, Lhs: okName, Rhs: trueLit}
    
    // Create: break
    breakStmt := &syntax.BranchStmt{Tok: syntax.Break}
    
    // Create: if slice[i] == item { ok = true; break }
    ifBody := &syntax.BlockStmt{List: []syntax.Stmt{setOk, breakStmt}}
    ifStmt := &syntax.IfStmt{Cond: comparison, Then: ifBody}
    
    // Create loop body
    loopBody := &syntax.BlockStmt{List: []syntax.Stmt{ifStmt}}
    
    // Create: for i := 0; i < len(slice); i++ { ... }
    forStmt := &syntax.ForStmt{Init: initI, Cond: condition, Post: incI, Body: loopBody}
    
    // Create: check ok
    newCheck := &syntax.CheckStmt{Cond: okName}
    
    // Create block containing all statements
    block := &syntax.BlockStmt{List: []syntax.Stmt{okInit, forStmt, newCheck}}
    
    return block, true
}

// rewriteIfWithMapIn handles: if (key in m) { ... }
// Rewrites to: if _, ok := m[key]; ok { ... }
func (t *InOperatorTransform) rewriteIfWithMapIn(is *syntax.IfStmt, ctx *TransformContext) (syntax.Stmt, bool) {
    op, ok := t.isMapInOperation(is.Cond, ctx)
    if !ok { return nil, false }
    
    pos := is.Pos()
    idx := &syntax.IndexExpr{X: op.Y, Index: op.X}
    if pos.IsKnown() { idx.SetPos(pos) }
    
    blank := &syntax.Name{Value: "_"}
    if pos.IsKnown() { blank.SetPos(pos) }
    
    okName := &syntax.Name{Value: "ok"}
    if pos.IsKnown() { okName.SetPos(pos) }
    
    lhsList := &syntax.ListExpr{ElemList: []syntax.Expr{blank, okName}}
    if pos.IsKnown() { lhsList.SetPos(pos) }
    
    init := &syntax.AssignStmt{Op: syntax.Def, Lhs: lhsList, Rhs: idx}
    if pos.IsKnown() { init.SetPos(pos) }
    
    ni := *is
    ni.Init = init
    ni.Cond = okName
    return &ni, true
}

// rewriteCheckWithStringIn handles: check (needle in haystack) for strings without stdlib
// Rewrites to a small loop computing ok and then: check ok
func (t *InOperatorTransform) rewriteCheckWithStringIn(cs *syntax.CheckStmt, ctx *TransformContext) (syntax.Stmt, bool) {
    // Unwrap parentheses
    cond := cs.Cond
    for {
        if p, ok := cond.(*syntax.ParenExpr); ok && p.X != nil {
            cond = p.X
        } else { break }
    }
    op, ok := cond.(*syntax.Operation)
    if !ok || op.Op != syntax.In { return nil, false }
    if t.inferContainerType(op.Y, ctx) != "string" { return nil, false }

    pos := cs.Pos()
    // ok := false
    okName := &syntax.Name{Value: "ok"}
    okAssign := &syntax.AssignStmt{Op: syntax.Def, Lhs: okName, Rhs: &syntax.Name{Value: "false"}}
    if pos.IsKnown() { okAssign.SetPos(pos) }
    // h := op.Y; n := op.X (convert rune to string if needed)
    h := &syntax.Name{Value: "h"}
    n := &syntax.Name{Value: "n"}
    hAssign := &syntax.AssignStmt{Op: syntax.Def, Lhs: h, Rhs: op.Y}
    nRhs := op.X
    if t.isRuneLiteral(op.X) { nRhs = t.convertRuneToString(op.X, pos) }
    nAssign := &syntax.AssignStmt{Op: syntax.Def, Lhs: n, Rhs: nRhs}
    // i loop: for i := 0; i <= len(h)-len(n); i++
    i := &syntax.Name{Value: "i"}
    zero := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
    initI := &syntax.AssignStmt{Op: syntax.Def, Lhs: i, Rhs: zero}
    lenName := &syntax.Name{Value: "len"}
    lenH := &syntax.CallExpr{Fun: lenName, ArgList: []syntax.Expr{h}}
    lenN := &syntax.CallExpr{Fun: lenName, ArgList: []syntax.Expr{n}}
    limit := &syntax.Operation{Op: syntax.Sub, X: lenH, Y: lenN}
    condI := &syntax.Operation{Op: syntax.Leq, X: i, Y: limit}
    one := &syntax.BasicLit{Kind: syntax.IntLit, Value: "1"}
    incI := &syntax.AssignStmt{Op: syntax.Add, Lhs: i, Rhs: one}
    // if h[i:i+len(n)] == n { ok = true }
    end := &syntax.Operation{Op: syntax.Add, X: i, Y: lenN}
    slice := &syntax.SliceExpr{X: h, Index: [3]syntax.Expr{i, end, nil}}
    eq := &syntax.Operation{Op: syntax.Eql, X: slice, Y: n}
    setOk := &syntax.AssignStmt{Op: 0, Lhs: okName, Rhs: &syntax.Name{Value: "true"}}
    ifThen := &syntax.BlockStmt{List: []syntax.Stmt{setOk}}
    ifStmt := &syntax.IfStmt{Cond: eq, Then: ifThen}
    loopBody := &syntax.BlockStmt{List: []syntax.Stmt{ifStmt}}
    forStmt := &syntax.ForStmt{Init: initI, Cond: condI, Post: incI, Body: loopBody}
    // check ok
    newCheck := &syntax.CheckStmt{Cond: okName}
    bs := &syntax.BlockStmt{List: []syntax.Stmt{okAssign, hAssign, nAssign, forStmt, newCheck}}
    return bs, true
}

// inferContainerType tries to determine if the container is a string, slice, or map
func (t *InOperatorTransform) inferContainerType(container syntax.Expr, ctx *TransformContext) string {
	// Check for string literals
	if basic, ok := container.(*syntax.BasicLit); ok {
		if basic.Kind == syntax.StringLit {
			return "string"
		}
	}
	
	// Check for composite literals (slices/arrays/maps)
	if comp, ok := container.(*syntax.CompositeLit); ok {
		if comp.Type != nil {
			// Check for map types
			if _, isMap := comp.Type.(*syntax.MapType); isMap {
				return "map"
			}
			// Check for slice/array types
			if _, isSlice := comp.Type.(*syntax.SliceType); isSlice {
				return "slice"
			}
			if _, isArray := comp.Type.(*syntax.ArrayType); isArray {
				return "slice"
			}
		}
		// If no explicit type, infer from usage - composite literals are usually slices
		return "slice"
	}
	
	// Check for iterator function calls
	if t.isIteratorType(container) {
		return "iterator"
	}
	
	// Check context for variable types
	if name, ok := container.(*syntax.Name); ok && ctx != nil && ctx.Types != nil {
		if varType, exists := ctx.Types[name.Value]; exists {
			if strings.Contains(varType, "[]") {
				return "slice"
			}
			if strings.Contains(varType, "map[") {
				return "map"
			}
			if varType == "string" {
				return "string"
			}
		}
	}
	
	return "unknown"
}

// createStringContainsCall creates strings.Contains(container, item) or inline version for GOPATH mode
func (t *InOperatorTransform) createStringContainsCall(op *syntax.Operation, visitor *inVisitor, pos syntax.Pos) syntax.Expr {
    if !pos.IsKnown() {
        pos = generatedNodePos(visitor.file)
    }
    return t.createInlineStringContains(op, visitor, pos)
}

// createInlineStringContains creates inline string containment check for GOPATH mode
// Uses simple logic to avoid import dependencies
func (t *InOperatorTransform) createInlineStringContains(op *syntax.Operation, visitor *inVisitor, pos syntax.Pos) syntax.Expr {
	// Handle string literal in string literal case
	if itemExpr, ok := op.X.(*syntax.BasicLit); ok && itemExpr.Kind == syntax.StringLit {
		if containerExpr, ok := op.Y.(*syntax.BasicLit); ok && containerExpr.Kind == syntax.StringLit {
			// Both are string literals - compute at compile time
			result := t.computeStringInString(itemExpr.Value, containerExpr.Value)
			resultLit := &syntax.Name{Value: result}
			resultLit.SetPos(pos)
			return resultLit
		}
	}
	
	// Handle rune literal in string literal case  
	if runeExpr, ok := op.X.(*syntax.BasicLit); ok && runeExpr.Kind == syntax.RuneLit {
		if stringExpr, ok := op.Y.(*syntax.BasicLit); ok && stringExpr.Kind == syntax.StringLit {
			// Both are literals - compute at compile time
			result := t.computeRuneInString(runeExpr.Value, stringExpr.Value)
			resultLit := &syntax.Name{Value: result}
			resultLit.SetPos(pos)
			return resultLit
		}
	}
	
    // Non-literal general case: inline naive substring search
    // Build: func() bool {
    //   h := <op.Y>; n := <item>
    //   if len(n) == 0 { return true }
    //   for i := 0; i <= len(h)-len(n); i++ { if h[i:i+len(n)] == n { return true } }
    //   return false
    // }()

    // Convert rune needle to string if needed
    item := op.X
    if t.isRuneLiteral(op.X) {
        item = t.convertRuneToString(op.X, pos, visitor.file)
    }

    hVar := &syntax.Name{Value: "h"}
    hVar.SetPos(pos)
    nVar := &syntax.Name{Value: "n"}
    nVar.SetPos(pos)

    hAssign := &syntax.AssignStmt{Op: syntax.Def, Lhs: hVar, Rhs: op.Y}
    hAssign.SetPos(pos)
    nAssign := &syntax.AssignStmt{Op: syntax.Def, Lhs: nVar, Rhs: item}
    nAssign.SetPos(pos)

    zero := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
    zero.SetPos(pos)
    one := &syntax.BasicLit{Kind: syntax.IntLit, Value: "1"}
    one.SetPos(pos)

    lenNameH := &syntax.Name{Value: "len"}
    lenNameH.SetPos(pos)
    lenH := &syntax.CallExpr{Fun: lenNameH, ArgList: []syntax.Expr{hVar}}
    lenH.SetPos(pos)

    lenNameN := &syntax.Name{Value: "len"}
    lenNameN.SetPos(pos)
    lenN := &syntax.CallExpr{Fun: lenNameN, ArgList: []syntax.Expr{nVar}}
    lenN.SetPos(pos)

    // if len(n) == 0 { return true }
    retTrueName := &syntax.Name{Value: "true"}
    retTrueName.SetPos(pos)
    retTrue := &syntax.ReturnStmt{Results: retTrueName}
    retTrue.SetPos(pos)
    lenEqZero := &syntax.Operation{Op: syntax.Eql, X: lenN, Y: zero}
    lenEqZero.SetPos(pos)
    ifEmpty := &syntax.IfStmt{Cond: lenEqZero, Then: &syntax.BlockStmt{List: []syntax.Stmt{retTrue}}}
    ifEmpty.SetPos(pos)

    // if len(n) > len(h) { return false }
    retFalseName := &syntax.Name{Value: "false"}
    retFalseName.SetPos(pos)
    retFalseEarly := &syntax.ReturnStmt{Results: retFalseName}
    retFalseEarly.SetPos(pos)
    lenGT := &syntax.Operation{Op: syntax.Gtr, X: lenN, Y: lenH}
    lenGT.SetPos(pos)
    ifTooLong := &syntax.IfStmt{Cond: lenGT, Then: &syntax.BlockStmt{List: []syntax.Stmt{retFalseEarly}}}
    ifTooLong.SetPos(pos)

    // for i := 0; i <= len(h)-len(n); i++
    iVar := &syntax.Name{Value: "i"}
    iVar.SetPos(pos)
    initI := &syntax.AssignStmt{Op: syntax.Def, Lhs: iVar, Rhs: zero}
    initI.SetPos(pos)
    lenHMinusLenN := &syntax.Operation{Op: syntax.Sub, X: lenH, Y: lenN}
    lenHMinusLenN.SetPos(pos)
    cond := &syntax.Operation{Op: syntax.Leq, X: iVar, Y: lenHMinusLenN}
    cond.SetPos(pos)
    inc := &syntax.AssignStmt{Op: syntax.Add, Lhs: iVar, Rhs: one}
    inc.SetPos(pos)

    end := &syntax.Operation{Op: syntax.Add, X: iVar, Y: lenN}
    end.SetPos(pos)
    slice := &syntax.SliceExpr{X: hVar, Index: [3]syntax.Expr{iVar, end, nil}}
    slice.SetPos(pos)
    eq := &syntax.Operation{Op: syntax.Eql, X: slice, Y: nVar}
    eq.SetPos(pos)
    returnTrue := &syntax.ReturnStmt{Results: retTrueName}
    returnTrue.SetPos(pos)
    ifMatch := &syntax.IfStmt{Cond: eq, Then: &syntax.BlockStmt{List: []syntax.Stmt{returnTrue}}}
    ifMatch.SetPos(pos)

    loopBody := &syntax.BlockStmt{List: []syntax.Stmt{ifMatch}}
    loopBody.SetPos(pos)
    loop := &syntax.ForStmt{Init: initI, Cond: cond, Post: inc, Body: loopBody}
    loop.SetPos(pos)

    retFalse := &syntax.ReturnStmt{Results: retFalseName}
    retFalse.SetPos(pos)

    body := &syntax.BlockStmt{List: []syntax.Stmt{hAssign, nAssign, ifEmpty, ifTooLong, loop, retFalse}}
    body.SetPos(pos)

    boolType := &syntax.Name{Value: "bool"}
    boolType.SetPos(pos)
    field := &syntax.Field{Type: boolType}
    field.SetPos(pos)
    funcType := &syntax.FuncType{ResultList: []*syntax.Field{field}}
    funcType.SetPos(pos)
    fn := &syntax.FuncLit{Type: funcType, Body: body}
    fn.SetPos(pos)
    call := &syntax.CallExpr{Fun: fn}
    call.SetPos(pos)
    return call
}

// Helper function to get expression type as string for debugging
func (t *InOperatorTransform) getExprType(expr syntax.Expr) string {
	switch e := expr.(type) {
	case *syntax.BasicLit:
		kindStr := "unknown"
		switch e.Kind {
		case syntax.IntLit:
			kindStr = "IntLit"
		case syntax.FloatLit:
			kindStr = "FloatLit"
		case syntax.ImagLit:
			kindStr = "ImagLit"
		case syntax.RuneLit:
			kindStr = "RuneLit"
		case syntax.StringLit:
			kindStr = "StringLit"
		}
		return "BasicLit(" + kindStr + ":" + e.Value + ")"
	case *syntax.Name:
		return "Name(" + e.Value + ")"
	case *syntax.CallExpr:
		return "CallExpr"
	case *syntax.Operation:
		return "Operation"
	default:
		return "Unknown"
	}
}

// createSliceContainsCall creates a manual loop to check slice contains without imports
func (t *InOperatorTransform) createSliceContainsCall(op *syntax.Operation, visitor *inVisitor) syntax.Expr {
	// For eval and simple cases, handle small literal slices by expanding to OR comparisons
	if compLit, ok := op.Y.(*syntax.CompositeLit); ok {
		if len(compLit.ElemList) <= 10 && len(compLit.ElemList) > 0 {
			// Create chain of comparisons: item == elem1 || item == elem2 || ...
			var result syntax.Expr
			
			for i, elem := range compLit.ElemList {
				comparison := &syntax.Operation{
					Op: syntax.Eql,
					X:  op.X, // item
					Y:  elem,
				}
				// Skip SetPos to avoid PosBase panic
				// if pos.IsKnown() { comparison.SetPos(pos) }
				
				if i == 0 {
					result = comparison
				} else {
					result = &syntax.Operation{
						Op: syntax.OrOr,
						X:  result,
						Y:  comparison,
					}
					// Skip SetPos to avoid PosBase panic
					// if pos.IsKnown() { result.SetPos(pos) }
				}
			}
			
			return result
		}
	}
	
    // For non-literal slices (variables), use slices.Contains(slice, item)
    slicesName := &syntax.Name{Value: "slices"}
    containsName := &syntax.Name{Value: "Contains"}
    slicesContains := &syntax.SelectorExpr{X: slicesName, Sel: containsName}
    return &syntax.CallExpr{Fun: slicesContains, ArgList: []syntax.Expr{op.Y, op.X}}
}

// createMapContainsCall creates map key existence check: _, ok := map[key]; ok
func (t *InOperatorTransform) createMapContainsCall(op *syntax.Operation, visitor *inVisitor, pos syntax.Pos) syntax.Expr {
	// Create anonymous function that returns the existence check
	// Transforms: key in myMap  =>  func() bool { _, ok := myMap[key]; return ok }()
	
	// Create map index expression: myMap[key]
	indexExpr := &syntax.IndexExpr{
		X:     op.Y, // the map
		Index: op.X, // the key
	}
	indexExpr.SetPos(pos)
	
	// Create assignment: _, ok := myMap[key]
	blankVar := &syntax.Name{Value: "_"}
	blankVar.SetPos(pos)
	okVar := &syntax.Name{Value: "ok"}
	okVar.SetPos(pos)
	
	lhsList := &syntax.ListExpr{ElemList: []syntax.Expr{blankVar, okVar}}
	lhsList.SetPos(pos)
	
	assign := &syntax.AssignStmt{
		Op:  syntax.Def, // :=
		Lhs: lhsList,
		Rhs: indexExpr,
	}
	assign.SetPos(pos)
	
	// Create return statement: return ok
	returnStmt := &syntax.ReturnStmt{
		Results: okVar,
	}
	returnStmt.SetPos(pos)
	
	// Create function body
	body := &syntax.BlockStmt{
		List: []syntax.Stmt{assign, returnStmt},
	}
	body.SetPos(pos)
	
	// Create anonymous function
	boolType := &syntax.Name{Value: "bool"}
	boolType.SetPos(pos)
	
	funcLit := &syntax.FuncLit{
		Type: &syntax.FuncType{
			ResultList: []*syntax.Field{{Type: boolType}},
		},
		Body: body,
	}
	funcLit.SetPos(pos)
	funcLit.Type.SetPos(pos)
	
	// Create function call
	call := &syntax.CallExpr{
		Fun: funcLit,
	}
	call.SetPos(pos)
	
	return call
}

// createIteratorContainsCall creates iterator membership check using range loop
// item in iterator() => func() bool { for v := range iterator() { if v == item { return true } } return false }()
func (t *InOperatorTransform) createIteratorContainsCall(op *syntax.Operation, visitor *inVisitor, pos syntax.Pos) syntax.Expr {
	// Create loop variable
	loopVar := &syntax.Name{Value: "v"}
	loopVar.SetPos(pos)
	
	// Create range clause: for v := range iterator()
	rangeClause := &syntax.RangeClause{
		Lhs: loopVar,
		Def: true,
		X:   op.Y, // the iterator call
	}
	rangeClause.SetPos(pos)
	
	// Create comparison: v == item
	comparison := &syntax.Operation{
		Op: syntax.Eql,
		X:  loopVar,
		Y:  op.X, // the item to find
	}
	comparison.SetPos(pos)
	
	// Create return true statement
	trueReturn := &syntax.ReturnStmt{
		Results: &syntax.Name{Value: "true"},
	}
	trueReturn.SetPos(pos)
	trueReturn.Results.SetPos(pos)
	
	// Create if body
	ifBody := &syntax.BlockStmt{
		List: []syntax.Stmt{trueReturn},
	}
	ifBody.SetPos(pos)
	
	// Create if statement: if v == item { return true }
	ifStmt := &syntax.IfStmt{
		Cond: comparison,
		Then: ifBody,
	}
	ifStmt.SetPos(pos)
	
	// Create for loop body
	forBody := &syntax.BlockStmt{
		List: []syntax.Stmt{ifStmt},
	}
	forBody.SetPos(pos)
	
	// Create for loop: for v := range iterator() { if v == item { return true } }
	forStmt := &syntax.ForStmt{
		Init: rangeClause,
		Body: forBody,
	}
	forStmt.SetPos(pos)
	
	// Create return false statement
	falseReturn := &syntax.ReturnStmt{
		Results: &syntax.Name{Value: "false"},
	}
	falseReturn.SetPos(pos)
	falseReturn.Results.SetPos(pos)
	
	// Create function body: { for ... ; return false }
	funcBody := &syntax.BlockStmt{
		List: []syntax.Stmt{forStmt, falseReturn},
	}
	funcBody.SetPos(pos)
	
	// Create anonymous function
	boolType := &syntax.Name{Value: "bool"}
	boolType.SetPos(pos)
	
	funcLit := &syntax.FuncLit{
		Type: &syntax.FuncType{
			ResultList: []*syntax.Field{{Type: boolType}},
		},
		Body: funcBody,
	}
	funcLit.SetPos(pos)
	funcLit.Type.SetPos(pos)
	
	// Create function call
	call := &syntax.CallExpr{
		Fun: funcLit,
	}
	call.SetPos(pos)
	
	return call
}

// isIteratorType attempts to detect if the expression is likely an iterator (reused from in_loop_transform)
func (t *InOperatorTransform) isIteratorType(expr syntax.Expr) bool {
	// Check if it's a function call that might return an iterator
	if call, ok := expr.(*syntax.CallExpr); ok {
		// Check if the function name suggests it returns an iterator
		if name, ok := call.Fun.(*syntax.Name); ok {
			funcName := name.Value
			return t.looksLikeIteratorFunction(funcName)
		}
		
		// Check for selector expressions like somePackage.Iterator()
		if sel, ok := call.Fun.(*syntax.SelectorExpr); ok {
			return t.looksLikeIteratorFunction(sel.Sel.Value)
		}
	}
	
	return false
}

// looksLikeIteratorFunction checks if a function name suggests it returns an iterator
func (t *InOperatorTransform) looksLikeIteratorFunction(name string) bool {
	// Common patterns for iterator function names
	iteratorPatterns := []string{
		"Iter", "Iterator", "Items", "Values", "Keys", "Entries", 
		"Numbers", "Range", "Sequence", "Stream", "Generate",
	}
	
	for _, pattern := range iteratorPatterns {
		if name == pattern || 
		   len(name) > len(pattern) && name[len(name)-len(pattern):] == pattern ||
		   len(name) > len(pattern) && name[:len(pattern)] == pattern {
			return true
		}
	}
	
	return false
}

func (t *InOperatorTransform) hasImport(file *syntax.File, name string) bool {
	if name[0] != '"' {
		name = "\"" + name + "\""
	}
	for _, decl := range file.DeclList {
		if importDecl, ok := decl.(*syntax.ImportDecl); ok {
			if importDecl.Path != nil && importDecl.Path.Value == name {
				return true
			}
		}
	}
	return false
}

func (t *InOperatorTransform) addStringsImport(file *syntax.File) {
	if t.hasImport(file, "strings") {
		return
	}

    stringsImport := &syntax.ImportDecl{
        Path: &syntax.BasicLit{
            Value: "\"strings\"",
            Kind:  syntax.StringLit,
        },
    }

	var insertPos int
	for i, decl := range file.DeclList {
		if _, ok := decl.(*syntax.ImportDecl); ok {
			insertPos = i + 1
		} else {
			break
		}
	}

	newDeclList := make([]syntax.Decl, 0, len(file.DeclList)+1)
	newDeclList = append(newDeclList, file.DeclList[:insertPos]...)
	newDeclList = append(newDeclList, stringsImport)
	newDeclList = append(newDeclList, file.DeclList[insertPos:]...)
	file.DeclList = newDeclList
}

func (t *InOperatorTransform) addSlicesImport(file *syntax.File) {
	if t.hasImport(file, "slices") {
		return
	}

    slicesImport := &syntax.ImportDecl{
        Path: &syntax.BasicLit{
            Value: "\"slices\"",
            Kind:  syntax.StringLit,
        },
    }

	var insertPos int
	for i, decl := range file.DeclList {
		if _, ok := decl.(*syntax.ImportDecl); ok {
			insertPos = i + 1
		} else {
			break
		}
	}

	newDeclList := make([]syntax.Decl, 0, len(file.DeclList)+1)
	newDeclList = append(newDeclList, file.DeclList[:insertPos]...)
	newDeclList = append(newDeclList, slicesImport)
	newDeclList = append(newDeclList, file.DeclList[insertPos:]...)
	file.DeclList = newDeclList
}


// isRuneLiteral checks if an expression is a rune literal (e.g., 'a')
func (t *InOperatorTransform) isRuneLiteral(expr syntax.Expr) bool {
	if basic, ok := expr.(*syntax.BasicLit); ok {
		return basic.Kind == syntax.RuneLit
	}
	return false
}

// convertRuneToString converts a rune literal to string(rune) call
func (t *InOperatorTransform) convertRuneToString(runeExpr syntax.Expr, pos syntax.Pos) syntax.Expr {
    // Create string(rune) call
    stringName := &syntax.Name{Value: "string"}
    
    call := &syntax.CallExpr{
        Fun:     stringName,
        ArgList: []syntax.Expr{runeExpr},
    }
    
    return call
}

// computeStringInString computes whether a string literal is contained in another string literal at compile time
func (t *InOperatorTransform) computeStringInString(itemLiteral, containerLiteral string) string {
	// Parse the item string literal (e.g., "hello" -> hello)
	if len(itemLiteral) < 2 || itemLiteral[0] != '"' || itemLiteral[len(itemLiteral)-1] != '"' {
		return "false" // Invalid string literal
	}
	itemContent := itemLiteral[1 : len(itemLiteral)-1] // Remove quotes

	// Parse the container string literal (e.g., "hello world" -> hello world)
	if len(containerLiteral) < 2 || containerLiteral[0] != '"' || containerLiteral[len(containerLiteral)-1] != '"' {
		return "false" // Invalid string literal
	}
	containerContent := containerLiteral[1 : len(containerLiteral)-1] // Remove quotes

	// Simple substring check
	if strings.Contains(containerContent, itemContent) {
		return "true"
	}
	return "false"
}

// computeRuneInString computes whether a rune literal is contained in a string literal at compile time
func (t *InOperatorTransform) computeRuneInString(runeLiteral, stringLiteral string) string {
	// Parse the rune literal (e.g., 'a' -> a, '\n' -> newline)
	// runeLiteral includes the quotes, e.g., "'a'"
	if len(runeLiteral) < 3 || runeLiteral[0] != '\'' || runeLiteral[len(runeLiteral)-1] != '\'' {
		return "false" // Invalid rune literal
	}
	
	runeContent := runeLiteral[1 : len(runeLiteral)-1] // Remove quotes
	
	// Parse the string literal (e.g., "abc" -> abc)
	// stringLiteral includes the quotes, e.g., "\"abc\""
	if len(stringLiteral) < 2 || stringLiteral[0] != '"' || stringLiteral[len(stringLiteral)-1] != '"' {
		return "false" // Invalid string literal
	}
	
	stringContent := stringLiteral[1 : len(stringLiteral)-1] // Remove quotes
	
	// For simple ASCII characters, do basic containment check
	if len(runeContent) == 1 && runeContent[0] < 128 {
		// Simple ASCII character
		char := runeContent[0]
		for i := 0; i < len(stringContent); i++ {
			if stringContent[i] == char {
				return "true"
			}
		}
		return "false"
	}
	
	// For more complex cases (escape sequences, unicode), fall back to false for safety
	return "false"
}

func init() {
	RegisterTransformer(&InOperatorTransform{})
}
