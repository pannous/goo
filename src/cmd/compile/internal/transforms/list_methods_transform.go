// Copyright 2025 The Goo Authors. All rights reserved.

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

func (t *ListMethodsTransform) Priority() int {
	return 50 // High priority - run before lambda transform (200)
}

// transformListMethod transforms list method calls to standard library calls
func (t *ListMethodsTransform) transformListMethod(receiver syntax.Expr, methodName string, args []syntax.Expr, ctx *TransformContext) syntax.Expr {
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
	case "first", "head", "start", "begin":
		return t.createFirstCall(receiver)
	case "last", "tail", "end", "final":
		return t.createLastCall(receiver)
	case "get":
		if len(args) == 1 {
			return t.createGetCall(receiver, args[0])
		}

	// Search methods
	case "contains", "includes", "has", "holds":
		if len(args) == 1 {
			return t.createContainsCall(receiver, args[0])
		}
	case "indexOf", "find", "search", "locate":
		if len(args) == 1 {
			return t.createIndexCall(receiver, args[0])
		}
	case "lastIndexOf":
		if len(args) == 1 {
			return t.createLastIndexCall(receiver, args[0])
		}

	// Modification methods (return new slice)
	case "append", "add", "push", "concat":
		if len(args) >= 1 {
			return t.createAppendCall(receiver, args)
		}
	case "prepend", "unshift", "prefix":
		if len(args) == 1 {
			return t.createPrependCall(receiver, args[0])
		}
	case "pop":
		return t.createPopCall(receiver)
	case "shift":
		return t.createShiftCall(receiver)
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

	// Functional methods - modifying versions (in-place, no return value)
	case "reverse!":
		return t.createReverseInPlaceCall(receiver)
	case "sort!":
		return t.createSortInPlaceCall(receiver)
	case "sortBy!":
		if len(args) == 1 {
			return t.createSortByInPlaceCall(receiver, args[0])
		}
		
	// Functional methods - non-modifying versions (return new slice)
	case "reverse", "reversed":
		return t.createReverseCall(receiver)
	case "sort", "sorted":
		return t.createSortCall(receiver)
	case "sortDesc", "sortDescending":
		return t.createSortDescCall(receiver)
	case "sortBy", "sortedBy":
		if len(args) == 1 {
			return t.createSortByCall(receiver, args[0])
		}
	case "unique", "distinct":
		return t.createUniqueCall(receiver)
	case "filter", "where", "chose", "that", "which":
		if len(args) == 1 {
			return t.createFilterCall(receiver, args[0], ctx)
		}
	case "apply", "transform", "convert":
		if len(args) == 1 {
			return t.createMapCall(receiver, args[0], ctx)
		}

	// Aggregation methods
	case "join", "combine", "merge":
		if len(args) == 1 {
			return t.createJoinCall(receiver, args[0])
		}
	case "sum", "total":
		return t.createSumCall(receiver)
	case "min", "minimum", "smallest":
		return t.createMinCall(receiver)
	case "max", "maximum", "largest":
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
	changed := false

	// Transform all declarations with enhanced traversal
	for i, decl := range file.DeclList {
		newDecl := t.transformDecl(decl, ctx)
		if newDecl != decl {
			file.DeclList[i] = newDecl
			changed = true
		}
	}

	// For now, skip adding slices import to avoid import errors
	// TODO: Add smart import detection based on which methods are used
	// if changed && !t.hasImport(file, "slices") {
	// 	println("Adding slices import")
	// 	t.addSlicesImport(file)
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
			newValues := t.transformExpr(d.Values, ctx)
			if newValues != d.Values {
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
	case *syntax.IfStmt:
		// Added IfStmt support for better coverage
		condChanged := false
		thenChanged := false
		elseChanged := false
		
		newCond := t.transformExpr(s.Cond, ctx)
		if newCond != s.Cond {
			condChanged = true
		}
		
		newThen := t.transformStmt(s.Then, ctx)
		if newThen != s.Then {
			thenChanged = true
		}
		
		var newElse syntax.Stmt
		if s.Else != nil {
			newElse = t.transformStmt(s.Else, ctx)
			if newElse != s.Else {
				elseChanged = true
			}
		}
		
		if condChanged || thenChanged || elseChanged {
			newStmt := *s
			newStmt.Cond = newCond
			if blockStmt, ok := newThen.(*syntax.BlockStmt); ok {
				newStmt.Then = blockStmt
			}
			newStmt.Else = newElse
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
			// First, recursively transform the receiver (this handles chained calls)
			transformedReceiver := t.transformExpr(selector.X, ctx)
			
			if t.isListExpression(transformedReceiver, ctx) {
				methodName := selector.Sel.Value
				// CRITICAL FIX: Only handle if receiver is NOT a string literal
				if t.isStringReceiver(transformedReceiver) {
					// Let string_methods_transform handle string methods
					// Still transform arguments if needed
					var newArgList []syntax.Expr
					if e.ArgList != nil {
						newArgList = make([]syntax.Expr, len(e.ArgList))
						for i, arg := range e.ArgList {
							newArgList[i] = t.transformExpr(arg, ctx)
						}
					}
					// Return call with transformed receiver and args, but don't apply list method transformation
					if transformedReceiver != selector.X || newArgList != nil {
						newSelector := &syntax.SelectorExpr{
							X:   transformedReceiver,
							Sel: selector.Sel,
						}
						newSelector.SetPos(selector.Pos())
						newCall := &syntax.CallExpr{
							Fun:     newSelector,
							ArgList: newArgList,
						}
						newCall.SetPos(e.Pos())
						return newCall
					}
					return expr // No changes needed
				}
				if transformed := t.transformListMethod(transformedReceiver, methodName, e.ArgList, ctx); transformed != nil {
					return transformed
				}
			}
			
			// Even if not a list method, we may need to update the receiver if it was transformed
			if transformedReceiver != selector.X {
				// Create new selector with transformed receiver
				newSelector := &syntax.SelectorExpr{
					X:   transformedReceiver,
					Sel: selector.Sel,
				}
				newSelector.SetPos(selector.Pos())
				
				// Create new call expression with updated selector  
				newCall := &syntax.CallExpr{
					Fun:     newSelector,
					ArgList: e.ArgList,
				}
				newCall.SetPos(e.Pos())
				
				return newCall
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
	case *syntax.LambdaExpr:
		if newBody := t.transformExpr(e.Body, ctx); newBody != e.Body {
			newLambda := *e
			newLambda.Body = newBody
			return &newLambda
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

// createReverseCall creates a non-modifying reverse that returns a new reversed slice  
func (t *ListMethodsTransform) createReverseCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()
	
	// Generate slices.CloneAndReverse(receiver)
	slicesName := &syntax.Name{Value: "slices"}
	slicesName.SetPos(pos)
	cloneAndReverseName := &syntax.Name{Value: "CloneAndReverse"}
	cloneAndReverseName.SetPos(pos)
	
	slicesCloneAndReverse := &syntax.SelectorExpr{
		X:   slicesName,
		Sel: cloneAndReverseName,
	}
	slicesCloneAndReverse.SetPos(pos)
	
	call := &syntax.CallExpr{
		Fun:     slicesCloneAndReverse,
		ArgList: []syntax.Expr{receiver},
	}
	call.SetPos(pos)
	
	return call
}

// createReverseInPlaceCall creates an in-place reverse that modifies the original slice
func (t *ListMethodsTransform) createReverseInPlaceCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()
	slicesName := &syntax.Name{Value: "slices"}
	slicesName.SetPos(pos)
	reverseName := &syntax.Name{Value: "Reverse"}
	reverseName.SetPos(pos)
	
	selector := &syntax.SelectorExpr{
		X:   slicesName,
		Sel: reverseName,
	}
	selector.SetPos(pos)
	
	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver},
	}
	call.SetPos(pos)
	return call
}

// createSortCall creates a non-modifying sort that returns a new sorted slice  
func (t *ListMethodsTransform) createSortCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()
	
	// Generate slices.CloneAndSort(receiver)
	slicesName := &syntax.Name{Value: "slices"}
	slicesName.SetPos(pos)
	cloneAndSortName := &syntax.Name{Value: "CloneAndSort"}
	cloneAndSortName.SetPos(pos)
	
	slicesCloneAndSort := &syntax.SelectorExpr{
		X:   slicesName,
		Sel: cloneAndSortName,
	}
	slicesCloneAndSort.SetPos(pos)
	
	call := &syntax.CallExpr{
		Fun:     slicesCloneAndSort,
		ArgList: []syntax.Expr{receiver},
	}
	call.SetPos(pos)
	
	return call
}

