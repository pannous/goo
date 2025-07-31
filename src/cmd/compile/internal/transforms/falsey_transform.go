// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transforms

import (
	"cmd/compile/internal/syntax"
	"strings"
)

// FalseyTransform converts 'not' operator to truthiness evaluation
// Transforms: not x -> !truthy(x)
type FalseyTransform struct{}

// falseyVisitor implements the visitor pattern for falsey transformation
type falseyVisitor struct {
	transform *FalseyTransform
	ctx       *TransformContext
	changed   bool
}

func (t *FalseyTransform) Name() string {
	return "falsey_transform"
}

func (t *FalseyTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &falseyVisitor{transform: t, ctx: ctx}
	
	// Use the visitor pattern to walk all nodes
	syntax.Walk(file, visitor)
	
	return visitor.changed
}

// Visit implements syntax.Visitor
func (v *falseyVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}
	
	// Look for nodes that contain expressions we can replace
	switch n := node.(type) {
	case *syntax.ExprStmt:
		if newExpr := v.transformFalseyExpr(n.X); newExpr != nil {
			n.X = newExpr
			v.changed = true
		}
	case *syntax.AssignStmt:
		if newExpr := v.transformFalseyExpr(n.Rhs); newExpr != nil {
			n.Rhs = newExpr
			v.changed = true
		}
	case *syntax.VarDecl:
		if n.Values != nil {
			if newExpr := v.transformFalseyExpr(n.Values); newExpr != nil {
				n.Values = newExpr
				v.changed = true
			}
		}
	case *syntax.CallExpr:
		// Handle falsey expressions in function arguments
		for i, arg := range n.ArgList {
			if newExpr := v.transformFalseyExpr(arg); newExpr != nil {
				n.ArgList[i] = newExpr
				v.changed = true
			}
		}
	case *syntax.ReturnStmt:
		if n.Results != nil {
			if newExpr := v.transformFalseyExpr(n.Results); newExpr != nil {
				n.Results = newExpr
				v.changed = true
			}
		}
	case *syntax.CheckStmt:
		if n.Cond != nil {
			if newExpr := v.transformFalseyExpr(n.Cond); newExpr != nil {
				n.Cond = newExpr
				v.changed = true
			}
		}
	case *syntax.Operation:
		if newExpr := v.transformFalseyExpr(n.X); newExpr != nil {
			n.X = newExpr
			v.changed = true
		}
		if n.Y != nil {
			if newExpr := v.transformFalseyExpr(n.Y); newExpr != nil {
				n.Y = newExpr
				v.changed = true
			}
		}
	case *syntax.IfStmt:
		if n.Cond != nil {
			if newExpr := v.transformFalseyExpr(n.Cond); newExpr != nil {
				n.Cond = newExpr
				v.changed = true
			}
		}
	case *syntax.ForStmt:
		if n.Cond != nil {
			if newExpr := v.transformFalseyExpr(n.Cond); newExpr != nil {
				n.Cond = newExpr
				v.changed = true
			}
		}
	}
	
	// Continue visiting child nodes
	return v
}

func (v *falseyVisitor) transformFalseyExpr(expr syntax.Expr) syntax.Expr {
	// Look for pattern: not x
	if op, ok := expr.(*syntax.Operation); ok && op.Op == syntax.Not {
		// Transform "not x" to "!truthy(x)"
		return v.createNotTruthyCall(op.X)
	}
	
	return nil
}

