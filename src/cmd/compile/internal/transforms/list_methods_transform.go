// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// ListMethodsTransform handles automatic transformation of list/slice method calls
// to their corresponding Go standard library function calls.
type ListMethodsTransform struct{}


func (t *ListMethodsTransform) Name() string {
	return "list_methods_transform"
}

// transformListMethod transforms list method calls to standard library calls
func (t *ListMethodsTransform) transformListMethod(receiver syntax.Expr, methodName string, args []syntax.Expr) syntax.Expr {
	println("transformListMethod:", methodName)
	switch methodName {
	// Basic list info
	case "size", "length", "len":
		return t.createLenCall(receiver)
	case "count":
		if len(args) == 1 {
			return t.createCountCall(receiver, args[0])
		}
		return t.createLenCall(receiver) // count() with no args = length
	case "isEmpty":
		return t.createIsEmptyCall(receiver)

	// Element access
	case "first", "head":
		return t.createFirstCall(receiver)
	case "last", "tail":
		return t.createLastCall(receiver)
	case "get":
		if len(args) == 1 {
			return t.createGetCall(receiver, args[0])
		}

	// Search methods
	case "contains", "includes":
		if len(args) == 1 {
			return t.createContainsCall(receiver, args[0])
		}
	case "indexOf", "find":
		if len(args) == 1 {
			return t.createIndexCall(receiver, args[0])
		}
	case "lastIndexOf":
		if len(args) == 1 {
			return t.createLastIndexCall(receiver, args[0])
		}

	// Modification methods (return new slice)
	case "append", "add":
		if len(args) >= 1 {
			return t.createAppendCall(receiver, args)
		}
	case "prepend":
		if len(args) == 1 {
			return t.createPrependCall(receiver, args[0])
		}
	case "insert":
		if len(args) == 2 {
			return t.createInsertCall(receiver, args[0], args[1])
		}
	case "remove", "delete":
		if len(args) == 1 {
			return t.createRemoveCall(receiver, args[0])
		}
	case "removeAt":
		if len(args) == 1 {
			return t.createRemoveAtCall(receiver, args[0])
		}

	// Slice operations
	case "slice", "sub":
		if len(args) == 2 {
			return t.createSliceCall(receiver, args[0], args[1])
		}
	case "from":
		if len(args) == 1 {
			return t.createFromCall(receiver, args[0])
		}
	case "to":
		if len(args) == 1 {
			return t.createToCall(receiver, args[0])
		}

	// Functional methods
	case "reverse":
		return t.createReverseCall(receiver)
	case "sort":
		return t.createSortCall(receiver)
	case "sortBy":
		if len(args) == 1 {
			return t.createSortByCall(receiver, args[0])
		}
	case "unique", "distinct":
		return t.createUniqueCall(receiver)
	case "filter":
		if len(args) == 1 {
			return t.createFilterCall(receiver, args[0])
		}
	case "map":
		if len(args) == 1 {
			return t.createMapCall(receiver, args[0])
		}

	// Aggregation methods
	case "join":
		if len(args) == 1 {
			return t.createJoinCall(receiver, args[0])
		}
	case "sum":
		return t.createSumCall(receiver)
	case "min":
		return t.createMinCall(receiver)
	case "max":
		return t.createMaxCall(receiver)

	// Comparison methods
	case "equals":
		if len(args) == 1 {
			return t.createEqualsCall(receiver, args[0])
		}

	// Copy methods
	case "copy", "clone":
		return t.createCopyCall(receiver)
	}

	// If we reach here, method is not recognized at all
	return t.createCompilerError(receiver, methodName, "unknown list method")
}

func (t *ListMethodsTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	println("ListMethodsTransform.Transform called")
	changed := false

	// Transform all declarations
	for i, decl := range file.DeclList {
		if newDecl := t.transformDecl(decl, ctx); newDecl != decl {
			file.DeclList[i] = newDecl
			changed = true
		}
	}

	// Add required imports if needed and transformations were made
	// Note: Skip adding imports for now to focus on core functionality
	// if changed && !t.hasImport(file, "slices") {
	//     println("Adding slices import")
	//     t.addSlicesImport(file)
	// }

	return changed
}

func (t *ListMethodsTransform) transformDecl(decl syntax.Decl, ctx *TransformContext) syntax.Decl {
	switch d := decl.(type) {
	case *syntax.FuncDecl:
		if newBody := t.transformStmt(d.Body, ctx); newBody != d.Body {
			newDecl := *d
			if blockStmt, ok := newBody.(*syntax.BlockStmt); ok {
				newDecl.Body = blockStmt
			}
			return &newDecl
		}
	case *syntax.VarDecl:
		if d.Values != nil {
			if newValues := t.transformExpr(d.Values, ctx); newValues != d.Values {
				newDecl := *d
				newDecl.Values = newValues
				return &newDecl
			}
		}
	}
	return decl
}