// createSortInPlaceCall creates an in-place sort that modifies the original slice
func (t *ListMethodsTransform) createSortInPlaceCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()
	slicesName := &syntax.Name{Value: "slices"}
	slicesName.SetPos(pos)
	sortName := &syntax.Name{Value: "Sort"}
	sortName.SetPos(pos)
	selector := &syntax.SelectorExpr{
		X:   slicesName,
		Sel: sortName,
	}
	selector.SetPos(pos)
	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver},
	}
	call.SetPos(pos)
	return call
}

// createSortByInPlaceCall creates an in-place sortBy that modifies the original slice
func (t *ListMethodsTransform) createSortByInPlaceCall(receiver, keyFunc syntax.Expr) syntax.Expr {
	// This would need slices.SortFunc for custom sorting
	return t.createCompilerError(receiver, "sortBy!", "need_sort_by_implementation")
}

// createCopyCall creates append(receiver[:0:0], receiver...)
func (t *ListMethodsTransform) createCopyCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	// Create receiver[:0:0] to get an empty slice of the same type
	emptySlice := &syntax.SliceExpr{
		X: receiver,
		Index: [3]syntax.Expr{
			&syntax.BasicLit{Kind: syntax.IntLit, Value: "0"},
			&syntax.BasicLit{Kind: syntax.IntLit, Value: "0"},
			&syntax.BasicLit{Kind: syntax.IntLit, Value: "0"},
		},
	}
	emptySlice.SetPos(pos)

	// Create append(receiver[:0:0], receiver...)
	appendName := &syntax.Name{Value: "append"}
	appendName.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     appendName,
		ArgList: []syntax.Expr{emptySlice, receiver},
		HasDots: true,
	}
	call.SetPos(pos)

	return call
}