func (v *falseyVisitor) createNotTruthyCall(expr syntax.Expr) syntax.Expr {
	// For non-boolean types, we need to convert to truthiness check
	// Transform "not x" to appropriate comparison based on type
	
	// For now, let's generate the pattern that works with existing truthiness:
	// Convert "not x" to "x == zero_value_of_type"
	
	// This is a simplified approach - we'll create comparisons for common types
	switch e := expr.(type) {
	case *syntax.BasicLit:
		// For literals, we can determine the comparison directly
		switch e.Kind {
		case syntax.IntLit:
			// not 0 -> 0 == 0 (true), not 1 -> 1 == 0 (false)
			zero := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
			zero.SetPos(expr.Pos())
			eq := &syntax.Operation{Op: syntax.Eql, X: expr, Y: zero}
			eq.SetPos(expr.Pos())
			return eq
		case syntax.StringLit:
			// not "" -> "" == "" (true), not "x" -> "x" == "" (false)  
			empty := &syntax.BasicLit{Kind: syntax.StringLit, Value: `""`}
			empty.SetPos(expr.Pos())
			eq := &syntax.Operation{Op: syntax.Eql, X: expr, Y: empty}
			eq.SetPos(expr.Pos())
			return eq
		case syntax.FloatLit:
			// not 0.0 -> 0.0 == 0.0 (true), not 3.14 -> 3.14 == 0.0 (false)
			zero := &syntax.BasicLit{Kind: syntax.FloatLit, Value: "0.0"}
			zero.SetPos(expr.Pos())
			eq := &syntax.Operation{Op: syntax.Eql, X: expr, Y: zero}
			eq.SetPos(expr.Pos())
			return eq
		}
	case *syntax.Name:
		// For variables, use type information to generate appropriate zero comparison
		if varType, exists := v.ctx.Types[e.Value]; exists {
			switch varType {
			case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
				zero := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
				zero.SetPos(expr.Pos())
				eq := &syntax.Operation{Op: syntax.Eql, X: expr, Y: zero}
				eq.SetPos(expr.Pos())
				return eq
			case "string":
				empty := &syntax.BasicLit{Kind: syntax.StringLit, Value: `""`}
				empty.SetPos(expr.Pos())
				eq := &syntax.Operation{Op: syntax.Eql, X: expr, Y: empty}
				eq.SetPos(expr.Pos())
				return eq
			case "float32", "float64":
				zero := &syntax.BasicLit{Kind: syntax.FloatLit, Value: "0.0"}
				zero.SetPos(expr.Pos())
				eq := &syntax.Operation{Op: syntax.Eql, X: expr, Y: zero}
				eq.SetPos(expr.Pos())
				return eq
			case "bool":
				// For booleans, not x should be !x
				notExpr := &syntax.Operation{Op: syntax.Not, X: expr}
				notExpr.SetPos(expr.Pos())
				return notExpr
			}
			// Handle slice types
			if strings.HasPrefix(varType, "[]") {
				// For slices, not slice means len(slice) == 0
				lenCall := &syntax.CallExpr{
					Fun:     &syntax.Name{Value: "len"},
					ArgList: []syntax.Expr{expr},
				}
				lenCall.SetPos(expr.Pos())
				zero := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
				zero.SetPos(expr.Pos())
				eq := &syntax.Operation{Op: syntax.Eql, X: lenCall, Y: zero}
				eq.SetPos(expr.Pos())
				return eq
			}
			// Handle map types
			if strings.HasPrefix(varType, "map[") {
				// For maps: not map -> map == nil
				// This handles the nil case, but misses empty vs filled distinction
				// Runtime truthiness would be better but requires different approach
				nilName := &syntax.Name{Value: "nil"}
				nilName.SetPos(expr.Pos())
				eq := &syntax.Operation{Op: syntax.Eql, X: expr, Y: nilName}
				eq.SetPos(expr.Pos())
				return eq
			}
			// Handle pointer types  
			if strings.HasPrefix(varType, "*") {
				// For pointers: not ptr -> ptr == nil
				nilName := &syntax.Name{Value: "nil"}
				nilName.SetPos(expr.Pos())
				eq := &syntax.Operation{Op: syntax.Eql, X: expr, Y: nilName}
				eq.SetPos(expr.Pos())
				return eq
			}
			// Handle channel types
			if strings.HasPrefix(varType, "chan ") {
				// For channels: not chan -> chan == nil
				nilName := &syntax.Name{Value: "nil"}
				nilName.SetPos(expr.Pos())
				eq := &syntax.Operation{Op: syntax.Eql, X: expr, Y: nilName}
				eq.SetPos(expr.Pos())
				return eq
			}
		}
	case *syntax.CompositeLit:
		// Handle slice and map literals
		if _, ok := e.Type.(*syntax.SliceType); ok {
			// For slice literals like []int{}, []int{1,2}
			// not []int{} -> len([]int{}) == 0 -> true
			// not []int{1,2} -> len([]int{1,2}) == 0 -> false
			lenCall := &syntax.CallExpr{
				Fun:     &syntax.Name{Value: "len"},
				ArgList: []syntax.Expr{expr},
			}
			lenCall.SetPos(expr.Pos())
			zero := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
			zero.SetPos(expr.Pos())
			eq := &syntax.Operation{Op: syntax.Eql, X: lenCall, Y: zero}
			eq.SetPos(expr.Pos())
			return eq
		}
	}
	
	// Fallback: for boolean expressions, use regular !
	notExpr := &syntax.Operation{
		Op: syntax.Not,
		X:  expr,
	}
	notExpr.SetPos(expr.Pos())
	return notExpr
}

func init() {
	RegisterTransformer(&FalseyTransform{})
}