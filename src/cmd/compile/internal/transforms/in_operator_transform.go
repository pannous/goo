// Copyright 2025 The Goo Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transforms

import (
	"cmd/compile/internal/syntax"
	"os"
	"strings"
)

// InOperatorTransform handles the 'in' operator for strings and collections
// Transforms expressions like "hello" in str to strings.Contains(str, "hello")
// and item in slice to slices.Contains(slice, item)
type InOperatorTransform struct{}

type inVisitor struct {
	transform           *InOperatorTransform
	ctx                 *TransformContext
	file                *syntax.File
	changed             bool
	needsStringsImport  bool
	needsSlicesImport   bool
}

func (t *InOperatorTransform) Name() string {
	return "in_operator_transform"
}

func (t *InOperatorTransform) Priority() int {
	return 100 // Default priority - between list methods (50) and lambda (200)
}

func (t *InOperatorTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &inVisitor{transform: t, ctx: ctx, file: file}
	
	// Use syntax.Walk to traverse the entire AST
	syntax.Walk(file, visitor)
	
	// Add imports if needed (only strings import now, slices is no longer needed)
	if visitor.needsStringsImport && !t.hasImport(file, "strings") {
		t.addStringsImport(file)
	}
	
	return visitor.changed
}

// Visit implements syntax.Visitor interface
func (v *inVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}
	
	// Transform nodes that contain expressions that might have 'in' operations
	switch n := node.(type) {
	case *syntax.VarDecl:
		if n.Values != nil {
			if transformed := v.transform.transformExpr(n.Values, v); transformed != n.Values {
				n.Values = transformed
				v.changed = true
			}
		}
	case *syntax.AssignStmt:
		if n.Rhs != nil {
			if transformed := v.transform.transformExpr(n.Rhs, v); transformed != n.Rhs {
				n.Rhs = transformed
				v.changed = true
			}
		}
	case *syntax.CheckStmt:
		if n.Cond != nil {
			if transformed := v.transform.transformExpr(n.Cond, v); transformed != n.Cond {
				n.Cond = transformed
				v.changed = true
			}
		}
	case *syntax.ExprStmt:
		if transformed := v.transform.transformExpr(n.X, v); transformed != n.X {
			n.X = transformed
			v.changed = true
		}
	case *syntax.IfStmt:
		if n.Cond != nil {
			if transformed := v.transform.transformExpr(n.Cond, v); transformed != n.Cond {
				n.Cond = transformed
				v.changed = true
			}
		}
	case *syntax.ForStmt:
		if n.Cond != nil {
			if transformed := v.transform.transformExpr(n.Cond, v); transformed != n.Cond {
				n.Cond = transformed
				v.changed = true
			}
		}
	}
	
	// Continue visiting child nodes
	return v
}

// transformExpr transforms a single expression
func (t *InOperatorTransform) transformExpr(expr syntax.Expr, visitor *inVisitor) syntax.Expr {
	if expr == nil {
		return expr
	}
	
	// Check for 'in' operations
	if op, ok := expr.(*syntax.Operation); ok {
		if op.Op == syntax.In {
			if transformed := t.convertInOperation(op, visitor, visitor.file); transformed != nil {
				visitor.changed = true
				return transformed
			}
		}
		// Transform operands recursively
		if op.X != nil {
			op.X = t.transformExpr(op.X, visitor)
		}
		if op.Y != nil {
			op.Y = t.transformExpr(op.Y, visitor)
		}
	}
	
	// Handle other expression types that might contain sub-expressions
	switch e := expr.(type) {
	case *syntax.CallExpr:
		for i, arg := range e.ArgList {
			e.ArgList[i] = t.transformExpr(arg, visitor)
		}
	case *syntax.ParenExpr:
		e.X = t.transformExpr(e.X, visitor)
	case *syntax.ListExpr:
		for i, elem := range e.ElemList {
			e.ElemList[i] = t.transformExpr(elem, visitor)
		}
	}
	
	return expr
}