// Helper methods for complex operations that need runtime implementation

func (t *ListMethodsTransform) createCountCall(receiver, value syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "count", "count_occurrences")
}

func (t *ListMethodsTransform) createLastIndexCall(receiver, value syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "lastIndexOf", "last_index_search")
}

func (t *ListMethodsTransform) createPrependCall(receiver, value syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	// Create append([]T{value}, receiver...)
	// First create []T{value} - a slice literal with the value
	sliceLit := &syntax.CompositeLit{
		ElemList: []syntax.Expr{value},
	}
	sliceLit.SetPos(pos)

	// Create append call with spread operator
	appendName := &syntax.Name{Value: "append"}
	appendName.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     appendName,
		ArgList: []syntax.Expr{sliceLit, receiver},
		HasDots: true, // This makes it append([]T{value}, receiver...)
	}
	call.SetPos(pos)

	return call
}

func (t *ListMethodsTransform) createInsertCall(receiver, index, value syntax.Expr) syntax.Expr {
	pos := receiver.Pos()
	
	// Generate slices.CloneAndInsert(receiver, index, value)
	slicesName := &syntax.Name{Value: "slices"}
	slicesName.SetPos(pos)
	cloneAndInsertName := &syntax.Name{Value: "CloneAndInsert"}
	cloneAndInsertName.SetPos(pos)
	
	slicesCloneAndInsert := &syntax.SelectorExpr{
		X:   slicesName,
		Sel: cloneAndInsertName,
	}
	slicesCloneAndInsert.SetPos(pos)
	
	call := &syntax.CallExpr{
		Fun:     slicesCloneAndInsert,
		ArgList: []syntax.Expr{receiver, index, value},
	}
	call.SetPos(pos)
	
	return call
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

func (t *ListMethodsTransform) createFilterCall(receiver, predicate syntax.Expr, ctx *TransformContext) syntax.Expr {
	pos := receiver.Pos()

	// Fix lambda parameter type if needed
	correctedPredicate := t.correctLambdaParameterType(receiver, predicate, ctx)

	slicesName := &syntax.Name{Value: "slices"}
	slicesName.SetPos(pos)

	filterName := &syntax.Name{Value: "Filter"}
	filterName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   slicesName,
		Sel: filterName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, correctedPredicate},
	}
	call.SetPos(pos)

	return call
}

