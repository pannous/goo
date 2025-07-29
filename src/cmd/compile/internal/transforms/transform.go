// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package transforms

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"os"
	"strings"
)

// Transformer represents a syntax tree transformation stage.
// It operates on syntax.File nodes before they are converted to unified IR.
// TransformContext provides shared context to all transformers.
type TransformContext struct {
	Types map[string]string // name -> inferred type, e.g., "int", "string"
}

type Transformer interface {
	// Transform modifies the syntax tree in place and returns whether any changes were made.
	Transform(file *syntax.File, ctx *TransformContext) bool

	// Name returns a human-readable name for this transformer.
	Name() string
}

// ApplyTransformations runs all registered transformers on the syntax tree.
// called bycmd/compile/internal/noder/unified.go
func ApplyTransformations(files []*syntax.File) {
	for _, file := range files {
		var userTransformers = os.Getenv("GOO_USE_TRANSFORMERS") == "1"
		if strings.Contains(file.Path.Filename(), ".goo") {
			userTransformers = true // Force transformers for .goo files
		}
		if !userTransformers {
			continue
		}
		if file.PkgName.Value != "main" {
			continue /* Skip non-main packages */
		}
		if isToolchainPath(file) {
			continue /* Skip toolchain packages */
		}

		ctx := &TransformContext{Types: make(map[string]string)}
		collectTypes(file, ctx)
		for _, transformer := range TransformRegistry {
			transformer.Transform(file, ctx)
			{
				fmt.Printf("Applied transformer: %s to package: %s\n", transformer.Name(), file.PkgName.Value)
			}
		}
	}
}

func isToolchainPath(file *syntax.File) bool {
	toolchainPaths := []string{
		"bufio", "bytes", "cmp", "command-line-arguments", "context", "crypto", "embed", "encoding", "errors", "expvar", "flag", "fmt", "hash", "html", "image", "io", "iter", "log", "maps", "math", "mime", "net", "os", "path", "plugin", "reflect", "regexp", "runtime", "slices", "sort", "strconv", "strings", "structs", "sync", "syscall", "testing", "time", "unicode", "unique", "weak",
	}
	for _, path := range toolchainPaths {
		if strings.HasPrefix(file.PkgName.Value, path) {
			return true // This is a toolchain package, skip it
		}
	}
	return false
}

// SyntaxWalker provides a framework for walking and transforming syntax trees.
type SyntaxWalker struct {
	// PreOrder and PostOrder callbacks for each node type
	VisitExpr func(syntax.Expr) syntax.Expr
	VisitStmt func(syntax.Stmt) syntax.Stmt
	VisitDecl func(syntax.Decl) syntax.Decl
}

