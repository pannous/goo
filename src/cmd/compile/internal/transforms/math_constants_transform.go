// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// MathConstantsTransform replaces Unicode math constants with their values
// Transforms: π -> math.Pi, τ -> 2*math.Pi
type MathConstantsTransform struct{}

func (t *MathConstantsTransform) Name() string {
	return "math_constants_transform"
}

func (t *MathConstantsTransform) Priority() int {
	return 50 // Run early, before most transforms
}

func (t *MathConstantsTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	changed := false

	walker := &SyntaxWalker{
		VisitExpr: func(expr syntax.Expr) syntax.Expr {
			if name, ok := expr.(*syntax.Name); ok {
				switch name.Value {
				case "π":
					// Replace π with math.Pi
					changed = true
					RequestMathImport()
					return t.createMathPi(name.Pos())
				case "τ":
					// Replace τ with 2*math.Pi
					changed = true
					RequestMathImport()
					return t.createTau(name.Pos())
				}
			}
			return expr
		},
	}

	walker.WalkFile(file)

	// Also handle top-level statements
	if len(file.TopLevelStmts) > 0 {
		for i, stmt := range file.TopLevelStmts {
			file.TopLevelStmts[i] = t.transformStmt(stmt, &changed)
		}
	}

	return changed
}

func (t *MathConstantsTransform) transformStmt(stmt syntax.Stmt, changed *bool) syntax.Stmt {
	walker := &SyntaxWalker{
		VisitExpr: func(expr syntax.Expr) syntax.Expr {
			if name, ok := expr.(*syntax.Name); ok {
				switch name.Value {
				case "π":
					*changed = true
					RequestMathImport()
					return t.createMathPi(name.Pos())
				case "τ":
					*changed = true
					RequestMathImport()
					return t.createTau(name.Pos())
				}
			}
			return expr
		},
	}
	walker.walkStmt(stmt)
	return stmt
}

// createMathPi creates math.Pi reference
func (t *MathConstantsTransform) createMathPi(pos syntax.Pos) syntax.Expr {
	mathName := &syntax.Name{Value: "math"}
	mathName.SetPos(pos)

	piName := &syntax.Name{Value: "Pi"}
	piName.SetPos(pos)

	sel := &syntax.SelectorExpr{
		X:   mathName,
		Sel: piName,
	}
	sel.SetPos(pos)
	return sel
}

// createTau creates 2*math.Pi
func (t *MathConstantsTransform) createTau(pos syntax.Pos) syntax.Expr {
	// Create 2
	two := &syntax.BasicLit{
		Kind:  syntax.IntLit,
		Value: "2",
	}
	two.SetPos(pos)

	// Create math.Pi
	mathPi := t.createMathPi(pos)

	// Create 2 * math.Pi
	mul := &syntax.Operation{
		Op: syntax.Mul,
		X:  two,
		Y:  mathPi,
	}
	mul.SetPos(pos)
	return mul
}

func init() {
	RegisterTransformer(&MathConstantsTransform{})
}