func (t *ListMethodsTransform) transformStmt(stmt syntax.Stmt, ctx *TransformContext) syntax.Stmt {
	if stmt == nil {
		return nil
	}

	switch s := stmt.(type) {
	case *syntax.BlockStmt:
		changed := false
		newList := make([]syntax.Stmt, len(s.List))
		for i, stmt := range s.List {
			newStmt := t.transformStmt(stmt, ctx)
			newList[i] = newStmt
			if newStmt != stmt {
				changed = true
			}
		}
		if changed {
			newBlock := *s
			newBlock.List = newList
			return &newBlock
		}
	case *syntax.ExprStmt:
		if newExpr := t.transformExpr(s.X, ctx); newExpr != s.X {
			newStmt := *s
			newStmt.X = newExpr
			return &newStmt
		}
	case *syntax.AssignStmt:
		lhsChanged := false
		rhsChanged := false
		newLhs := t.transformExpr(s.Lhs, ctx)
		newRhs := t.transformExpr(s.Rhs, ctx)
		if newLhs != s.Lhs {
			lhsChanged = true
		}
		if newRhs != s.Rhs {
			rhsChanged = true
		}
		if lhsChanged || rhsChanged {
			newStmt := *s
			newStmt.Lhs = newLhs
			newStmt.Rhs = newRhs
			return &newStmt
		}
	case *syntax.ReturnStmt:
		if s.Results != nil {
			if newResults := t.transformExpr(s.Results, ctx); newResults != s.Results {
				newStmt := *s
				newStmt.Results = newResults
				return &newStmt
			}
		}
	case *syntax.CheckStmt:
		if newCond := t.transformExpr(s.Cond, ctx); newCond != s.Cond {
			newStmt := *s
			newStmt.Cond = newCond
			return &newStmt
		}
	}
	return stmt
}

func (t *ListMethodsTransform) transformExpr(expr syntax.Expr, ctx *TransformContext) syntax.Expr {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *syntax.CallExpr:
		// Check if this is a list method call
		if selector, ok := e.Fun.(*syntax.SelectorExpr); ok {
			if t.isListExpression(selector.X, ctx) {
				methodName := selector.Sel.Value
				if transformed := t.transformListMethod(selector.X, methodName, e.ArgList); transformed != nil {
					println("TRANSFORMING list method:", methodName)
					return transformed
				}
			}
		}
		// Transform function and arguments
		funChanged := false
		argsChanged := false
		newFun := t.transformExpr(e.Fun, ctx)
		if newFun != e.Fun {
			funChanged = true
		}
		var newArgList []syntax.Expr
		if e.ArgList != nil {
			newArgList = make([]syntax.Expr, len(e.ArgList))
			for i, arg := range e.ArgList {
				newArg := t.transformExpr(arg, ctx)
				newArgList[i] = newArg
				if newArg != arg {
					argsChanged = true
				}
			}
		}
		if funChanged || argsChanged {
			newCall := *e
			newCall.Fun = newFun
			newCall.ArgList = newArgList
			return &newCall
		}
	case *syntax.Operation:
		xChanged := false
		yChanged := false
		newX := t.transformExpr(e.X, ctx)
		if newX != e.X {
			xChanged = true
		}
		var newY syntax.Expr
		if e.Y != nil {
			newY = t.transformExpr(e.Y, ctx)
			if newY != e.Y {
				yChanged = true
			}
		}
		if xChanged || yChanged {
			newOp := *e
			newOp.X = newX
			newOp.Y = newY
			return &newOp
		}
	case *syntax.ParenExpr:
		if newX := t.transformExpr(e.X, ctx); newX != e.X {
			newParen := *e
			newParen.X = newX
			return &newParen
		}
	}
	return expr
}


// Basic list operations

// createLenCall creates len(receiver)
func (t *ListMethodsTransform) createLenCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	lenName := &syntax.Name{Value: "len"}
	lenName.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     lenName,
		ArgList: []syntax.Expr{receiver},
	}
	call.SetPos(pos)

	return call
}

// createIsEmptyCall creates len(receiver) == 0
func (t *ListMethodsTransform) createIsEmptyCall(receiver syntax.Expr) syntax.Expr {
	// Since we can't easily replace a call with an operation in the AST,
	// we'll create a helper function call that returns the boolean result
	return &syntax.CallExpr{
		Fun:     &syntax.Name{Value: "listIsEmpty"},
		ArgList: []syntax.Expr{receiver},
	}
}