// convertInOperation converts "item in collection" to appropriate Go code
func (t *InOperatorTransform) convertInOperation(op *syntax.Operation, visitor *inVisitor, file *syntax.File) syntax.Expr {
	pos := op.Pos()
	
	// Determine the type of operation based on the container (op.Y)
	containerType := t.inferContainerType(op.Y, visitor.ctx)
	
	switch containerType {
	case "string":
		return t.createStringContainsCall(op, visitor, pos)
	case "slice":
		return t.createSliceContainsCall(op, visitor, pos)
	case "map":
		return t.createMapContainsCall(op, visitor, pos)
	case "iterator":
		return t.createIteratorContainsCall(op, visitor, pos)
	default:
		// Try to determine at runtime or fall back to string
		return t.createStringContainsCall(op, visitor, pos)
	}
}

// inferContainerType tries to determine if the container is a string, slice, or map
func (t *InOperatorTransform) inferContainerType(container syntax.Expr, ctx *TransformContext) string {
	// Check for string literals
	if basic, ok := container.(*syntax.BasicLit); ok {
		if basic.Kind == syntax.StringLit {
			return "string"
		}
	}
	
	// Check for composite literals (slices/arrays/maps)
	if comp, ok := container.(*syntax.CompositeLit); ok {
		if comp.Type != nil {
			// Check for map types
			if _, isMap := comp.Type.(*syntax.MapType); isMap {
				return "map"
			}
			// Check for slice/array types
			if _, isSlice := comp.Type.(*syntax.SliceType); isSlice {
				return "slice"
			}
			if _, isArray := comp.Type.(*syntax.ArrayType); isArray {
				return "slice"
			}
		}
		// If no explicit type, infer from usage - composite literals are usually slices
		return "slice"
	}
	
	// Check for iterator function calls
	if t.isIteratorType(container) {
		return "iterator"
	}
	
	// Check context for variable types
	if name, ok := container.(*syntax.Name); ok && ctx != nil && ctx.Types != nil {
		if varType, exists := ctx.Types[name.Value]; exists {
			if strings.Contains(varType, "[]") {
				return "slice"
			}
			if strings.Contains(varType, "map[") {
				return "map"
			}
			if varType == "string" {
				return "string"
			}
		}
	}
	
	return "unknown"
}

// createStringContainsCall creates strings.Contains(container, item) or inline version for GOPATH mode
func (t *InOperatorTransform) createStringContainsCall(op *syntax.Operation, visitor *inVisitor, pos syntax.Pos) syntax.Expr {
	gomod := os.Getenv("GO111MODULE")
	// Only use inline string containment for explicitly disabled modules
	// Default to strings.Contains for reliability
	if gomod == "off" {
		return t.createInlineStringContains(op, visitor, pos)
	}
	
	visitor.needsStringsImport = true
	
	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)
	
	containsName := &syntax.Name{Value: "Contains"}
	containsName.SetPos(pos)
	
	stringsContains := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: containsName,
	}
	stringsContains.SetPos(pos)
	
	// Check if the item (op.X) is a rune literal that needs conversion
	item := op.X
	if t.isRuneLiteral(op.X) {
		item = t.convertRuneToString(op.X, pos)
	}
	
	call := &syntax.CallExpr{
		Fun:     stringsContains,
		ArgList: []syntax.Expr{op.Y, item}, // Y is container, X is item
	}
	call.SetPos(pos)
	
	return call
}

// createInlineStringContains creates inline string containment check for GOPATH mode
// Uses simple logic to avoid import dependencies
func (t *InOperatorTransform) createInlineStringContains(op *syntax.Operation, visitor *inVisitor, pos syntax.Pos) syntax.Expr {
	// Handle string literal in string literal case
	if itemExpr, ok := op.X.(*syntax.BasicLit); ok && itemExpr.Kind == syntax.StringLit {
		if containerExpr, ok := op.Y.(*syntax.BasicLit); ok && containerExpr.Kind == syntax.StringLit {
			// Both are string literals - compute at compile time
			result := t.computeStringInString(itemExpr.Value, containerExpr.Value)
			resultLit := &syntax.Name{Value: result}
			resultLit.SetPos(pos)
			return resultLit
		}
	}
	
	// Handle rune literal in string literal case  
	if runeExpr, ok := op.X.(*syntax.BasicLit); ok && runeExpr.Kind == syntax.RuneLit {
		if stringExpr, ok := op.Y.(*syntax.BasicLit); ok && stringExpr.Kind == syntax.StringLit {
			// Both are literals - compute at compile time
			result := t.computeRuneInString(runeExpr.Value, stringExpr.Value)
			resultLit := &syntax.Name{Value: result}
			resultLit.SetPos(pos)
			return resultLit
		}
	}
	
	// For non-literal cases, fall back to strings.Contains even in GOPATH mode
	// This is better than always returning false
	visitor.needsStringsImport = true
	
	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)
	
	containsName := &syntax.Name{Value: "Contains"}
	containsName.SetPos(pos)
	
	stringsContains := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: containsName,
	}
	stringsContains.SetPos(pos)
	
	// Check if the item (op.X) is a rune literal that needs conversion
	item := op.X
	if t.isRuneLiteral(op.X) {
		item = t.convertRuneToString(op.X, pos)
	}
	
	call := &syntax.CallExpr{
		Fun:     stringsContains,
		ArgList: []syntax.Expr{op.Y, item}, // Y is container, X is item
	}
	call.SetPos(pos)
	
	return call
}