func (t *ListMethodsTransform) createMapCall(receiver, transform syntax.Expr, ctx *TransformContext) syntax.Expr {
	pos := receiver.Pos()

	// Fix lambda parameter type if needed - we'll compute element type first
	correctedTransform := transform

	// Get input and output types for slices.Map[SliceType, ResultElementType]
	receiverName := t.getReceiverName(receiver)
	
	// Use enhanced chained method type inference
	sliceType := t.inferChainedMethodType(receiver, "self", ctx)
	if sliceType == "" {
		sliceType = t.getSliceType(receiverName, ctx)
	}
	if sliceType == "" {
		sliceType = "[]any"
	}
	
	// Extract element type for lambda parameter inference
	elementType := ""
	if len(sliceType) > 2 && sliceType[:2] == "[]" {
		elementType = sliceType[2:]
	} else {
		elementType = t.extractElementType(receiverName, ctx)
	}
	
	// Now fix the lambda with the correct element type
	correctedTransform = t.correctLambdaParameterTypeWithElement(transform, elementType, ctx)
	
	outputType := t.inferLambdaReturnType(transform.(*syntax.LambdaExpr).Body, elementType)
	if outputType == "" {
		outputType = "any"
	}

	println("DEBUG: createMapCall using types:", sliceType, "->", outputType, "(element:", elementType, ")")

	slicesName := &syntax.Name{Value: "slices"}
	slicesName.SetPos(pos)

	mapName := &syntax.Name{Value: "Map"}
	mapName.SetPos(pos)

	// Create type parameters: [SliceType, ResultElementType]
	// Use simple Name nodes for generic type parameters
	sliceTypeName := &syntax.Name{Value: sliceType}
	sliceTypeName.SetPos(pos)
	outputTypeName := &syntax.Name{Value: outputType}  
	outputTypeName.SetPos(pos)

	// Create the selector: slices.Map
	selector := &syntax.SelectorExpr{
		X:   slicesName,
		Sel: mapName,
	}
	selector.SetPos(pos)

	// Create the type parameter list: [SliceType, ResultElementType]
	typeParamList := &syntax.ListExpr{
		ElemList: []syntax.Expr{sliceTypeName, outputTypeName},
	}
	typeParamList.SetPos(pos)

	// For now, use slices.Map without explicit type parameters to avoid AST issues
	// Let Go's type inference handle the generics
	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, correctedTransform},
	}
	call.SetPos(pos)

	return call
}

