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
	visitor := &checkVisitor{transform: t, ctx: ctx}

	// Walk all nodes looking for CheckStmt
	syntax.Walk(file, visitor)

	return visitor.changed
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
		}
	}

	// Look for top-level statements in file
	if file, ok := node.(*syntax.File); ok {
		if v.transform.transformTopLevelStmts(file, v.ctx) {
			v.changed = true
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

	// Create the if body
	ifBody := &syntax.BlockStmt{
		List: []syntax.Stmt{panicStmt},
	}
	ifBody.SetPos(pos)

	// Create the if statement
	ifStmt := &syntax.IfStmt{
		Cond: negatedCond,
		Then: ifBody,
	}
	ifStmt.SetPos(pos)

	return ifStmt
}

func init() {
	RegisterTransformer(&CheckTransform{})
}