// WalkFile traverses a syntax file and applies transformations.
func (w *SyntaxWalker) WalkFile(file *syntax.File) {
	if file == nil {
		return
	}

	fmt.Println("Walking file for package:", file.PkgName.Value)

	for i, decl := range file.DeclList {
		if w.VisitDecl != nil {
			if newDecl := w.VisitDecl(decl); newDecl != nil {
				file.DeclList[i] = newDecl
			}
		}
		w.walkDecl(file.DeclList[i])
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

	e := expr
	switch e := e.(type) {
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
	case *syntax.LambdaExpr:
		w.walkExpr(e.Body)
	case *syntax.AsCastExpr:
		w.walkExpr(e.X)
		w.walkExpr(e.Type)
	}
}

// TransformRegistry holds all registered transformers.
var TransformRegistry []Transformer

// RegisterTransformer adds a transformer to the global registry.
func RegisterTransformer(t Transformer) {
	if t.Name() != "try_catch_transform" && t.Name() != "map_dot_transform" {
		return
	} // DEBUG - only allow specific transformers
	// Check if the transformer is already registered
	for _, existing := range TransformRegistry {
		if existing.Name() == t.Name() {
			return // Already registered, skip
		}
	}
	TransformRegistry = append(TransformRegistry, t)
}

// collectTypes walks the syntax tree and populates ctx.Types with variable names and their inferred types.
func collectTypes(file *syntax.File, ctx *TransformContext) {
	for _, decl := range file.DeclList {
		if f, ok := decl.(*syntax.FuncDecl); ok {
			collectFromStmt(f.Body, ctx)
		}
		// Also collect from top-level variable declarations
		if v, ok := decl.(*syntax.VarDecl); ok {
			collectFromVarDecl(v, ctx)
		}
	}
}

// collectFromVarDecl extracts type information from variable declarations
func collectFromVarDecl(varDecl *syntax.VarDecl, ctx *TransformContext) {
	if varDecl.Type != nil {
		// Handle explicit type declarations like: var f2 float64
		if typeName, ok := varDecl.Type.(*syntax.Name); ok {
			for _, name := range varDecl.NameList {
				ctx.Types[name.Value] = typeName.Value
			}
		}
	}
}

func collectFromStmt(stmt syntax.Stmt, ctx *TransformContext) {
	switch s := stmt.(type) {
	case *syntax.BlockStmt:
		for _, sub := range s.List {
			collectFromStmt(sub, ctx)
		}
	case *syntax.DeclStmt:
		for _, decl := range s.DeclList {
			if varDecl, ok := decl.(*syntax.VarDecl); ok {
				collectFromVarDecl(varDecl, ctx)
			}
		}
	case *syntax.AssignStmt:
		var lhsElems []syntax.Expr
		var rhsElems []syntax.Expr

		if lhsList, ok := s.Lhs.(*syntax.ListExpr); ok {
			lhsElems = lhsList.ElemList
		} else {
			lhsElems = []syntax.Expr{s.Lhs}
		}

		if rhsList, ok := s.Rhs.(*syntax.ListExpr); ok {
			rhsElems = rhsList.ElemList
		} else {
			rhsElems = []syntax.Expr{s.Rhs}
		}

		if len(lhsElems) != len(rhsElems) {
			return
		}

		for i := range lhsElems {
			lhs, ok1 := lhsElems[i].(*syntax.Name)
			if !ok1 {
				continue
			}

			// Handle literal assignments
			if rhsLit, ok2 := rhsElems[i].(*syntax.BasicLit); ok2 {
				switch rhsLit.Kind {
				case syntax.IntLit:
					ctx.Types[lhs.Value] = "int"
				case syntax.StringLit:
					ctx.Types[lhs.Value] = "string"
				case syntax.FloatLit:
					ctx.Types[lhs.Value] = "float64"
				case syntax.RuneLit:
					ctx.Types[lhs.Value] = "rune"
				case syntax.ImagLit:
					ctx.Types[lhs.Value] = "complex128"
				default:
					ctx.Types[lhs.Value] = "unknown"
				}
			}

			// Handle boolean names (true/false)
			if rhsName, ok2 := rhsElems[i].(*syntax.Name); ok2 {
				if rhsName.Value == "true" || rhsName.Value == "false" {
					ctx.Types[lhs.Value] = "bool"
				}
			}

			// Handle composite literals (slices, arrays, maps)
			if rhsComp, ok2 := rhsElems[i].(*syntax.CompositeLit); ok2 {
				if sliceType, ok3 := rhsComp.Type.(*syntax.SliceType); ok3 {
					// This is a slice type like []int
					if elemType, ok4 := sliceType.Elem.(*syntax.Name); ok4 {
						ctx.Types[lhs.Value] = "[]" + elemType.Value
					} else {
						ctx.Types[lhs.Value] = "slice"
					}
				} else if arrayType, ok3 := rhsComp.Type.(*syntax.ArrayType); ok3 {
					if arrayType.Len == nil {
						// This is a slice type like []int
						if elemType, ok4 := arrayType.Elem.(*syntax.Name); ok4 {
							ctx.Types[lhs.Value] = "[]" + elemType.Value
						} else {
							ctx.Types[lhs.Value] = "slice"
						}
					} else {
						// This is an array type like [3]int
						if elemType, ok4 := arrayType.Elem.(*syntax.Name); ok4 {
							ctx.Types[lhs.Value] = "[]" + elemType.Value // treat as slice for method purposes
						} else {
							ctx.Types[lhs.Value] = "array"
						}
					}
				} else if mapType, ok3 := rhsComp.Type.(*syntax.MapType); ok3 {
					// This is a map type like map[string]int
					keyType := "unknown"
					valueType := "unknown"
					if keyName, ok4 := mapType.Key.(*syntax.Name); ok4 {
						keyType = keyName.Value
					}
					if valueName, ok4 := mapType.Value.(*syntax.Name); ok4 {
						valueType = valueName.Value
					}
					ctx.Types[lhs.Value] = "map[" + keyType + "]" + valueType
				}
			}
		}
	}
}