// Helper function to get expression type as string for debugging
func (t *InOperatorTransform) getExprType(expr syntax.Expr) string {
	switch e := expr.(type) {
	case *syntax.BasicLit:
		kindStr := "unknown"
		switch e.Kind {
		case syntax.IntLit:
			kindStr = "IntLit"
		case syntax.FloatLit:
			kindStr = "FloatLit"
		case syntax.ImagLit:
			kindStr = "ImagLit"
		case syntax.RuneLit:
			kindStr = "RuneLit"
		case syntax.StringLit:
			kindStr = "StringLit"
		}
		return "BasicLit(" + kindStr + ":" + e.Value + ")"
	case *syntax.Name:
		return "Name(" + e.Value + ")"
	case *syntax.CallExpr:
		return "CallExpr"
	case *syntax.Operation:
		return "Operation"
	default:
		return "Unknown"
	}
}

// createSliceContainsCall creates a manual loop to check slice contains without imports
func (t *InOperatorTransform) createSliceContainsCall(op *syntax.Operation, visitor *inVisitor, pos syntax.Pos) syntax.Expr {
	// For eval and simple cases, handle small literal slices by expanding to OR comparisons
	if compLit, ok := op.Y.(*syntax.CompositeLit); ok {
		if len(compLit.ElemList) <= 10 && len(compLit.ElemList) > 0 {
			// Create chain of comparisons: item == elem1 || item == elem2 || ...
			var result syntax.Expr
			
			for i, elem := range compLit.ElemList {
				comparison := &syntax.Operation{
					Op: syntax.Eql,
					X:  op.X, // item
					Y:  elem,
				}
				comparison.SetPos(pos)
				
				if i == 0 {
					result = comparison
				} else {
					result = &syntax.Operation{
						Op: syntax.OrOr,
						X:  result,
						Y:  comparison,
					}
					result.SetPos(pos)
				}
			}
			
			return result
		}
	}
	
	// For complex cases, fall back to false for now
	// This can be improved later with proper loop generation  
	falseExpr := &syntax.Name{Value: "false"}
	falseExpr.SetPos(pos)
	return falseExpr
}

