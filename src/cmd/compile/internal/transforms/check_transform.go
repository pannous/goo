// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// CheckTransform converts check statements to if+panic statements
// Transforms: check condition -> if !(condition) { panic("Check failed: condition") }
//
// ⚠️  KNOWN ISSUE: Fails in explicit func main() with "invalid syntax tree: invalid statement"
// Works in simplified syntax (top-level statements) but not function bodies.
// Attempted fixes: new() vs literal, direct traversal vs Walk, block modifications.
// Root cause unclear - types2 sees CheckStmt despite transformation.
// Workaround: Use regular if statements for tests.
type CheckTransform struct{}

// checkVisitor implements the visitor pattern for check transformation
type checkVisitor struct {
	transform *CheckTransform
	ctx       *TransformContext
	changed   bool
}

func (t *CheckTransform) Name() string {
	return "check_transform"
}

func (t *CheckTransform) Priority() int {
	return 150 // Run after as_cast_transform (100) to ensure as expressions are transformed first
}

func (t *CheckTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	changed := false

	// Handle top-level statements (simplified syntax)
	if len(file.TopLevelStmts) > 0 {
		changed = t.transformTopLevelStmts(file, ctx) || changed
	}

	// Handle function bodies (explicit func main)
	for _, decl := range file.DeclList {
		if funcDecl, ok := decl.(*syntax.FuncDecl); ok && funcDecl.Body != nil {
			changed = t.transformBlock(funcDecl.Body, ctx) || changed
		}
	}

	return changed
}

// Visit implements syntax.Visitor
func (v *checkVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}

	// Look for blocks containing CheckStmt
	if block, ok := node.(*syntax.BlockStmt); ok {
		if v.transform.transformBlock(block, v.ctx) {
			v.changed = true
			// Return v to continue - block.List already modified in place
		}
	}

	// Look for top-level statements in file
	if file, ok := node.(*syntax.File); ok {
		if v.transform.transformTopLevelStmts(file, v.ctx) {
			v.changed = true
			// Return v to continue - TopLevelStmts already modified
		}
	}

	// Continue visiting child nodes
	return v
}

// transformBlock replaces CheckStmt in a block with if+panic
func (t *CheckTransform) transformBlock(block *syntax.BlockStmt, ctx *TransformContext) bool {
	changed := false
	newList := make([]syntax.Stmt, 0, len(block.List))

	for _, stmt := range block.List {
		if checkStmt, ok := stmt.(*syntax.CheckStmt); ok {
			// Transform check to if+panic
			ifStmt := t.convertCheckToIf(checkStmt)
			newList = append(newList, ifStmt)
			changed = true
		} else {
			newList = append(newList, stmt)
		}
	}

	if changed {
		block.List = newList
	}

	return changed
}

// transformTopLevelStmts handles CheckStmt in file.TopLevelStmts
func (t *CheckTransform) transformTopLevelStmts(file *syntax.File, ctx *TransformContext) bool {
	if len(file.TopLevelStmts) == 0 {
		return false
	}

	changed := false
	newList := make([]syntax.Stmt, 0, len(file.TopLevelStmts))

	for _, stmt := range file.TopLevelStmts {
		if checkStmt, ok := stmt.(*syntax.CheckStmt); ok {
			// Transform check to if+panic
			ifStmt := t.convertCheckToIf(checkStmt)
			newList = append(newList, ifStmt)
			changed = true
		} else {
			newList = append(newList, stmt)
		}
	}

	if changed {
		file.TopLevelStmts = newList
	}

	return changed
}

// convertCheckToIf converts a CheckStmt to an if+panic statement
// check condition -> if !(condition) { panic("Check failed: condition") }
func (t *CheckTransform) convertCheckToIf(checkStmt *syntax.CheckStmt) *syntax.IfStmt {
	pos := checkStmt.Pos()

	// Create the negated condition: !(condition)
	negatedCond := &syntax.Operation{
		Op: syntax.Not,
		X:  checkStmt.Cond,
	}
	negatedCond.SetPos(pos)

	// Create the panic message string literal
	// Use backticks for raw string to avoid escaping issues
	var panicMsg string
	if checkStmt.OrigText != "" {
		panicMsg = "`Check failed: " + checkStmt.OrigText + "`"
	} else {
		panicMsg = "`Check failed`"
	}

	panicMsgLit := &syntax.BasicLit{
		Value: panicMsg,
		Kind:  syntax.StringLit,
	}
	panicMsgLit.SetPos(pos)

	// Create panic function name
	panicName := &syntax.Name{Value: "panic"}
	panicName.SetPos(pos)

	// Create panic call: panic("Check failed: ...")
	panicCall := &syntax.CallExpr{
		Fun:     panicName,
		ArgList: []syntax.Expr{panicMsgLit},
	}
	panicCall.SetPos(pos)

	// Create expression statement for panic
	panicStmt := &syntax.ExprStmt{
		X: panicCall,
	}
	panicStmt.SetPos(pos)

	// Create the if body (use new() like parser does)
	ifBody := new(syntax.BlockStmt)
	ifBody.List = []syntax.Stmt{panicStmt}
	ifBody.SetPos(pos)

	// Create the if statement (use new() like parser does)
	ifStmt := new(syntax.IfStmt)
	ifStmt.Cond = negatedCond
	ifStmt.Then = ifBody
	ifStmt.SetPos(pos)

	return ifStmt
}

func init() {
	RegisterTransformer(&CheckTransform{})
}
