// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package transforms

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"os"
	"sort"
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

	// Priority returns the execution priority (lower numbers run first).
	Priority() int
}

// NodeTransformer represents a transformer that operates on specific node patterns
type NodeTransformer interface {
	// CanHandle returns true if this transformer can handle the given node
	CanHandle(node syntax.Node, ctx *TransformContext) bool
	
	// TransformNode transforms the given node and returns the modified node or nil if no change
	TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node
	
	// PostProcess is called after all transformations to handle file-level changes like imports
	PostProcess(file *syntax.File, ctx *TransformContext) bool
	
	// Name returns a human-readable name for this transformer
	Name() string
	
	// Priority returns the execution priority (lower numbers run first)
	Priority() int
}

// CentralTransformVisitor implements the centralized pattern matching and transformation
type CentralTransformVisitor struct {
	context      *TransformContext
	transformers []Transformer
	changed      bool
	appliedTransformers map[string]Transformer  // Track which transformer instances were applied
}

// WalkFile is the main entry point that walks the file and applies transformations
func (v *CentralTransformVisitor) WalkFile(file *syntax.File) {
	if file == nil {
		return
	}

	// Initialize tracking
	v.appliedTransformers = make(map[string]Transformer)

	// Walk and transform nodes
	for i, decl := range file.DeclList {
		if newDecl := v.walkDecl(decl); newDecl != nil && newDecl != decl {
			file.DeclList[i] = newDecl
			v.changed = true
		}
	}
	
	// Post-process applied transformers (handle imports, etc.)
	for _, transformer := range v.appliedTransformers {
		if nodeTransformer, ok := transformer.(NodeTransformer); ok {
			if nodeTransformer.PostProcess(file, v.context) {
				v.changed = true
			}
		}
	}
}

func (v *CentralTransformVisitor) walkDecl(decl syntax.Decl) syntax.Decl {
	if decl == nil {
		return nil
	}

	// Apply transformers to this declaration (only for relevant leaf nodes)
	if newDecl := v.tryTransformLeafNode(decl); newDecl != nil {
		if transformedDecl, ok := newDecl.(syntax.Decl); ok {
			decl = transformedDecl
		}
	}

	// Walk children
	switch d := decl.(type) {
	case *syntax.FuncDecl:
		if newBody := v.walkStmt(d.Body); newBody != nil && newBody != d.Body {
			if blockStmt, ok := newBody.(*syntax.BlockStmt); ok {
				d.Body = blockStmt
				v.changed = true
			}
		}
	case *syntax.VarDecl:
		if d.Values != nil {
			if newValues := v.walkExpr(d.Values); newValues != nil && newValues != d.Values {
				d.Values = newValues
				v.changed = true
			}
		}
	case *syntax.ConstDecl:
		if d.Values != nil {
			if newValues := v.walkExpr(d.Values); newValues != nil && newValues != d.Values {
				d.Values = newValues
				v.changed = true
			}
		}
	}

	return decl
}

