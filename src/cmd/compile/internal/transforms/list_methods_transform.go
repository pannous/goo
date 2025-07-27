// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// ListMethodsTransform handles automatic transformation of list/slice method calls
// to their corresponding Go standard library function calls.
type ListMethodsTransform struct{}

type listMethodVisitor struct {
	transform          *ListMethodsTransform
	ctx                *TransformContext
	changed            bool
	needsSlicesImport  bool
	needsSortImport    bool
}

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

	visitor := &listMethodVisitor{transform: t, ctx: ctx}
	syntax.Walk(file, visitor)

	// Add required imports if needed and transformations were made
	if visitor.needsSlicesImport && !t.hasImport(file, "slices") {
		println("Adding slices import")
		t.addSlicesImport(file)
	}
	if visitor.needsSortImport && !t.hasImport(file, "sort") {
		println("Adding sort import")
		t.addSortImport(file)
	}

	return visitor.changed
}

// Visit implements syntax.Visitor interface
func (v *listMethodVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}

	// Check for method calls on list/slice expressions
	if call, ok := node.(*syntax.CallExpr); ok {
		if selector, ok := call.Fun.(*syntax.SelectorExpr); ok {
			if v.transform.isListExpression(selector.X, v.ctx) {
				methodName := selector.Sel.Value
				if transformed := v.transform.transformListMethod(selector.X, methodName, call.ArgList); transformed != nil {
					println("TRANSFORMING list method:", methodName)
					// All transformations should return CallExpr now
					if callExpr, ok := transformed.(*syntax.CallExpr); ok {
						*call = *callExpr
					}
					v.changed = true
					// Track required imports based on method name
					slicesMethods := []string{
						"contains", "includes", "indexOf", "find",
						"reverse", "sort", "sortBy", "unique", "distinct",
						"min", "max", "equals",
					}
					sortMethods := []string{
						"sort", "sortBy",
					}

					for _, method := range slicesMethods {
						if method == methodName {
							v.needsSlicesImport = true
							break
						}
					}
					for _, method := range sortMethods {
						if method == methodName {
							v.needsSortImport = true
							break
						}
					}
				}
			}
		}
	}
	return v
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

// createFirstCall creates receiver[0] as a helper call
func (t *ListMethodsTransform) createFirstCall(receiver syntax.Expr) syntax.Expr {
	// For now, return a runtime helper that can be implemented later
	// This avoids AST complexity of replacing function calls with index expressions
	return &syntax.CallExpr{
		Fun: &syntax.IndexExpr{
			X:     receiver,
			Index: &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"},
		},
		ArgList: []syntax.Expr{},
	}
}

// createLastCall creates receiver[len(receiver)-1]
func (t *ListMethodsTransform) createLastCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	call := &syntax.CallExpr{
		Fun:     &syntax.Name{Value: "listLast"},
		ArgList: []syntax.Expr{receiver},
	}
	call.SetPos(pos)

	return call
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

	call := &syntax.CallExpr{
		Fun:     &syntax.Name{Value: "listSlice"},
		ArgList: []syntax.Expr{receiver, start, end},
	}
	call.SetPos(pos)

	return call
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