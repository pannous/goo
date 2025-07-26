// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transforms

import (
	"cmd/compile/internal/syntax"
)

// Transformer represents a syntax tree transformation stage.
// It operates on syntax.File nodes before they are converted to unified IR.
type Transformer interface {
	// Transform modifies the syntax tree in place and returns whether any changes were made.
	Transform(file *syntax.File) bool
	
	// Name returns a human-readable name for this transformer.
	Name() string
}

// TransformRegistry holds all registered transformers.
var TransformRegistry []Transformer

// RegisterTransformer adds a transformer to the global registry.
func RegisterTransformer(t Transformer) {
	TransformRegistry = append(TransformRegistry, t)
}

// ApplyTransformations runs all registered transformers on the syntax tree.
func ApplyTransformations(files []*syntax.File) {
	for _, transformer := range TransformRegistry {
		for _, file := range files {
			if transformer.Transform(file) {
				// Optional: add debug logging here
			}
		}
	}
}

// SyntaxWalker provides a framework for walking and transforming syntax trees.
type SyntaxWalker struct {
	// PreOrder and PostOrder callbacks for each node type
	VisitExpr  func(syntax.Expr) syntax.Expr
	VisitStmt  func(syntax.Stmt) syntax.Stmt
	VisitDecl  func(syntax.Decl) syntax.Decl
}

// WalkFile traverses a syntax file and applies transformations.
func (w *SyntaxWalker) WalkFile(file *syntax.File) {
	if file == nil {
		return
	}
	
	for i, decl := range file.DeclList {
		if w.VisitDecl != nil {
			if newDecl := w.VisitDecl(decl); newDecl != nil {
				file.DeclList[i] = newDecl
			}
		}
		w.walkDecl(decl)
	}
}

func (w *SyntaxWalker) walkDecl(decl syntax.Decl) {
	if decl == nil {
		return
	}
	
	switch d := decl.(type) {
	case *syntax.FuncDecl:
		w.walkStmt(d.Body)
	case *syntax.VarDecl:
		if d.Values != nil {
			w.walkExpr(d.Values)
		}
	case *syntax.ConstDecl:
		if d.Values != nil {
			w.walkExpr(d.Values)
		}
	}
}

func (w *SyntaxWalker) walkStmt(stmt syntax.Stmt) {
	if stmt == nil {
		return
	}
	
	if w.VisitStmt != nil {
		if newStmt := w.VisitStmt(stmt); newStmt != nil {
			stmt = newStmt
		}
	}
	
	switch s := stmt.(type) {
	case *syntax.BlockStmt:
		for i, stmt := range s.List {
			if w.VisitStmt != nil {
				if newStmt := w.VisitStmt(stmt); newStmt != nil {
					s.List[i] = newStmt
				}
			}
			w.walkStmt(stmt)
		}
	case *syntax.ExprStmt:
		w.walkExpr(s.X)
	case *syntax.AssignStmt:
		w.walkExpr(s.Lhs)
		w.walkExpr(s.Rhs)
	case *syntax.DeclStmt:
		for _, decl := range s.DeclList {
			w.walkDecl(decl)
		}
	case *syntax.IfStmt:
		if s.Init != nil {
			w.walkStmt(s.Init)
		}
		w.walkExpr(s.Cond)
		w.walkStmt(s.Then)
		if s.Else != nil {
			w.walkStmt(s.Else)
		}
	case *syntax.ForStmt:
		if s.Init != nil {
			w.walkStmt(s.Init)
		}
		if s.Cond != nil {
			w.walkExpr(s.Cond)
		}
		if s.Post != nil {
			w.walkStmt(s.Post)
		}
		w.walkStmt(s.Body)
	case *syntax.ReturnStmt:
		if s.Results != nil {
			w.walkExpr(s.Results)
		}
	}
}

func (w *SyntaxWalker) walkExpr(expr syntax.Expr) {
	if expr == nil {
		return
	}
	
	if w.VisitExpr != nil {
		if newExpr := w.VisitExpr(expr); newExpr != nil {
			expr = newExpr
		}
	}
	
	switch e := expr.(type) {
	case *syntax.Operation:
		w.walkExpr(e.X)
		if e.Y != nil {
			w.walkExpr(e.Y)
		}
	case *syntax.CallExpr:
		w.walkExpr(e.Fun)
		if e.ArgList != nil {
			for _, arg := range e.ArgList {
				w.walkExpr(arg)
			}
		}
	case *syntax.SelectorExpr:
		w.walkExpr(e.X)
	case *syntax.IndexExpr:
		w.walkExpr(e.X)
		w.walkExpr(e.Index)
	case *syntax.ListExpr:
		for _, elem := range e.ElemList {
			w.walkExpr(elem)
		}
	}
}