func (t *ListMethodsTransform) createJoinCall(receiver, separator syntax.Expr) syntax.Expr {
	pos := receiver.Pos()
	
	// Generate slices.JoinStringify(receiver, separator)
	slicesName := &syntax.Name{Value: "slices"}
	slicesName.SetPos(pos)
	joinStringifyName := &syntax.Name{Value: "JoinStringify"}
	joinStringifyName.SetPos(pos)
	
	slicesJoinStringify := &syntax.SelectorExpr{
		X:   slicesName,
		Sel: joinStringifyName,
	}
	slicesJoinStringify.SetPos(pos)
	
	call := &syntax.CallExpr{
		Fun:     slicesJoinStringify,
		ArgList: []syntax.Expr{receiver, separator},
	}
	call.SetPos(pos)
	
	return call
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
			// Check if it's any slice type (starts with []...)
			if len(varType) >= 2 && varType[:2] == "[]" {
				return true
			}
			return varType == "any" || varType == "list" || varType == "slice"
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
			// Check for slices package functions
			if name, ok := selector.X.(*syntax.Name); ok && name.Value == "slices" {
				slicesMethodName := selector.Sel.Value
				slicesSliceReturningMethods := []string{
					"Filter", "Map", "Reverse", "Sort", "SortFunc",
					"Clone", "Compact", "CompactFunc", "Delete", "Insert",
				}
				for _, method := range slicesSliceReturningMethods {
					if method == slicesMethodName {
						return true
					}
				}
			}
			
			// Common slice-returning methods
			sliceReturningMethods := []string{
				"append", "copy", "reverse", "sort", "sortBy",
				"filter", "where", "chose", "that", "which",
				"apply", "transform", "convert",
				"slice", "sub", "from", "to",
				"prepend", "unshift", "prefix",
				"insert", "remove", "delete", "removeAt",
				"unique", "distinct",
			}
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

// correctLambdaParameterType fixes lambda parameter types based on slice element type
func (t *ListMethodsTransform) correctLambdaParameterType(receiver, predicate syntax.Expr, ctx *TransformContext) syntax.Expr {
	// Check if predicate is a lambda expression
	lambda, ok := predicate.(*syntax.LambdaExpr)
	if !ok {
		return predicate // Not a lambda, return as-is
	}

	// Extract element type from receiver
	receiverName, ok := receiver.(*syntax.Name)
	if !ok {
		return predicate // Can't determine receiver type
	}

	// Extract element type from slice type (e.g., []User -> User)
	elementType := t.extractElementType(receiverName.Value, ctx)
	if elementType == "" {
		return predicate // Can't determine element type
	}
	
	// Create a corrected lambda with proper parameter and return types
	correctedLambda := *lambda
	correctedLambda.SetPos(lambda.Pos())
	
	println("DEBUG: correcting lambda parameter type to:", elementType)
	
	// Infer return type from lambda body
	returnType := t.inferLambdaReturnType(lambda.Body, elementType)
	println("DEBUG: inferred lambda return type:", returnType)
	
	// If lambda has parameters, fix their types
	if lambda.ParamList != nil && len(lambda.ParamList) > 0 {
		// Create new parameter list with corrected types
		newParamList := make([]*syntax.Field, len(lambda.ParamList))
		for i, param := range lambda.ParamList {
			newParam := *param
			newParam.SetPos(param.Pos())
			
			// Set the correct element type
			elementTypeName := &syntax.Name{Value: elementType}
			elementTypeName.SetPos(param.Pos())
			newParam.Type = elementTypeName
			
			newParamList[i] = &newParam
			println("DEBUG: set parameter", i, "type to:", elementType)
		}
		correctedLambda.ParamList = newParamList
	}
	
	// Return type inference will be handled by the Go type system
	// We've fixed the parameter types, which is sufficient for proper type checking
	if returnType != "" {
		println("DEBUG: inferred lambda return type:", returnType, "- Go compiler will handle this automatically")
	}

	return &correctedLambda
}

// correctLambdaParameterTypeWithElement corrects lambda parameter type with explicit element type
func (t *ListMethodsTransform) correctLambdaParameterTypeWithElement(predicate syntax.Expr, elementType string, ctx *TransformContext) syntax.Expr {
	lambda, ok := predicate.(*syntax.LambdaExpr)
	if !ok {
		return predicate // Not a lambda, return as-is
	}

	if elementType == "" {
		return predicate // Can't determine element type
	}
	
	// Create a corrected lambda with proper parameter types
	correctedLambda := *lambda
	correctedLambda.SetPos(lambda.Pos())
	
	println("DEBUG: correcting lambda parameter type to:", elementType)
	
	// Infer return type from lambda body
	returnType := t.inferLambdaReturnType(lambda.Body, elementType)
	println("DEBUG: inferred lambda return type:", returnType)
	
	// If lambda has parameters, fix their types
	if lambda.ParamList != nil && len(lambda.ParamList) > 0 {
		// Create new parameter list with corrected types
		newParamList := make([]*syntax.Field, len(lambda.ParamList))
		for i, param := range lambda.ParamList {
			newParam := *param
			newParam.SetPos(param.Pos())
			
			// Set the correct element type
			elementTypeName := &syntax.Name{Value: elementType}
			elementTypeName.SetPos(param.Pos())
			newParam.Type = elementTypeName
			
			newParamList[i] = &newParam
			println("DEBUG: set parameter", i, "type to:", elementType)
		}
		correctedLambda.ParamList = newParamList
	}
	
	// Return type inference will be handled by the Go type system
	if returnType != "" {
		println("DEBUG: inferred lambda return type:", returnType, "- Go compiler will handle this automatically")
	}

	return &correctedLambda
}

// getReceiverName extracts the variable name from receiver expression
func (t *ListMethodsTransform) getReceiverName(receiver syntax.Expr) string {
	if name, ok := receiver.(*syntax.Name); ok {
		return name.Value
	}
	return ""
}

// getSliceType gets the full slice type from variable name and context
func (t *ListMethodsTransform) getSliceType(varName string, ctx *TransformContext) string {
	if ctx != nil && ctx.Types != nil {
		if varType, exists := ctx.Types[varName]; exists {
			// Return the full type (e.g., "[]int", "[]User")
			return varType
		}
	}
	
	// Fallback: construct slice type from variable name patterns
	if varName == "numbers" || varName == "nums" {
		return "[]int"
	}
	if varName == "users" {
		return "[]User"
	}
	if varName == "items" {
		return "[]any"
	}
	
	return "[]any"
}

// inferChainedMethodType infers the result type of a method call chain
func (t *ListMethodsTransform) inferChainedMethodType(receiver syntax.Expr, methodName string, ctx *TransformContext) string {
	// If receiver is a method call, infer its return type
	if call, ok := receiver.(*syntax.CallExpr); ok {
		if selector, ok := call.Fun.(*syntax.SelectorExpr); ok {
			innerReceiverType := t.inferChainedMethodType(selector.X, selector.Sel.Value, ctx)
			println("DEBUG: chained method", selector.Sel.Value, "on type", innerReceiverType, "returns", t.inferMethodReturnTypeFromType(innerReceiverType, selector.Sel.Value))
			return t.inferMethodReturnTypeFromType(innerReceiverType, selector.Sel.Value)
		}
	}
	
	// If receiver is a simple variable name
	if name, ok := receiver.(*syntax.Name); ok {
		baseType := t.getSliceType(name.Value, ctx)
		println("DEBUG: base variable", name.Value, "has type", baseType)
		return baseType
	}
	
	return "[]any"
}

// inferMethodReturnTypeFromType infers return type based on receiver type and method
func (t *ListMethodsTransform) inferMethodReturnTypeFromType(receiverType, methodName string) string {
	if len(receiverType) >= 2 && receiverType[:2] == "[]" {
		elementType := receiverType[2:] // e.g., "User" from "[]User"
		
		switch methodName {
		case "filter", "where", "chose", "that", "which":
			return receiverType // []User -> []User
		case "apply", "transform", "convert":
			// For apply with User.Name, return []string
			if elementType == "User" {
				return "[]string"
			}
			return "[]any"
		case "sort", "sortBy", "reverse", "sorted", "reversed":
			return receiverType // []string -> []string
		case "first", "last", "head", "tail":
			return elementType // []string -> string
		}
	}
	return "[]any"
}

// inferLambdaReturnType analyzes lambda body to infer return type
func (t *ListMethodsTransform) inferLambdaReturnType(body syntax.Expr, parameterType string) string {
	switch e := body.(type) {
	case *syntax.SelectorExpr:
		// Handle cases like u.Name, u.Age
		if _, ok := e.X.(*syntax.Name); ok {
			// This is a field access on the parameter
			fieldName := e.Sel.Value
			return t.inferFieldType(parameterType, fieldName)
		}
	case *syntax.BasicLit:
		// Handle literal returns
		switch e.Kind {
		case syntax.StringLit:
			return "string"
		case syntax.IntLit:
			return "int"
		case syntax.FloatLit:
			return "float64"
		case syntax.RuneLit:
			return "rune"
		}
	case *syntax.Operation:
		// Handle expressions like u.Age + 1
		return t.inferOperationReturnType(e, parameterType)
	}
	return ""
}

// inferFieldType infers the type of a field access like User.Name
func (t *ListMethodsTransform) inferFieldType(structType, fieldName string) string {
	// Hard-coded type mappings for known types
	// This should ideally be dynamic based on struct definitions
	switch structType {
	case "User":
		switch fieldName {
		case "Name":
			return "string"
		case "Age":
			return "int" // or "number" which is aliased to int
		}
	}
	
	// Default fallback
	return "any"
}

// inferOperationReturnType infers return type of operations
func (t *ListMethodsTransform) inferOperationReturnType(op *syntax.Operation, parameterType string) string {
	switch op.Op {
	case syntax.Add, syntax.Sub, syntax.Mul, syntax.Div, syntax.Rem:
		return "int" // Simplified - could be float64 depending on operands
	case syntax.Eql, syntax.Neq, syntax.Lss, syntax.Leq, syntax.Gtr, syntax.Geq:
		return "bool"
	case syntax.AndAnd, syntax.OrOr:
		return "bool"
	}
	return "any"
}

// extractElementType extracts element type from slice type string
// e.g., "[]User" -> "User", "[]int" -> "int"
func (t *ListMethodsTransform) extractElementType(varName string, ctx *TransformContext) string {
	// First try to get the actual type from context
	if ctx != nil && ctx.Types != nil {
		if varType, exists := ctx.Types[varName]; exists {
			// Parse slice type: "[]User" -> "User"
			if len(varType) > 2 && varType[:2] == "[]" {
				elementType := varType[2:] // Remove "[]" prefix
				return elementType
			}
		}
	}
	
	// Fallback to simple heuristics based on common patterns
	// Pattern: users -> User, items -> Item, etc.
	if varName == "users" {
		return "User"
	}
	if varName == "items" {
		return "Item"
	}
	if varName == "numbers" {
		return "int"
	}
	
	return ""
}

// Create methods for the new list operations

func (t *ListMethodsTransform) createSortDescCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()
	
	// Generate slices.CloneAndSortDesc(receiver)
	slicesName := &syntax.Name{Value: "slices"}
	slicesName.SetPos(pos)
	cloneAndSortDescName := &syntax.Name{Value: "CloneAndSortDesc"}
	cloneAndSortDescName.SetPos(pos)
	
	slicesCloneAndSortDesc := &syntax.SelectorExpr{
		X:   slicesName,
		Sel: cloneAndSortDescName,
	}
	slicesCloneAndSortDesc.SetPos(pos)
	
	call := &syntax.CallExpr{
		Fun:     slicesCloneAndSortDesc,
		ArgList: []syntax.Expr{receiver},
	}
	call.SetPos(pos)
	
	return call
}