func (v *CentralTransformVisitor) walkStmt(stmt syntax.Stmt) syntax.Stmt {
	if stmt == nil {
		return nil
	}

	// Apply transformers to this statement (only for relevant leaf nodes)
	if newStmt := v.tryTransformLeafNode(stmt); newStmt != nil {
		if transformedStmt, ok := newStmt.(syntax.Stmt); ok {
			stmt = transformedStmt
		}
	}

	// Walk children
	switch s := stmt.(type) {
	case *syntax.BlockStmt:
		for i, childStmt := range s.List {
			if newChildStmt := v.walkStmt(childStmt); newChildStmt != nil && newChildStmt != childStmt {
				s.List[i] = newChildStmt
				v.changed = true
			}
		}
	case *syntax.ExprStmt:
		if newX := v.walkExpr(s.X); newX != nil && newX != s.X {
			s.X = newX
			v.changed = true
		}
	case *syntax.AssignStmt:
		if newLhs := v.walkExpr(s.Lhs); newLhs != nil && newLhs != s.Lhs {
			s.Lhs = newLhs
			v.changed = true
		}
		if newRhs := v.walkExpr(s.Rhs); newRhs != nil && newRhs != s.Rhs {
			s.Rhs = newRhs
			v.changed = true
		}
	case *syntax.DeclStmt:
		for i, childDecl := range s.DeclList {
			if newChildDecl := v.walkDecl(childDecl); newChildDecl != nil && newChildDecl != childDecl {
				s.DeclList[i] = newChildDecl
				v.changed = true
			}
		}
	case *syntax.IfStmt:
		if s.Init != nil {
			if newInit := v.walkStmt(s.Init); newInit != nil && newInit != s.Init {
				if simpleStmt, ok := newInit.(syntax.SimpleStmt); ok {
					s.Init = simpleStmt
					v.changed = true
				}
			}
		}
		if newCond := v.walkExpr(s.Cond); newCond != nil && newCond != s.Cond {
			s.Cond = newCond
			v.changed = true
		}
		if newThen := v.walkStmt(s.Then); newThen != nil && newThen != s.Then {
			if blockStmt, ok := newThen.(*syntax.BlockStmt); ok {
				s.Then = blockStmt
				v.changed = true
			}
		}
		if s.Else != nil {
			if newElse := v.walkStmt(s.Else); newElse != nil && newElse != s.Else {
				s.Else = newElse
				v.changed = true
			}
		}
	case *syntax.ForStmt:
		if s.Init != nil {
			if newInit := v.walkStmt(s.Init); newInit != nil && newInit != s.Init {
				if simpleStmt, ok := newInit.(syntax.SimpleStmt); ok {
					s.Init = simpleStmt
					v.changed = true
				}
			}
		}
		if s.Cond != nil {
			if newCond := v.walkExpr(s.Cond); newCond != nil && newCond != s.Cond {
				s.Cond = newCond
				v.changed = true
			}
		}
		if s.Post != nil {
			if newPost := v.walkStmt(s.Post); newPost != nil && newPost != s.Post {
				if simpleStmt, ok := newPost.(syntax.SimpleStmt); ok {
					s.Post = simpleStmt
					v.changed = true
				}
			}
		}
		if newBody := v.walkStmt(s.Body); newBody != nil && newBody != s.Body {
			if blockStmt, ok := newBody.(*syntax.BlockStmt); ok {
				s.Body = blockStmt
				v.changed = true
			}
		}
	case *syntax.ReturnStmt:
		if s.Results != nil {
			if newResults := v.walkExpr(s.Results); newResults != nil && newResults != s.Results {
				s.Results = newResults
				v.changed = true
			}
		}
	}

	return stmt
}

