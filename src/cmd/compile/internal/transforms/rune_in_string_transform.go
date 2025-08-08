// Copyright 2025 The Goo Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// RuneInStringTransform converts rune in string to string(rune) in string
type RuneInStringTransform struct{}

func (t *RuneInStringTransform) Name() string {
	return "rune_in_string_transform"
}

func (t *RuneInStringTransform) Priority() int {
	return 999 // Disable for now - run after everything
}

// NodeTransformer interface implementation
func (t *RuneInStringTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
	// Handle direct In operations
	if op, ok := node.(*syntax.Operation); ok {
		return op.Op == syntax.In && t.isRuneInString(op)
	}

	// Handle check statements that contain In operations
	if checkStmt, ok := node.(*syntax.CheckStmt); ok {
		if op, ok := checkStmt.Cond.(*syntax.Operation); ok {
			return op.Op == syntax.In && t.isRuneInString(op)
		}
	}

	return false
}

func (t *RuneInStringTransform) TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node {
	// Handle direct In operations
	if op, ok := node.(*syntax.Operation); ok && op.Op == syntax.In {
		if t.isRuneInString(op) {
			return t.convertRuneToString(op)
		}
	}

	// Handle check statements that contain In operations
	if checkStmt, ok := node.(*syntax.CheckStmt); ok {
		if op, ok := checkStmt.Cond.(*syntax.Operation); ok && op.Op == syntax.In {
			if t.isRuneInString(op) {
				// Create new check statement with converted operation
				newCheckStmt := &syntax.CheckStmt{
					Cond:     t.convertRuneToString(op),
					OrigText: checkStmt.OrigText, // Preserve original text
				}
				newCheckStmt.SetPos(checkStmt.Pos())
				return newCheckStmt
			}
		}
	}

	return nil
}

func (t *RuneInStringTransform) PostProcess(file *syntax.File, ctx *TransformContext) bool {
	return false
}

// Legacy Transform method for backward compatibility
func (t *RuneInStringTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	return false
}

// isRuneInString checks if this is a rune literal in string operation
func (t *RuneInStringTransform) isRuneInString(op *syntax.Operation) bool {
	// Check if left operand is a rune literal
	if basic, ok := op.X.(*syntax.BasicLit); ok {
		if basic.Kind == syntax.RuneLit {
			// Always convert rune literals in 'in' operations
			return true
		}
	}
	return false
}

// convertRuneToString converts 'r' in "string" to string('r') in "string"
func (t *RuneInStringTransform) convertRuneToString(op *syntax.Operation) syntax.Expr {
	pos := op.Pos()

	// Ensure the original operands have position information
	if op.X != nil && op.X.Pos().IsKnown() {
		pos = op.X.Pos()
	}

	// Create string type name with position
	stringName := &syntax.Name{Value: "string"}
	stringName.SetPos(pos)

	// Create string conversion call: string(op.X)
	// Make sure op.X has position info
	if op.X.Pos().IsKnown() == false && pos.IsKnown() {
		// This shouldn't happen, but just in case
		if basicLit, ok := op.X.(*syntax.BasicLit); ok {
			basicLit.SetPos(pos)
		}
	}

	stringConversion := &syntax.CallExpr{
		Fun:     stringName,
		ArgList: []syntax.Expr{op.X},
	}
	stringConversion.SetPos(pos)

	// Create new operation: string(op.X) in op.Y
	newOp := &syntax.Operation{
		Op: syntax.In,
		X:  stringConversion,
		Y:  op.Y,
	}
	newOp.SetPos(pos)

	return newOp
}

func init() {
	RegisterTransformer(&RuneInStringTransform{})
}