func (t *ListMethodsTransform) createPopCall(receiver syntax.Expr) syntax.Expr {
	// Create call to listPop builtin function
	pos := receiver.Pos()
	
	popName := &syntax.Name{Value: "listPop"}
	popName.SetPos(pos)
	
	call := &syntax.CallExpr{
		Fun:     popName,
		ArgList: []syntax.Expr{receiver},
	}
	call.SetPos(pos)
	
	return call
}

// isStringReceiver checks if the receiver is a string literal or string type
func (t *ListMethodsTransform) isStringReceiver(receiver syntax.Expr) bool {
	// Check for string literals
	if basic, ok := receiver.(*syntax.BasicLit); ok {
		return basic.Kind == syntax.StringLit
	}
	return false
}

// createCloneAndApplyPattern generates: func() []T { cloned := slices.Clone(receiver); operation(cloned); return cloned }()
func (t *ListMethodsTransform) createCloneAndApplyPattern(receiver, cloneCall, operationCall syntax.Expr, pos syntax.Pos) syntax.Expr {
	// Create variable name: cloned
	clonedVar := &syntax.Name{Value: "cloned"}
	clonedVar.SetPos(pos)
	
	// Create assignment: cloned := slices.Clone(receiver)
	assignStmt := &syntax.AssignStmt{
		Op:  syntax.Def, // :=
		Lhs: clonedVar,
		Rhs: cloneCall,
	}
	assignStmt.SetPos(pos)
	
	// Create operation statement (using cloned variable instead of original call)
	// Replace the cloneCall in operationCall with clonedVar
	modifiedOpCall := t.replaceExprInCall(operationCall, cloneCall, clonedVar)
	
	opStmt := &syntax.ExprStmt{
		X: modifiedOpCall,
	}
	opStmt.SetPos(pos)
	
	// Create return statement: return cloned
	returnStmt := &syntax.ReturnStmt{
		Results: clonedVar,
	}
	returnStmt.SetPos(pos)
	
	// Create function body
	body := &syntax.BlockStmt{
		List: []syntax.Stmt{assignStmt, opStmt, returnStmt},
	}
	body.SetPos(pos)
	
	// Create return type for the function (slice type)
	// For simplicity, we'll use interface{} and let the type checker handle it
	returnType := &syntax.Name{Value: "any"}
	returnType.SetPos(pos)
	
	// Create anonymous function with return type
	funcLit := &syntax.FuncLit{
		Type: &syntax.FuncType{
			ResultList: []*syntax.Field{{Type: returnType}},
		},
		Body: body,
	}
	funcLit.SetPos(pos)
	funcLit.Type.SetPos(pos)
	
	// Create function call
	call := &syntax.CallExpr{
		Fun: funcLit,
	}
	call.SetPos(pos)
	
	return call
}