func (v *CentralTransformVisitor) walkExpr(expr syntax.Expr) syntax.Expr {
	if expr == nil {
		return nil
	}

	// Apply transformers to this expression (only for relevant leaf nodes)
	if newExpr := v.tryTransformLeafNode(expr); newExpr != nil {
		if transformedExpr, ok := newExpr.(syntax.Expr); ok {
			expr = transformedExpr
		}
	}

	// Walk children
	switch e := expr.(type) {
	case *syntax.Operation:
		if newX := v.walkExpr(e.X); newX != nil && newX != e.X {
			e.X = newX
			v.changed = true
		}
		if e.Y != nil {
			if newY := v.walkExpr(e.Y); newY != nil && newY != e.Y {
				e.Y = newY
				v.changed = true
			}
		}
	case *syntax.CallExpr:
		if newFun := v.walkExpr(e.Fun); newFun != nil && newFun != e.Fun {
			e.Fun = newFun
			v.changed = true
		}
		if e.ArgList != nil {
			for i, arg := range e.ArgList {
				if newArg := v.walkExpr(arg); newArg != nil && newArg != arg {
					e.ArgList[i] = newArg
					v.changed = true
				}
			}
		}
	case *syntax.SelectorExpr:
		if newX := v.walkExpr(e.X); newX != nil && newX != e.X {
			e.X = newX
			v.changed = true
		}
	case *syntax.IndexExpr:
		if newX := v.walkExpr(e.X); newX != nil && newX != e.X {
			e.X = newX
			v.changed = true
		}
		if newIndex := v.walkExpr(e.Index); newIndex != nil && newIndex != e.Index {
			e.Index = newIndex
			v.changed = true
		}
	case *syntax.ListExpr:
		for i, elem := range e.ElemList {
			if newElem := v.walkExpr(elem); newElem != nil && newElem != elem {
				e.ElemList[i] = newElem
				v.changed = true
			}
		}
	case *syntax.LambdaExpr:
		if newBody := v.walkExpr(e.Body); newBody != nil && newBody != e.Body {
			e.Body = newBody
			v.changed = true
		}
	case *syntax.AsCastExpr:
		if newX := v.walkExpr(e.X); newX != nil && newX != e.X {
			e.X = newX
			v.changed = true
		}
		if newType := v.walkExpr(e.Type); newType != nil && newType != e.Type {
			e.Type = newType
			v.changed = true
		}
	case *syntax.ParenExpr:
		// Handle parenthesized expressions like ("x" in "abc")
		if newX := v.walkExpr(e.X); newX != nil && newX != e.X {
			e.X = newX
			v.changed = true
		}
	}

	return expr
}

// tryTransformNode attempts to transform a node using all registered transformers
func (v *CentralTransformVisitor) tryTransformNode(node syntax.Node) syntax.Node {
	for _, transformer := range v.transformers {
		if nodeTransformer, ok := transformer.(NodeTransformer); ok {
			if nodeTransformer.CanHandle(node, v.context) {
				if newNode := nodeTransformer.TransformNode(node, v.context); newNode != nil {
					fmt.Printf("Node transformation applied by: %s\n", nodeTransformer.Name())
					v.appliedTransformers[nodeTransformer.Name()] = transformer
					return newNode
				}
			}
		}
	}
	return nil
}

// tryTransformLeafNode calls transformers on specific expression/operation nodes
func (v *CentralTransformVisitor) tryTransformLeafNode(node syntax.Node) syntax.Node {
	// Only call transformers on "leaf" nodes they actually care about
	switch node.(type) {
	case *syntax.Operation, *syntax.CallExpr, *syntax.CompositeLit, *syntax.FuncDecl, *syntax.AsCastExpr, *syntax.SelectorExpr, *syntax.TypeDecl, *syntax.ImportDecl, *syntax.LambdaExpr, *syntax.ReturnStmt, *syntax.TryStmt, *syntax.TryCatchStmt:
		// These are the actual nodes transformers want to handle
		return v.tryTransformNode(node)
	}
	return nil
}