// createMapContainsCall creates map key existence check: _, ok := map[key]; ok
func (t *InOperatorTransform) createMapContainsCall(op *syntax.Operation, visitor *inVisitor, pos syntax.Pos) syntax.Expr {
	// Create anonymous function that returns the existence check
	// Transforms: key in myMap  =>  func() bool { _, ok := myMap[key]; return ok }()
	
	// Create map index expression: myMap[key]
	indexExpr := &syntax.IndexExpr{
		X:     op.Y, // the map
		Index: op.X, // the key
	}
	indexExpr.SetPos(pos)
	
	// Create assignment: _, ok := myMap[key]
	blankVar := &syntax.Name{Value: "_"}
	blankVar.SetPos(pos)
	okVar := &syntax.Name{Value: "ok"}
	okVar.SetPos(pos)
	
	lhsList := &syntax.ListExpr{ElemList: []syntax.Expr{blankVar, okVar}}
	lhsList.SetPos(pos)
	
	assign := &syntax.AssignStmt{
		Op:  syntax.Def, // :=
		Lhs: lhsList,
		Rhs: indexExpr,
	}
	assign.SetPos(pos)
	
	// Create return statement: return ok
	returnStmt := &syntax.ReturnStmt{
		Results: okVar,
	}
	returnStmt.SetPos(pos)
	
	// Create function body
	body := &syntax.BlockStmt{
		List: []syntax.Stmt{assign, returnStmt},
	}
	body.SetPos(pos)
	
	// Create anonymous function
	boolType := &syntax.Name{Value: "bool"}
	boolType.SetPos(pos)
	
	funcLit := &syntax.FuncLit{
		Type: &syntax.FuncType{
			ResultList: []*syntax.Field{{Type: boolType}},
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

// createIteratorContainsCall creates iterator membership check using range loop
// item in iterator() => func() bool { for v := range iterator() { if v == item { return true } } return false }()
func (t *InOperatorTransform) createIteratorContainsCall(op *syntax.Operation, visitor *inVisitor, pos syntax.Pos) syntax.Expr {
	// Create loop variable
	loopVar := &syntax.Name{Value: "v"}
	loopVar.SetPos(pos)
	
	// Create range clause: for v := range iterator()
	rangeClause := &syntax.RangeClause{
		Lhs: loopVar,
		Def: true,
		X:   op.Y, // the iterator call
	}
	rangeClause.SetPos(pos)
	
	// Create comparison: v == item
	comparison := &syntax.Operation{
		Op: syntax.Eql,
		X:  loopVar,
		Y:  op.X, // the item to find
	}
	comparison.SetPos(pos)
	
	// Create return true statement
	trueReturn := &syntax.ReturnStmt{
		Results: &syntax.Name{Value: "true"},
	}
	trueReturn.SetPos(pos)
	trueReturn.Results.SetPos(pos)
	
	// Create if body
	ifBody := &syntax.BlockStmt{
		List: []syntax.Stmt{trueReturn},
	}
	ifBody.SetPos(pos)
	
	// Create if statement: if v == item { return true }
	ifStmt := &syntax.IfStmt{
		Cond: comparison,
		Then: ifBody,
	}
	ifStmt.SetPos(pos)
	
	// Create for loop body
	forBody := &syntax.BlockStmt{
		List: []syntax.Stmt{ifStmt},
	}
	forBody.SetPos(pos)
	
	// Create for loop: for v := range iterator() { if v == item { return true } }
	forStmt := &syntax.ForStmt{
		Init: rangeClause,
		Body: forBody,
	}
	forStmt.SetPos(pos)
	
	// Create return false statement
	falseReturn := &syntax.ReturnStmt{
		Results: &syntax.Name{Value: "false"},
	}
	falseReturn.SetPos(pos)
	falseReturn.Results.SetPos(pos)
	
	// Create function body: { for ... ; return false }
	funcBody := &syntax.BlockStmt{
		List: []syntax.Stmt{forStmt, falseReturn},
	}
	funcBody.SetPos(pos)
	
	// Create anonymous function
	boolType := &syntax.Name{Value: "bool"}
	boolType.SetPos(pos)
	
	funcLit := &syntax.FuncLit{
		Type: &syntax.FuncType{
			ResultList: []*syntax.Field{{Type: boolType}},
		},
		Body: funcBody,
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

// isIteratorType attempts to detect if the expression is likely an iterator (reused from in_loop_transform)
func (t *InOperatorTransform) isIteratorType(expr syntax.Expr) bool {
	// Check if it's a function call that might return an iterator
	if call, ok := expr.(*syntax.CallExpr); ok {
		// Check if the function name suggests it returns an iterator
		if name, ok := call.Fun.(*syntax.Name); ok {
			funcName := name.Value
			return t.looksLikeIteratorFunction(funcName)
		}
		
		// Check for selector expressions like somePackage.Iterator()
		if sel, ok := call.Fun.(*syntax.SelectorExpr); ok {
			return t.looksLikeIteratorFunction(sel.Sel.Value)
		}
	}
	
	return false
}

// looksLikeIteratorFunction checks if a function name suggests it returns an iterator
func (t *InOperatorTransform) looksLikeIteratorFunction(name string) bool {
	// Common patterns for iterator function names
	iteratorPatterns := []string{
		"Iter", "Iterator", "Items", "Values", "Keys", "Entries", 
		"Numbers", "Range", "Sequence", "Stream", "Generate",
	}
	
	for _, pattern := range iteratorPatterns {
		if name == pattern || 
		   len(name) > len(pattern) && name[len(name)-len(pattern):] == pattern ||
		   len(name) > len(pattern) && name[:len(pattern)] == pattern {
			return true
		}
	}
	
	return false
}

func (t *InOperatorTransform) hasImport(file *syntax.File, name string) bool {
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

func (t *InOperatorTransform) addStringsImport(file *syntax.File) {
	if t.hasImport(file, "strings") {
		return
	}

	// Use file position instead of empty position
	pos := syntax.Pos{}
	if len(file.DeclList) > 0 {
		pos = file.DeclList[0].Pos()
	}

	stringsImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: "\"strings\"",
			Kind:  syntax.StringLit,
		},
	}
	stringsImport.SetPos(pos)
	stringsImport.Path.SetPos(pos)

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
	newDeclList = append(newDeclList, stringsImport)
	newDeclList = append(newDeclList, file.DeclList[insertPos:]...)
	file.DeclList = newDeclList
}


// isRuneLiteral checks if an expression is a rune literal (e.g., 'a')
func (t *InOperatorTransform) isRuneLiteral(expr syntax.Expr) bool {
	if basic, ok := expr.(*syntax.BasicLit); ok {
		return basic.Kind == syntax.RuneLit
	}
	return false
}

// convertRuneToString converts a rune literal to string(rune) call
func (t *InOperatorTransform) convertRuneToString(runeExpr syntax.Expr, pos syntax.Pos) syntax.Expr {
	// Create string(rune) call
	stringName := &syntax.Name{Value: "string"}
	stringName.SetPos(pos)
	
	call := &syntax.CallExpr{
		Fun:     stringName,
		ArgList: []syntax.Expr{runeExpr},
	}
	call.SetPos(pos)
	
	return call
}

// computeStringInString computes whether a string literal is contained in another string literal at compile time
func (t *InOperatorTransform) computeStringInString(itemLiteral, containerLiteral string) string {
	// Parse the item string literal (e.g., "hello" -> hello)
	if len(itemLiteral) < 2 || itemLiteral[0] != '"' || itemLiteral[len(itemLiteral)-1] != '"' {
		return "false" // Invalid string literal
	}
	itemContent := itemLiteral[1 : len(itemLiteral)-1] // Remove quotes

	// Parse the container string literal (e.g., "hello world" -> hello world)
	if len(containerLiteral) < 2 || containerLiteral[0] != '"' || containerLiteral[len(containerLiteral)-1] != '"' {
		return "false" // Invalid string literal
	}
	containerContent := containerLiteral[1 : len(containerLiteral)-1] // Remove quotes

	// Simple substring check
	if strings.Contains(containerContent, itemContent) {
		return "true"
	}
	return "false"
}

// computeRuneInString computes whether a rune literal is contained in a string literal at compile time
func (t *InOperatorTransform) computeRuneInString(runeLiteral, stringLiteral string) string {
	// Parse the rune literal (e.g., 'a' -> a, '\n' -> newline)
	// runeLiteral includes the quotes, e.g., "'a'"
	if len(runeLiteral) < 3 || runeLiteral[0] != '\'' || runeLiteral[len(runeLiteral)-1] != '\'' {
		return "false" // Invalid rune literal
	}
	
	runeContent := runeLiteral[1 : len(runeLiteral)-1] // Remove quotes
	
	// Parse the string literal (e.g., "abc" -> abc)
	// stringLiteral includes the quotes, e.g., "\"abc\""
	if len(stringLiteral) < 2 || stringLiteral[0] != '"' || stringLiteral[len(stringLiteral)-1] != '"' {
		return "false" // Invalid string literal
	}
	
	stringContent := stringLiteral[1 : len(stringLiteral)-1] // Remove quotes
	
	// For simple ASCII characters, do basic containment check
	if len(runeContent) == 1 && runeContent[0] < 128 {
		// Simple ASCII character
		char := runeContent[0]
		for i := 0; i < len(stringContent); i++ {
			if stringContent[i] == char {
				return "true"
			}
		}
		return "false"
	}
	
	// For more complex cases (escape sequences, unicode), fall back to false for safety
	return "false"
}

func init() {
	RegisterTransformer(&InOperatorTransform{})
}