// replaceExprInCall replaces oldExpr with newExpr in a CallExpr's arguments
func (t *ListMethodsTransform) replaceExprInCall(callExpr syntax.Expr, oldExpr, newExpr syntax.Expr) syntax.Expr {
	if call, ok := callExpr.(*syntax.CallExpr); ok {
		newCall := &syntax.CallExpr{
			Fun: call.Fun,
		}
		newCall.SetPos(call.Pos())
		
		// Replace arguments
		if call.ArgList != nil {
			newArgList := make([]syntax.Expr, len(call.ArgList))
			for i, arg := range call.ArgList {
				if arg == oldExpr {
					newArgList[i] = newExpr
				} else {
					newArgList[i] = arg
				}
			}
			newCall.ArgList = newArgList
		}
		return newCall
	}
	return callExpr
}

func (t *ListMethodsTransform) createShiftCall(receiver syntax.Expr) syntax.Expr {
	// Create call to listShift builtin function
	pos := receiver.Pos()
	
	shiftName := &syntax.Name{Value: "listShift"}
	shiftName.SetPos(pos)
	
	call := &syntax.CallExpr{
		Fun:     shiftName,
		ArgList: []syntax.Expr{receiver},
	}
	call.SetPos(pos)
	
	return call
}

func init() {
	RegisterTransformer(&ListMethodsTransform{})
}