// Legacy Visit method for compatibility (not used in new architecture)
func (v *CentralTransformVisitor) Visit(node syntax.Node) syntax.Visitor {
	// This is kept for compatibility but not used in the new architecture
	return v
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
		
		fmt.Printf("Transform execution order:\n")
		for i, transformer := range TransformRegistry {
			fmt.Printf("  %d. %s (priority %d)\n", i+1, transformer.Name(), transformer.Priority())
		}
		
		// Initialize import manager for this file
		GlobalImportManager = NewImportManager()
		
		// Use centralized visitor for NodeTransformers first
		centralVisitor := &CentralTransformVisitor{
			context: ctx,
			transformers: TransformRegistry,
			changed: false,
			appliedTransformers: make(map[string]Transformer),
		}
		
		centralVisitor.WalkFile(file)
		
		// Fall back to old interface for transformers that don't implement NodeTransformer
		for _, transformer := range TransformRegistry {
			if _, ok := transformer.(NodeTransformer); !ok {
				// Use old interface for backward compatibility
				if transformer.Transform(file, ctx) {
					fmt.Printf("Applied transformer: %s to package: %s\n", transformer.Name(), file.PkgName.Value)
					centralVisitor.changed = true
				}
			}
		}
		
		// Apply all requested imports centrally
		if GlobalImportManager.ApplyImports(file) {
			fmt.Printf("ImportManager: Applied imports to package: %s\n", file.PkgName.Value)
			centralVisitor.changed = true
		}
		
		if centralVisitor.changed {
			fmt.Printf("Applied transformations to package: %s\n", file.PkgName.Value)
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
	//if t.Name() != "try_catch_transform" && t.Name() != "map_dot_transform" {return} // DEBUG - only allow specific transformers
	// Check if the transformer is already registered
	for _, existing := range TransformRegistry {
		if existing.Name() == t.Name() {
			return // Already registered, skip
		}
	}
	TransformRegistry = append(TransformRegistry, t)
	
	// Sort by priority (lower numbers run first)
	sort.Slice(TransformRegistry, func(i, j int) bool {
		return TransformRegistry[i].Priority() < TransformRegistry[j].Priority()
	})
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
		// Handle slice type declarations like: var xs []User
		if arrayType, ok := varDecl.Type.(*syntax.ArrayType); ok {
			if elementType, ok := arrayType.Elem.(*syntax.Name); ok {
				sliceType := "[]" + elementType.Value
				for _, name := range varDecl.NameList {
					ctx.Types[name.Value] = sliceType
				}
			}
		}
	} else if varDecl.Values != nil {
		// Handle implicit type inference from values
		// Like: users := []User{{...}} or filtered := users.filter(...)
		if len(varDecl.NameList) == 1 {
			varName := varDecl.NameList[0].Value
			inferredType := inferTypeFromExpression(varDecl.Values, ctx)
			if inferredType != "" {
				ctx.Types[varName] = inferredType
			}
		}
	}
}

// inferTypeFromExpression attempts to infer the type of an expression
func inferTypeFromExpression(expr syntax.Expr, ctx *TransformContext) string {
	switch e := expr.(type) {
	case *syntax.CompositeLit:
		// Handle []User{{...}} literals
		if arrayType, ok := e.Type.(*syntax.ArrayType); ok {
			if elementType, ok := arrayType.Elem.(*syntax.Name); ok {
				return "[]" + elementType.Value
			}
		}
	case *syntax.CallExpr:
		// Handle method calls like users.filter(...)
		if selector, ok := e.Fun.(*syntax.SelectorExpr); ok {
			if receiverName, ok := selector.X.(*syntax.Name); ok {
				receiverType := ctx.Types[receiverName.Value]
				methodName := selector.Sel.Value
				
				// Infer return type based on method and receiver type
				return inferMethodReturnType(receiverType, methodName)
			}
		}
	}
	return ""
}

// inferMethodReturnType infers what type a method call returns
func inferMethodReturnType(receiverType, methodName string) string {
	if len(receiverType) >= 2 && receiverType[:2] == "[]" {
		elementType := receiverType[2:] // e.g., "User" from "[]User"
		
		switch methodName {
		case "filter", "where", "chose", "that", "which":
			// filter returns same type as input: []User -> []User
			return receiverType
		case "apply", "transform", "convert":
			// apply transforms elements, return []string for common cases
			// This could be enhanced to analyze the actual lambda
			return "[]string" // Most common case is converting to strings
		case "sort", "sortBy", "reverse":
			// sort returns same type as input: []User -> []User
			return receiverType
		case "first", "last", "head", "tail":
			// first returns element type: []User -> User
			return elementType
		}
	}
	return ""
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
			
			// Handle method call assignments like: filtered := users.filter(...)
			inferredType := inferTypeFromExpression(rhsElems[i], ctx)
			if inferredType != "" {
				ctx.Types[lhs.Value] = inferredType
			}
		}
	}
}