// createFirstCall creates receiver[0]
func (t *ListMethodsTransform) createFirstCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	index := &syntax.IndexExpr{
		X:     receiver,
		Index: &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"},
	}
	index.SetPos(pos)

	return index
}

// createLastCall creates receiver[len(receiver)-1]
func (t *ListMethodsTransform) createLastCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	// Create len(receiver)
	lenCall := &syntax.CallExpr{
		Fun:     &syntax.Name{Value: "len"},
		ArgList: []syntax.Expr{receiver},
	}
	lenCall.SetPos(pos)

	// Create len(receiver) - 1
	minusOne := &syntax.Operation{
		Op: syntax.Sub,
		X:  lenCall,
		Y:  &syntax.BasicLit{Kind: syntax.IntLit, Value: "1"},
	}
	minusOne.SetPos(pos)

	// Create receiver[len(receiver)-1]
	index := &syntax.IndexExpr{
		X:     receiver,
		Index: minusOne,
	}
	index.SetPos(pos)

	return index
}

// createGetCall creates receiver[index]
func (t *ListMethodsTransform) createGetCall(receiver, index syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	call := &syntax.CallExpr{
		Fun:     &syntax.Name{Value: "listGet"},
		ArgList: []syntax.Expr{receiver, index},
	}
	call.SetPos(pos)

	return call
}

// createContainsCall creates slices.Contains(receiver, value)
func (t *ListMethodsTransform) createContainsCall(receiver, value syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	slicesName := &syntax.Name{Value: "slices"}
	slicesName.SetPos(pos)

	funcName := &syntax.Name{Value: "Contains"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   slicesName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, value},
	}
	call.SetPos(pos)

	return call
}

// createIndexCall creates slices.Index(receiver, value)
func (t *ListMethodsTransform) createIndexCall(receiver, value syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	slicesName := &syntax.Name{Value: "slices"}
	slicesName.SetPos(pos)

	funcName := &syntax.Name{Value: "Index"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   slicesName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, value},
	}
	call.SetPos(pos)

	return call
}

// createAppendCall creates append(receiver, args...)
func (t *ListMethodsTransform) createAppendCall(receiver syntax.Expr, args []syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	appendName := &syntax.Name{Value: "append"}
	appendName.SetPos(pos)

	argList := make([]syntax.Expr, 0, len(args)+1)
	argList = append(argList, receiver)
	argList = append(argList, args...)

	call := &syntax.CallExpr{
		Fun:     appendName,
		ArgList: argList,
	}
	call.SetPos(pos)

	return call
}

// createSliceCall creates receiver[start:end]
func (t *ListMethodsTransform) createSliceCall(receiver, start, end syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	slice := &syntax.SliceExpr{
		X:     receiver,
		Index: [3]syntax.Expr{start, end, nil},
	}
	slice.SetPos(pos)

	return slice
}

// createFromCall creates receiver[start:]
func (t *ListMethodsTransform) createFromCall(receiver, start syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	slice := &syntax.SliceExpr{
		X:     receiver,
		Index: [3]syntax.Expr{start, nil, nil},
	}
	slice.SetPos(pos)

	return slice
}

// createToCall creates receiver[:end]
func (t *ListMethodsTransform) createToCall(receiver, end syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	slice := &syntax.SliceExpr{
		X:     receiver,
		Index: [3]syntax.Expr{nil, end, nil},
	}
	slice.SetPos(pos)

	return slice
}

// createReverseCall creates slices.Reverse(receiver)
func (t *ListMethodsTransform) createReverseCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "slices"},
			Sel: &syntax.Name{Value: "Reverse"},
		},
		ArgList: []syntax.Expr{receiver},
	}
}

// createSortCall creates slices.Sort(receiver)
func (t *ListMethodsTransform) createSortCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "slices"},
			Sel: &syntax.Name{Value: "Sort"},
		},
		ArgList: []syntax.Expr{receiver},
	}
}

// createCopyCall creates slices.Clone(receiver)
func (t *ListMethodsTransform) createCopyCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "slices"},
			Sel: &syntax.Name{Value: "Clone"},
		},
		ArgList: []syntax.Expr{receiver},
	}
}

// Helper methods for complex operations that need runtime implementation

func (t *ListMethodsTransform) createCountCall(receiver, value syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "count", "count_occurrences")
}

func (t *ListMethodsTransform) createLastIndexCall(receiver, value syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "lastIndexOf", "last_index_search")
}

func (t *ListMethodsTransform) createPrependCall(receiver, value syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "prepend", "prepend_element")
}

