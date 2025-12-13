// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
	"strings"
)

// IfTruthinessTransform converts non-boolean if/check conditions to boolean expressions
// This runs BEFORE check_transform to ensure all conditions are boolean
type IfTruthinessTransform struct{}

func (t *IfTruthinessTransform) Name() string {
	return "if_truthiness_transform"
}

func (t *IfTruthinessTransform) Priority() int {
	return 140 // Run BEFORE check_transform (150) to fix conditions first
}

func (t *IfTruthinessTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	changed := false

	walker := &SyntaxWalker{
		VisitStmt: func(stmt syntax.Stmt) syntax.Stmt {
			switch s := stmt.(type) {
			case *syntax.IfStmt:
				if s.Cond != nil {
					if newCond := t.makeTruthy(s.Cond, ctx); newCond != s.Cond {
						s.Cond = newCond
						changed = true
					}
				}
			case *syntax.CheckStmt:
				if s.Cond != nil {
					if newCond := t.makeTruthy(s.Cond, ctx); newCond != s.Cond {
						s.Cond = newCond
						changed = true
					}
				}
			case *syntax.ForStmt:
				if s.Cond != nil {
					if newCond := t.makeTruthy(s.Cond, ctx); newCond != s.Cond {
						s.Cond = newCond
						changed = true
					}
				}
			}
			return stmt
		},
	}

	walker.WalkFile(file)

	// Also handle top-level statements
	if len(file.TopLevelStmts) > 0 {
		for i, stmt := range file.TopLevelStmts {
			file.TopLevelStmts[i] = t.transformTopLevelStmt(stmt, &changed, ctx)
		}
	}

	return changed
}

func (t *IfTruthinessTransform) transformTopLevelStmt(stmt syntax.Stmt, changed *bool, ctx *TransformContext) syntax.Stmt {
	walker := &SyntaxWalker{
		VisitStmt: func(stmt syntax.Stmt) syntax.Stmt {
			switch s := stmt.(type) {
			case *syntax.IfStmt:
				if s.Cond != nil {
					if newCond := t.makeTruthy(s.Cond, ctx); newCond != s.Cond {
						s.Cond = newCond
						*changed = true
					}
				}
			case *syntax.CheckStmt:
				if s.Cond != nil {
					if newCond := t.makeTruthy(s.Cond, ctx); newCond != s.Cond {
						s.Cond = newCond
						*changed = true
					}
				}
			case *syntax.ForStmt:
				if s.Cond != nil {
					if newCond := t.makeTruthy(s.Cond, ctx); newCond != s.Cond {
						s.Cond = newCond
						*changed = true
					}
				}
			}
			return stmt
		},
	}
	walker.walkStmt(stmt)
	return stmt
}

// makeTruthy converts non-boolean expressions to boolean checks
func (t *IfTruthinessTransform) makeTruthy(expr syntax.Expr, ctx *TransformContext) syntax.Expr {
	pos := expr.Pos()

	// Skip if already boolean (Operation with ==, !=, <, >, etc.)
	if op, ok := expr.(*syntax.Operation); ok {
		switch op.Op {
		case syntax.Eql, syntax.Neq, syntax.Lss, syntax.Leq, syntax.Gtr, syntax.Geq,
			syntax.AndAnd, syntax.OrOr, syntax.Not:
			return expr // Already boolean
		}
	}

	// Check for boolean names (true/false)
	if name, ok := expr.(*syntax.Name); ok {
		if name.Value == "true" || name.Value == "false" {
			return expr // Already boolean
		}
	}

	// Check for literals
	if lit, ok := expr.(*syntax.BasicLit); ok {
		switch lit.Kind {
		case syntax.IntLit, syntax.FloatLit:
			// if 0 -> if 0 != 0, if 42 -> if 42 != 0
			zero := &syntax.BasicLit{Kind: lit.Kind, Value: "0"}
			if lit.Kind == syntax.FloatLit {
				zero.Value = "0.0"
			}
			zero.SetPos(pos)
			neq := &syntax.Operation{Op: syntax.Neq, X: expr, Y: zero}
			neq.SetPos(pos)
			return neq
		case syntax.StringLit:
			// if "" -> if "" != "", if "hello" -> if "hello" != ""
			empty := &syntax.BasicLit{Kind: syntax.StringLit, Value: `""`}
			empty.SetPos(pos)
			neq := &syntax.Operation{Op: syntax.Neq, X: expr, Y: empty}
			neq.SetPos(pos)
			return neq
		}
	}

	// Check for variables with type information
	if name, ok := expr.(*syntax.Name); ok {
		if varType, exists := ctx.Types[name.Value]; exists {
			return t.makeTruthyForType(expr, varType, pos)
		}
	}

	// Check for composite literals
	if comp, ok := expr.(*syntax.CompositeLit); ok {
		// Slices: if slice -> if slice != nil
		if _, ok := comp.Type.(*syntax.SliceType); ok {
			nilName := &syntax.Name{Value: "nil"}
			nilName.SetPos(pos)
			neq := &syntax.Operation{Op: syntax.Neq, X: expr, Y: nilName}
			neq.SetPos(pos)
			return neq
		}
	}

	// Default: assume it should be != nil
	nilName := &syntax.Name{Value: "nil"}
	nilName.SetPos(pos)
	neq := &syntax.Operation{Op: syntax.Neq, X: expr, Y: nilName}
	neq.SetPos(pos)
	return neq
}

// makeTruthyForType creates appropriate truthiness check based on type
func (t *IfTruthinessTransform) makeTruthyForType(expr syntax.Expr, varType string, pos syntax.Pos) syntax.Expr {
	switch varType {
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "rune":
		// if x -> if x != 0
		zero := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
		zero.SetPos(pos)
		neq := &syntax.Operation{Op: syntax.Neq, X: expr, Y: zero}
		neq.SetPos(pos)
		return neq
	case "float32", "float64":
		// if x -> if x != 0.0
		zero := &syntax.BasicLit{Kind: syntax.FloatLit, Value: "0.0"}
		zero.SetPos(pos)
		neq := &syntax.Operation{Op: syntax.Neq, X: expr, Y: zero}
		neq.SetPos(pos)
		return neq
	case "string":
		// if x -> if x != ""
		empty := &syntax.BasicLit{Kind: syntax.StringLit, Value: `""`}
		empty.SetPos(pos)
		neq := &syntax.Operation{Op: syntax.Neq, X: expr, Y: empty}
		neq.SetPos(pos)
		return neq
	case "bool":
		// Already boolean, no change needed
		return expr
	default:
		// For slices, maps, pointers, channels: if x -> if x != nil
		if strings.HasPrefix(varType, "[]") || strings.HasPrefix(varType, "map[") ||
			strings.HasPrefix(varType, "*") || strings.HasPrefix(varType, "chan ") {
			nilName := &syntax.Name{Value: "nil"}
			nilName.SetPos(pos)
			neq := &syntax.Operation{Op: syntax.Neq, X: expr, Y: nilName}
			neq.SetPos(pos)
			return neq
		}
	}

	// Default: no transformation
	return expr
}

func init() {
	RegisterTransformer(&IfTruthinessTransform{})
}