func (t *ListMethodsTransform) createInsertCall(receiver, index, value syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "insert", "insert_at_index")
}

func (t *ListMethodsTransform) createRemoveCall(receiver, value syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "remove", "remove_element")
}

func (t *ListMethodsTransform) createRemoveAtCall(receiver, index syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "removeAt", "remove_at_index")
}

func (t *ListMethodsTransform) createSortByCall(receiver, keyFunc syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "sortBy", "sort_by_function")
}

func (t *ListMethodsTransform) createUniqueCall(receiver syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "unique", "remove_duplicates")
}

func (t *ListMethodsTransform) createFilterCall(receiver, predicate syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "filter", "filter_elements")
}

func (t *ListMethodsTransform) createMapCall(receiver, transform syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "map", "transform_elements")
}

func (t *ListMethodsTransform) createJoinCall(receiver, separator syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "join", "join_elements")
}

func (t *ListMethodsTransform) createSumCall(receiver syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "sum", "sum_elements")
}

func (t *ListMethodsTransform) createMinCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "slices"},
			Sel: &syntax.Name{Value: "Min"},
		},
		ArgList: []syntax.Expr{receiver},
	}
}

func (t *ListMethodsTransform) createMaxCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "slices"},
			Sel: &syntax.Name{Value: "Max"},
		},
		ArgList: []syntax.Expr{receiver},
	}
}

func (t *ListMethodsTransform) createEqualsCall(receiver, other syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "slices"},
			Sel: &syntax.Name{Value: "Equal"},
		},
		ArgList: []syntax.Expr{receiver, other},
	}
}

// createCompilerError creates a compiler error for unimplemented methods
func (t *ListMethodsTransform) createCompilerError(receiver syntax.Expr, methodName, description string) syntax.Expr {
	// Instead of creating a syntax error, we'll create a call to a non-existent function
	// that will produce a clear error message
	errorFuncName := "TODO_implement_runtime_function_for_list_" + methodName + "_" + description
	return &syntax.CallExpr{
		Fun:     &syntax.Name{Value: errorFuncName},
		ArgList: []syntax.Expr{receiver},
	}
}

// isListExpression returns true if the expression is definitely a list/slice
func (t *ListMethodsTransform) isListExpression(expr syntax.Expr, ctx *TransformContext) bool {
	// Check if it's a slice literal (including array literals which can be slices)
	if composite, ok := expr.(*syntax.CompositeLit); ok {
		// Handle [1, 2, 3] syntax (without explicit type)
		if composite.Type == nil {
			return true // assume it's a slice literal
		}
		// Handle []int{1, 2, 3} syntax
		if arrayType, ok := composite.Type.(*syntax.ArrayType); ok {
			return arrayType.Len == nil // slice type
		}
		// Handle [3]int{1, 2, 3} syntax (arrays can also use methods)
		return true
	}

	// Check if it's a list variable with known type
	if name, ok := expr.(*syntax.Name); ok {
		if ctx != nil && ctx.Types != nil {
			varType := ctx.Types[name.Value]
			return varType == "[]any" || varType == "list" || varType == "slice" || varType == "[]int"
		}
		// For now, assume named variables could be slices if they're not in types
		return true
	}

	// Check if it's a slice/array index or slice operation result
	if _, ok := expr.(*syntax.SliceExpr); ok {
		return true
	}

	// Check if it's a function call that returns a slice
	if call, ok := expr.(*syntax.CallExpr); ok {
		if selector, ok := call.Fun.(*syntax.SelectorExpr); ok {
			// Common slice-returning methods
			sliceReturningMethods := []string{"append", "copy", "reverse", "sort"}
			methodName := selector.Sel.Value
			for _, method := range sliceReturningMethods {
				if method == methodName {
					return true
				}
			}
		}
		// Built-in functions that return slices
		if name, ok := call.Fun.(*syntax.Name); ok {
			if name.Value == "append" || name.Value == "make" {
				return true
			}
		}
	}

	// For other cases, be conservative
	return false
}

func (t *ListMethodsTransform) addSlicesImport(file *syntax.File) {
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

func (t *ListMethodsTransform) addSortImport(file *syntax.File) {
	if t.hasImport(file, "sort") {
		return
	}

	sortImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: "\"sort\"",
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
	newDeclList = append(newDeclList, sortImport)
	newDeclList = append(newDeclList, file.DeclList[insertPos:]...)
	file.DeclList = newDeclList
}

func (t *ListMethodsTransform) hasImport(file *syntax.File, name string) bool {
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

func init() {
	RegisterTransformer(&ListMethodsTransform{})
}