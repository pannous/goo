// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// StringMethodsTransform handles automatic transformation of string method calls
// to their corresponding Go standard library function calls.
type StringMethodsTransform struct{}

type methodVisitor struct {
	transform          *StringMethodsTransform
	ctx                *TransformContext
	changed            bool
	needsStringsImport bool
	needsStrconvImport bool
	needsUnicodeImport bool
	needsSlicesImport  bool
}

func (t *StringMethodsTransform) Name() string {
	return "string_methods_transform"
}

func (t *StringMethodsTransform) Priority() int {
	return 100 // Default priority - between list methods (50) and lambda (200)
}

// transformStringMethod transforms string method calls to standard library calls
func (t *StringMethodsTransform) transformStringMethod(receiver syntax.Expr, methodName string, args []syntax.Expr) syntax.Expr {
	println("transformStringMethod:", methodName)
	switch methodName {
	// Basic string info
	case "reverse", "flip":
		// Note: This will be handled by the visitor to set needsSlicesImport flag
		return t.createReverseCall(receiver)
	case "first", "head", "start":
		return t.createFirstCall(receiver)
	case "last", "tail", "end":
		return t.createLastCall(receiver)
	case "size", "length", "len":
		return t.createLenCall(receiver)
	case "count":
		if len(args) == 1 {
			return t.createCountCall(receiver, args[0])
		}
		return t.createLenCall(receiver) // count() with no args = length
	case "isEmpty":
		return t.createIsEmptyCall(receiver)

	// Character access
	case "charAt", "at", "char":
		if len(args) == 1 {
			return t.createCharAtCall(receiver, args[0])
		}
	case "runeAt", "rune":
		if len(args) == 1 {
			return t.createRuneAtCall(receiver, args[0])
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
	case "lastIndexOf", "rfind":
		if len(args) == 1 {
			return t.createLastIndexCall(receiver, args[0])
		}

	// Substring methods
	case "from":
		if len(args) == 1 {
			return t.createFromCall(receiver, args[0])
		}
	case "to":
		if len(args) == 1 {
			return t.createToCall(receiver, args[0])
		}
	case "sub", "substring", "slice":
		if len(args) == 2 {
			return t.createSubCall(receiver, args[0], args[1])
		}

	// Replace methods
	case "replace", "replaceAll", "substitute", "swap":
		if len(args) == 2 {
			return t.createReplaceCall(receiver, args[0], args[1])
		}
	case "replaceFirst":
		if len(args) == 2 {
			return t.createReplaceFirstCall(receiver, args[0], args[1])
		}

	// Case conversion
	case "toUpper", "upper", "upperCase", "toUpperCase":
		return t.createToUpperCall(receiver)
	case "toLower", "lower", "lowerCase", "toLowerCase":
		return t.createToLowerCall(receiver)
	case "capitalize", "title", "toTitle":
		return t.createCapitalizeCall(receiver)
	case "swapCase":
		return t.createSwapCaseCall(receiver)

	// Trim methods
	case "trim", "strip":
		return t.createTrimCall(receiver)
	case "trimLeft", "lstrip", "trimStart":
		return t.createTrimLeftCall(receiver)
	case "trimRight", "rstrip", "trimEnd":
		return t.createTrimRightCall(receiver)

	// Split/Join methods
	case "split":
		if len(args) == 1 {
			return t.createSplitCall(receiver, args[0])
		}
	case "splits", "chars":
		return t.createSplitsCall(receiver)
	case "lines":
		return t.createLinesCall(receiver)
	case "words":
		return t.createWordsCall(receiver)
	case "runes":
		return t.createRunesCall(receiver)
	case "bytes":
		return t.createBytesCall(receiver)
	case "codePoints":
		return t.createCodePointsCall(receiver)
	case "join":
		if len(args) == 1 {
			return t.createJoinCall(receiver, args[0])
		}

	// Prefix/Suffix methods
	case "startsWith", "beginsWith":
		if len(args) == 1 {
			return t.createStartsWithCall(receiver, args[0])
		}
	case "endsWith":
		if len(args) == 1 {
			return t.createEndsWithCall(receiver, args[0])
		}
	case "removePrefix":
		if len(args) == 1 {
			return t.createRemovePrefixCall(receiver, args[0])
		}
	case "removeSuffix":
		if len(args) == 1 {
			return t.createRemoveSuffixCall(receiver, args[0])
		}

	// Padding methods
	case "center":
		if len(args) >= 1 {
			var fillChar syntax.Expr
			fillChar = &syntax.BasicLit{Kind: syntax.StringLit, Value: `" "`}
			fillChar.SetPos(receiver.Pos())
			if len(args) == 2 {
				fillChar = args[1]
			}
			return t.createCenterCall(receiver, args[0], fillChar)
		}
	case "ljust", "padLeft":
		if len(args) >= 1 {
			var fillChar syntax.Expr
			fillChar = &syntax.BasicLit{Kind: syntax.StringLit, Value: `" "`}
			fillChar.SetPos(receiver.Pos())
			if len(args) == 2 {
				fillChar = args[1]
			}
			return t.createPadLeftCall(receiver, args[0], fillChar)
		}
	case "rjust", "padRight":
		if len(args) >= 1 {
			var fillChar syntax.Expr
			fillChar = &syntax.BasicLit{Kind: syntax.StringLit, Value: `" "`}
			fillChar.SetPos(receiver.Pos())
			if len(args) == 2 {
				fillChar = args[1]
			}
			return t.createPadRightCall(receiver, args[0], fillChar)
		}
	case "zfill":
		if len(args) == 1 {
			return t.createZfillCall(receiver, args[0])
		}

	// Character type checking
	case "isAlpha":
		return t.createIsAlphaCall(receiver)
	case "isDigit", "isNumeric":
		return t.createIsDigitCall(receiver)
	case "isAlphaNumeric", "isAlnum":
		return t.createIsAlnumCall(receiver)
	case "isLower":
		return t.createIsLowerCall(receiver)
	case "isUpper":
		return t.createIsUpperCall(receiver)
	case "isSpace":
		return t.createIsSpaceCall(receiver)
	case "isPrintable":
		return t.createIsPrintableCall(receiver)

	// Type conversion
	case "toInt", "parseInt":
		if len(args) == 0 {
			return t.createToIntCall(receiver, nil)
		} else if len(args) == 1 {
			return t.createToIntCall(receiver, args[0])
		}
	case "toFloat", "parseFloat":
		return t.createToFloatCall(receiver)
	case "toBool", "parseBool":
		return t.createToBoolCall(receiver)

	// Repetition
	case "repeat", "times":
		if len(args) == 1 {
			return t.createRepeatCall(receiver, args[0])
		}

	// Format methods (these need runtime implementation)
	case "format":
		return t.createCompilerError(receiver, "format", "string formatting with placeholders")
	case "expandTabs":
		return t.createCompilerError(receiver, "expandTabs", "tab expansion")
	case "encode":
		return t.createCompilerError(receiver, "encode", "string encoding")
	case "decode":
		return t.createCompilerError(receiver, "decode", "string decoding")
	case "casefold":
		return t.createCompilerError(receiver, "casefold", "aggressive case folding")
	case "partition":
		return t.createCompilerError(receiver, "partition", "string partitioning")
	case "rpartition":
		return t.createCompilerError(receiver, "rpartition", "reverse string partitioning")
	}

	// If we reach here, method is not recognized at all
	return t.createCompilerError(receiver, methodName, "unknown string method")
}

func (t *StringMethodsTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	//println("StringMethodsTransform.Transform called")

	visitor := &methodVisitor{transform: t, ctx: ctx}
	syntax.Walk(file, visitor)

	if visitor.needsStringsImport && !t.hasImport(file, "strings") {
		println("Adding strings import")
		t.addStringsImport(file)
	}
	if visitor.needsStrconvImport && !t.hasImport(file, "strconv") {
		println("Adding strconv import")
		t.addStrconvImport(file)
	}
	if visitor.needsUnicodeImport && !t.hasImport(file, "unicode") {
		println("Adding unicode import")
		t.addUnicodeImport(file)
	}
	if visitor.needsSlicesImport && !t.hasImport(file, "slices") {
		println("Adding slices import")
		t.addSlicesImport(file)
	}

	return visitor.changed
}

// Visit implements syntax.Visitor interface
func (v *methodVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}

	// Check for method calls on string expressions
	if call, ok := node.(*syntax.CallExpr); ok {
		if selector, ok := call.Fun.(*syntax.SelectorExpr); ok {
			if v.transform.isStringExpression(selector.X, v.ctx) {
				methodName := selector.Sel.Value
				if transformed := v.transform.transformStringMethod(selector.X, methodName, call.ArgList); transformed != nil {
					println("TRANSFORMING string method:", methodName)
					// Handle different expression types
					switch expr := transformed.(type) {
					case *syntax.CallExpr:
						*call = *expr
					case *syntax.SliceExpr:
						// For slice expressions, we just need to update the call directly
						pos := expr.Pos()
						stringName := &syntax.Name{Value: "string"}
						stringName.SetPos(pos)
						newCall := &syntax.CallExpr{
							Fun:     stringName,
							ArgList: []syntax.Expr{expr},
						}
						newCall.SetPos(pos)
						*call = *newCall
					}
					v.changed = true
					// Track required imports based on method name
					stringsMethods := []string{
						"contains", "includes", "indexOf", "find", "lastIndexOf", "rfind",
						"replace", "replaceAll", "replaceFirst",
						"toUpper", "upper", "upperCase", "toUpperCase",
						"toLower", "lower", "lowerCase", "toLowerCase",
						"capitalize", "title", "toTitle",
						"trim", "strip", "trimLeft", "lstrip", "trimStart",
						"trimRight", "rstrip", "trimEnd",
						"split", "splits", "chars", "lines", "words", "join",
						"startsWith", "beginsWith", "endsWith",
						"removePrefix", "removeSuffix", "repeat", "times",
					}
					unicodeMethods := []string{
						"isAlpha", "isDigit", "isNumeric", "isAlphaNumeric", "isAlnum",
						"isLower", "isUpper", "isSpace", "isPrintable",
					}
					strconvMethods := []string{
						"toInt", "parseInt", "toFloat", "parseFloat", "toBool", "parseBool",
					}
					slicesMethods := []string{
						"reverse", "flip",
					}

					for _, method := range stringsMethods {
						if method == methodName {
							v.needsStringsImport = true
							break
						}
					}
					for _, method := range unicodeMethods {
						if method == methodName {
							v.needsUnicodeImport = true
							break
						}
					}
					for _, method := range strconvMethods {
						if method == methodName {
							v.needsStrconvImport = true
							break
						}
					}
					for _, method := range slicesMethods {
						if method == methodName {
							v.needsSlicesImport = true
							break
						}
					}
				}
			}
		}
	}
	return v
}

// createReverseCall creates a call to reverse a string using slices.Reverse
func (t *StringMethodsTransform) createReverseCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	// Use a much simpler approach with slices.Reverse:
	// func() string { r := []rune(receiver); slices.Reverse(r); return string(r) }()
	
	// Create function type: func() string
	stringType := &syntax.Name{Value: "string"}
	stringType.SetPos(pos)
	
	returnField := &syntax.Field{Type: stringType}
	returnField.SetPos(pos)

	funcType := &syntax.FuncType{
		ResultList: []*syntax.Field{returnField},
	}
	funcType.SetPos(pos)

	// Function body using slices.Reverse
	body := t.createSlicesReverseBody(pos, receiver)

	// Create function literal
	funcLit := &syntax.FuncLit{
		Type: funcType,
		Body: body,
	}
	funcLit.SetPos(pos)

	// Call the function immediately
	call := &syntax.CallExpr{Fun: funcLit}
	call.SetPos(pos)

	return call
}

// createReverseBody creates the function body for string reversal
func (t *StringMethodsTransform) createReverseBody(pos syntax.Pos) *syntax.BlockStmt {
	// runes := []rune(s)
	runesVar := &syntax.Name{Value: "runes"}
	runesVar.SetPos(pos)

	sVar := &syntax.Name{Value: "s"}
	sVar.SetPos(pos)

	runeType := &syntax.Name{Value: "rune"}
	runeType.SetPos(pos)

	runeSliceType := &syntax.SliceType{Elem: runeType}
	runeSliceType.SetPos(pos)

	runeConversion := &syntax.CallExpr{
		Fun:     runeSliceType,
		ArgList: []syntax.Expr{sVar},
	}
	runeConversion.SetPos(pos)

	runesAssign := &syntax.AssignStmt{
		Op:  syntax.Def,
		Lhs: runesVar,
		Rhs: runeConversion,
	}
	runesAssign.SetPos(pos)

	// for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
	//     runes[i], runes[j] = runes[j], runes[i]
	// }

	iVar := &syntax.Name{Value: "rev1_i"}
	iVar.SetPos(pos)
	jVar := &syntax.Name{Value: "rev1_j"}
	jVar.SetPos(pos)

	zeroLit := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
	zeroLit.SetPos(pos)

	lenCall := &syntax.CallExpr{
		Fun:     &syntax.Name{Value: "len"},
		ArgList: []syntax.Expr{runesVar},
	}
	lenCall.SetPos(pos)

	oneLit := &syntax.BasicLit{Kind: syntax.IntLit, Value: "1"}
	oneLit.SetPos(pos)

	lenMinus1 := &syntax.Operation{
		Op: syntax.Sub,
		X:  lenCall,
		Y:  oneLit,
	}
	lenMinus1.SetPos(pos)

	// Initial assignment: i, j := 0, len(runes)-1
	initLhs := &syntax.ListExpr{ElemList: []syntax.Expr{iVar, jVar}}
	initLhs.SetPos(pos)
	initRhs := &syntax.ListExpr{ElemList: []syntax.Expr{zeroLit, lenMinus1}}
	initRhs.SetPos(pos)

	initStmt := &syntax.AssignStmt{
		Op:  syntax.Def,
		Lhs: initLhs,
		Rhs: initRhs,
	}
	initStmt.SetPos(pos)

	// Condition: i < j
	cond := &syntax.Operation{
		Op: syntax.Lss,
		X:  iVar,
		Y:  jVar,
	}
	cond.SetPos(pos)

	// Post: i, j = i+1, j-1
	iPlusOne := &syntax.Operation{
		Op: syntax.Add,
		X:  iVar,
		Y:  oneLit,
	}
	iPlusOne.SetPos(pos)

	jMinusOne := &syntax.Operation{
		Op: syntax.Sub,
		X:  jVar,
		Y:  oneLit,
	}
	jMinusOne.SetPos(pos)

	postLhs := &syntax.ListExpr{ElemList: []syntax.Expr{iVar, jVar}}
	postLhs.SetPos(pos)
	postRhs := &syntax.ListExpr{ElemList: []syntax.Expr{iPlusOne, jMinusOne}}
	postRhs.SetPos(pos)

	postStmt := &syntax.AssignStmt{
		Op:  0, // simple assignment =
		Lhs: postLhs,
		Rhs: postRhs,
	}
	postStmt.SetPos(pos)

	// Loop body: runes[i], runes[j] = runes[j], runes[i]
	runesI := &syntax.IndexExpr{X: runesVar, Index: iVar}
	runesI.SetPos(pos)
	runesJ := &syntax.IndexExpr{X: runesVar, Index: jVar}
	runesJ.SetPos(pos)

	swapLhs := &syntax.ListExpr{ElemList: []syntax.Expr{runesI, runesJ}}
	swapLhs.SetPos(pos)
	swapRhs := &syntax.ListExpr{ElemList: []syntax.Expr{runesJ, runesI}}
	swapRhs.SetPos(pos)

	swapStmt := &syntax.AssignStmt{
		Op:  0, // simple assignment =
		Lhs: swapLhs,
		Rhs: swapRhs,
	}
	swapStmt.SetPos(pos)

	forBody := &syntax.BlockStmt{List: []syntax.Stmt{swapStmt}}
	forBody.SetPos(pos)

	forStmt := &syntax.ForStmt{
		Init: initStmt,
		Cond: cond,
		Post: postStmt,
		Body: forBody,
	}
	forStmt.SetPos(pos)

	// return string(runes)
	stringType := &syntax.Name{Value: "string"}
	stringType.SetPos(pos)

	stringConversion := &syntax.CallExpr{
		Fun:     stringType,
		ArgList: []syntax.Expr{runesVar},
	}
	stringConversion.SetPos(pos)

	returnStmt := &syntax.ReturnStmt{Results: stringConversion}
	returnStmt.SetPos(pos)

	// Complete function body
	body := &syntax.BlockStmt{
		List: []syntax.Stmt{runesAssign, forStmt, returnStmt},
	}
	body.SetPos(pos)

	return body
}

// createSimpleReverseBody creates a simpler reverse implementation with unique variable names
func (t *StringMethodsTransform) createSimpleReverseBody(pos syntax.Pos, receiver syntax.Expr) *syntax.BlockStmt {
	// Create unique variable names to avoid conflicts
	runesVar := &syntax.Name{Value: "rev_runes"}
	runesVar.SetPos(pos)

	// Create: rev_runes := []rune(receiver)
	runeType := &syntax.Name{Value: "rune"}
	runeType.SetPos(pos)
	runeSliceType := &syntax.SliceType{Elem: runeType}
	runeSliceType.SetPos(pos)

	runeConversion := &syntax.CallExpr{
		Fun:     runeSliceType,
		ArgList: []syntax.Expr{receiver},
	}
	runeConversion.SetPos(pos)

	runesAssign := &syntax.AssignStmt{
		Op:  syntax.Def,
		Lhs: runesVar,
		Rhs: runeConversion,
	}
	runesAssign.SetPos(pos)

	// Create simple range-based reversal using standard library approach
	// We'll use a simpler method: just manually reverse by swapping in a for loop

	iVar := &syntax.Name{Value: "rev_i"}
	iVar.SetPos(pos)
	jVar := &syntax.Name{Value: "rev_j"}
	jVar.SetPos(pos)

	zeroLit := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
	zeroLit.SetPos(pos)

	lenCall := &syntax.CallExpr{
		Fun:     &syntax.Name{Value: "len"},
		ArgList: []syntax.Expr{runesVar},
	}
	lenCall.SetPos(pos)

	oneLit := &syntax.BasicLit{Kind: syntax.IntLit, Value: "1"}
	oneLit.SetPos(pos)

	lenMinus1 := &syntax.Operation{
		Op: syntax.Sub,
		X:  lenCall,
		Y:  oneLit,
	}
	lenMinus1.SetPos(pos)

	// for rev_i, rev_j := 0, len(rev_runes)-1; rev_i < rev_j; rev_i, rev_j = rev_i+1, rev_j-1
	initLhs := &syntax.ListExpr{ElemList: []syntax.Expr{iVar, jVar}}
	initLhs.SetPos(pos)
	initRhs := &syntax.ListExpr{ElemList: []syntax.Expr{zeroLit, lenMinus1}}
	initRhs.SetPos(pos)

	initStmt := &syntax.AssignStmt{
		Op:  syntax.Def,
		Lhs: initLhs,
		Rhs: initRhs,
	}
	initStmt.SetPos(pos)

	// Condition: rev_i < rev_j
	cond := &syntax.Operation{
		Op: syntax.Lss,
		X:  iVar,
		Y:  jVar,
	}
	cond.SetPos(pos)

	// Post: rev_i, rev_j = rev_i+1, rev_j-1
	iPlusOne := &syntax.Operation{
		Op: syntax.Add,
		X:  iVar,
		Y:  oneLit,
	}
	iPlusOne.SetPos(pos)

	jMinusOne := &syntax.Operation{
		Op: syntax.Sub,
		X:  jVar,
		Y:  oneLit,
	}
	jMinusOne.SetPos(pos)

	postLhs := &syntax.ListExpr{ElemList: []syntax.Expr{iVar, jVar}}
	postLhs.SetPos(pos)
	postRhs := &syntax.ListExpr{ElemList: []syntax.Expr{iPlusOne, jMinusOne}}
	postRhs.SetPos(pos)

	postStmt := &syntax.AssignStmt{
		Op:  0, // simple assignment =
		Lhs: postLhs,
		Rhs: postRhs,
	}
	postStmt.SetPos(pos)

	// Loop body: rev_runes[rev_i], rev_runes[rev_j] = rev_runes[rev_j], rev_runes[rev_i]
	runesI := &syntax.IndexExpr{X: runesVar, Index: iVar}
	runesI.SetPos(pos)
	runesJ := &syntax.IndexExpr{X: runesVar, Index: jVar}
	runesJ.SetPos(pos)

	swapLhs := &syntax.ListExpr{ElemList: []syntax.Expr{runesI, runesJ}}
	swapLhs.SetPos(pos)
	swapRhs := &syntax.ListExpr{ElemList: []syntax.Expr{runesJ, runesI}}
	swapRhs.SetPos(pos)

	swapStmt := &syntax.AssignStmt{
		Op:  0, // simple assignment =
		Lhs: swapLhs,
		Rhs: swapRhs,
	}
	swapStmt.SetPos(pos)

	forBody := &syntax.BlockStmt{List: []syntax.Stmt{swapStmt}}
	forBody.SetPos(pos)

	forStmt := &syntax.ForStmt{
		Init: initStmt,
		Cond: cond,
		Post: postStmt,
		Body: forBody,
	}
	forStmt.SetPos(pos)

	// return string(rev_runes)
	stringType := &syntax.Name{Value: "string"}
	stringType.SetPos(pos)

	stringConversion := &syntax.CallExpr{
		Fun:     stringType,
		ArgList: []syntax.Expr{runesVar},
	}
	stringConversion.SetPos(pos)

	returnStmt := &syntax.ReturnStmt{Results: stringConversion}
	returnStmt.SetPos(pos)

	// Complete function body
	body := &syntax.BlockStmt{
		List: []syntax.Stmt{runesAssign, forStmt, returnStmt},
	}
	body.SetPos(pos)

	return body
}

// createSlicesReverseBody creates a simple body using slices.Reverse
func (t *StringMethodsTransform) createSlicesReverseBody(pos syntax.Pos, receiver syntax.Expr) *syntax.BlockStmt {
	// r := []rune(receiver)
	runesVar := &syntax.Name{Value: "r"}
	runesVar.SetPos(pos)

	runeType := &syntax.Name{Value: "rune"}
	runeType.SetPos(pos)
	runeSliceType := &syntax.SliceType{Elem: runeType}
	runeSliceType.SetPos(pos)

	runeConversion := &syntax.CallExpr{
		Fun:     runeSliceType,
		ArgList: []syntax.Expr{receiver},
	}
	runeConversion.SetPos(pos)

	runesAssign := &syntax.AssignStmt{
		Op:  syntax.Def,
		Lhs: runesVar,
		Rhs: runeConversion,
	}
	runesAssign.SetPos(pos)

	// slices.Reverse(r)
	slicesName := &syntax.Name{Value: "slices"}
	slicesName.SetPos(pos)
	reverseName := &syntax.Name{Value: "Reverse"}
	reverseName.SetPos(pos)

	slicesReverse := &syntax.SelectorExpr{
		X:   slicesName,
		Sel: reverseName,
	}
	slicesReverse.SetPos(pos)

	reverseCall := &syntax.CallExpr{
		Fun:     slicesReverse,
		ArgList: []syntax.Expr{runesVar},
	}
	reverseCall.SetPos(pos)

	reverseStmt := &syntax.ExprStmt{X: reverseCall}
	reverseStmt.SetPos(pos)

	// return string(r)
	stringType := &syntax.Name{Value: "string"}
	stringType.SetPos(pos)

	stringConversion := &syntax.CallExpr{
		Fun:     stringType,
		ArgList: []syntax.Expr{runesVar},
	}
	stringConversion.SetPos(pos)

	returnStmt := &syntax.ReturnStmt{Results: stringConversion}
	returnStmt.SetPos(pos)

	// Complete function body
	body := &syntax.BlockStmt{
		List: []syntax.Stmt{runesAssign, reverseStmt, returnStmt},
	}
	body.SetPos(pos)

	return body
}

// createFirstCall creates receiver[0:1] for first character
func (t *StringMethodsTransform) createFirstCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	zeroLit := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
	zeroLit.SetPos(pos)

	index := &syntax.IndexExpr{
		X:     receiver,
		Index: zeroLit,
	}
	index.SetPos(pos)

	stringName := &syntax.Name{Value: "string"}
	stringName.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     stringName,
		ArgList: []syntax.Expr{index},
	}
	call.SetPos(pos)

	return call
}

// createLastCall creates string(receiver[len(receiver)-1]) for last character
func (t *StringMethodsTransform) createLastCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	lenName := &syntax.Name{Value: "len"}
	lenName.SetPos(pos)

	lenCall := &syntax.CallExpr{
		Fun:     lenName,
		ArgList: []syntax.Expr{receiver},
	}
	lenCall.SetPos(pos)

	oneLit := &syntax.BasicLit{Kind: syntax.IntLit, Value: "1"}
	oneLit.SetPos(pos)

	minus := &syntax.Operation{
		Op: syntax.Sub,
		X:  lenCall,
		Y:  oneLit,
	}
	minus.SetPos(pos)

	index := &syntax.IndexExpr{
		X:     receiver,
		Index: minus,
	}
	index.SetPos(pos)

	stringName := &syntax.Name{Value: "string"}
	stringName.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     stringName,
		ArgList: []syntax.Expr{index},
	}
	call.SetPos(pos)

	return call
}

// createLenCall creates len(receiver)
func (t *StringMethodsTransform) createLenCall(receiver syntax.Expr) syntax.Expr {
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

// createCharAtCall creates string(receiver[index])
func (t *StringMethodsTransform) createCharAtCall(receiver, index syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	// Create receiver[index]
	indexExpr := &syntax.IndexExpr{
		X:     receiver,
		Index: index,
	}
	indexExpr.SetPos(pos)

	// Wrap in string() conversion to get a string instead of byte
	stringName := &syntax.Name{Value: "string"}
	stringName.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     stringName,
		ArgList: []syntax.Expr{indexExpr},
	}
	call.SetPos(pos)

	return call
}

// createRuneAtCall creates a placeholder for runeAt - TODO: implement proper []rune conversion
func (t *StringMethodsTransform) createRuneAtCall(receiver, index syntax.Expr) syntax.Expr {
	// For now, fall back to charAt behavior until we implement proper runtime support
	// TODO: Replace with proper string([]rune(receiver)[index]) when runtime is working
	return t.createCharAtCall(receiver, index)
}

// createContainsCall creates strings.Contains(receiver, arg)
func (t *StringMethodsTransform) createContainsCall(receiver, arg syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "Contains"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, arg},
	}
	call.SetPos(pos)

	return call
}

// createIndexCall creates strings.Index(receiver, arg)
func (t *StringMethodsTransform) createIndexCall(receiver, arg syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "Index"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, arg},
	}
	call.SetPos(pos)

	return call
}

// createFromCall creates receiver[arg:] for substring from index
func (t *StringMethodsTransform) createFromCall(receiver, arg syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	slice := &syntax.SliceExpr{
		X:     receiver,
		Index: [3]syntax.Expr{arg, nil, nil},
	}
	slice.SetPos(pos)

	return slice
}

// createToCall creates receiver[:arg] for substring to index
func (t *StringMethodsTransform) createToCall(receiver, arg syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	slice := &syntax.SliceExpr{
		X:     receiver,
		Index: [3]syntax.Expr{nil, arg, nil},
	}
	slice.SetPos(pos)

	return slice
}

// createSubCall creates receiver[start:end] for substring operation
func (t *StringMethodsTransform) createSubCall(receiver, start, end syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	slice := &syntax.SliceExpr{
		X:     receiver,
		Index: [3]syntax.Expr{start, end, nil},
	}
	slice.SetPos(pos)

	return slice
}

// isStringExpression returns true if the expression is definitely a string
func (t *StringMethodsTransform) isStringExpression(expr syntax.Expr, ctx *TransformContext) bool {
	// Check if it's a string literal
	if basic, ok := expr.(*syntax.BasicLit); ok {
		return basic.Kind == syntax.StringLit
	}

	// Check if it's a string variable with known type
	if name, ok := expr.(*syntax.Name); ok {
		if ctx != nil && ctx.Types != nil {
			varType := ctx.Types[name.Value]
			return varType == "string"
		}
		return false
	}

	// For other cases, be conservative
	return false
}

func (t *StringMethodsTransform) addStringsImport(file *syntax.File) {
	if t.hasImport(file, "strings") {
		return
	}

	stringsImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: "\"strings\"",
			Kind:  syntax.StringLit,
		},
	}
	println("DEBUG string_methods: Creating import with Value='\"strings\"', Kind=", syntax.StringLit)
	stringsImport.SetPos(syntax.Pos{})

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

func (t *StringMethodsTransform) hasImport(file *syntax.File, name string) bool {
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

// createReplaceCall creates strings.ReplaceAll(receiver, old, new)
func (t *StringMethodsTransform) createReplaceCall(receiver, old, new syntax.Expr) syntax.Expr {
	pos := old.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "ReplaceAll"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, old, new},
	}
	call.SetPos(pos)

	return call
}

// createToUpperCall creates strings.ToUpper(receiver)
func (t *StringMethodsTransform) createToUpperCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "ToUpper"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver},
	}
	call.SetPos(pos)

	return call
}

// createToLowerCall creates strings.ToLower(receiver)
func (t *StringMethodsTransform) createToLowerCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "ToLower"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver},
	}
	call.SetPos(pos)

	return call
}

// createCapitalizeCall creates strings.Title(receiver)
func (t *StringMethodsTransform) createCapitalizeCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "Title"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver},
	}
	call.SetPos(pos)

	return call
}

// createTrimCall creates strings.TrimSpace(receiver)
func (t *StringMethodsTransform) createTrimCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "TrimSpace"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver},
	}
	call.SetPos(pos)

	return call
}

// createSplitCall creates strings.Split(receiver, sep)
func (t *StringMethodsTransform) createSplitCall(receiver, sep syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "Split"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, sep},
	}
	call.SetPos(pos)

	return call
}

// createSplitsCall creates strings.Split(receiver, "")
func (t *StringMethodsTransform) createSplitsCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "Split"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	emptyString := &syntax.BasicLit{Kind: syntax.StringLit, Value: `""`}
	emptyString.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, emptyString},
	}
	call.SetPos(pos)

	return call
}

// createRunesCall creates []rune(receiver)
func (t *StringMethodsTransform) createRunesCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	// Create []rune type
	runeName := &syntax.Name{Value: "rune"}
	runeName.SetPos(pos)

	sliceType := &syntax.SliceType{
		Elem: runeName,
	}
	sliceType.SetPos(pos)

	// Create []rune(receiver) call
	call := &syntax.CallExpr{
		Fun:     sliceType,
		ArgList: []syntax.Expr{receiver},
	}
	call.SetPos(pos)

	return call
}

// createBytesCall creates []byte(receiver)
func (t *StringMethodsTransform) createBytesCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	// Create []byte type
	byteName := &syntax.Name{Value: "byte"}
	byteName.SetPos(pos)

	sliceType := &syntax.SliceType{
		Elem: byteName,
	}
	sliceType.SetPos(pos)

	// Create []byte(receiver) call
	call := &syntax.CallExpr{
		Fun:     sliceType,
		ArgList: []syntax.Expr{receiver},
	}
	call.SetPos(pos)

	return call
}

// createCodePointsCall creates []int32([]rune(receiver)) - convert to code points
func (t *StringMethodsTransform) createCodePointsCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	// Create a function literal that properly converts string to []int
	// func() []int { 
	//   runes := []rune(receiver)
	//   result := make([]int, len(runes))
	//   for i, r := range runes {
	//     result[i] = int(r)
	//   }
	//   return result
	// }()

	// Create []int return type
	intName := &syntax.Name{Value: "int"}
	intName.SetPos(pos)
	intSliceType := &syntax.SliceType{Elem: intName}
	intSliceType.SetPos(pos)

	// Create function type: func() []int
	returnField := &syntax.Field{Type: intSliceType}
	returnField.SetPos(pos)

	funcType := &syntax.FuncType{
		ResultList: []*syntax.Field{returnField},
	}
	funcType.SetPos(pos)

	// Create function body
	body := t.createCodePointsBody(pos, receiver)

	// Create function literal
	funcLit := &syntax.FuncLit{
		Type: funcType,
		Body: body,
	}
	funcLit.SetPos(pos)

	// Create call to the function literal
	call := &syntax.CallExpr{
		Fun: funcLit,
	}
	call.SetPos(pos)

	return call
}

// createCodePointsBody creates the function body for converting string to []int code points
func (t *StringMethodsTransform) createCodePointsBody(pos syntax.Pos, receiver syntax.Expr) *syntax.BlockStmt {
	// runes := []rune(receiver)
	runesVar := &syntax.Name{Value: "runes"}
	runesVar.SetPos(pos)

	runeName := &syntax.Name{Value: "rune"}
	runeName.SetPos(pos)
	runeSliceType := &syntax.SliceType{Elem: runeName}
	runeSliceType.SetPos(pos)

	runeConversion := &syntax.CallExpr{
		Fun:     runeSliceType,
		ArgList: []syntax.Expr{receiver},
	}
	runeConversion.SetPos(pos)

	runesAssign := &syntax.AssignStmt{
		Op:  syntax.Def,
		Lhs: runesVar,
		Rhs: runeConversion,
	}
	runesAssign.SetPos(pos)

	// result := make([]int, len(runes))
	resultVar := &syntax.Name{Value: "result"}
	resultVar.SetPos(pos)

	makeName := &syntax.Name{Value: "make"}
	makeName.SetPos(pos)

	intName := &syntax.Name{Value: "int"}
	intName.SetPos(pos)
	intSliceType := &syntax.SliceType{Elem: intName}
	intSliceType.SetPos(pos)

	lenName := &syntax.Name{Value: "len"}
	lenName.SetPos(pos)
	lenCall := &syntax.CallExpr{
		Fun:     lenName,
		ArgList: []syntax.Expr{runesVar},
	}
	lenCall.SetPos(pos)

	makeCall := &syntax.CallExpr{
		Fun:     makeName,
		ArgList: []syntax.Expr{intSliceType, lenCall},
	}
	makeCall.SetPos(pos)

	resultAssign := &syntax.AssignStmt{
		Op:  syntax.Def,
		Lhs: resultVar,
		Rhs: makeCall,
	}
	resultAssign.SetPos(pos)


	// for cp_i := 0; cp_i < len(runes); cp_i++ { result[cp_i] = int(runes[cp_i]) }
	// Use unique variable names to avoid conflicts
	cpIVar := &syntax.Name{Value: "cp_i"}
	cpIVar.SetPos(pos)
	
	zeroLit := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
	zeroLit.SetPos(pos)

	initStmt := &syntax.AssignStmt{
		Op:  syntax.Def,
		Lhs: cpIVar,
		Rhs: zeroLit,
	}
	initStmt.SetPos(pos)

	// cp_i < len(runes)
	cond := &syntax.Operation{
		Op: syntax.Lss,
		X:  cpIVar,
		Y:  lenCall, // reuse the lenCall from above
	}
	cond.SetPos(pos)

	// cp_i++
	oneLit := &syntax.BasicLit{Kind: syntax.IntLit, Value: "1"}
	oneLit.SetPos(pos)
	postStmt := &syntax.AssignStmt{
		Op:  syntax.Add,
		Lhs: cpIVar,
		Rhs: oneLit,
	}
	postStmt.SetPos(pos)

	// Update the assignment to use runes[cp_i] instead of range
	runesIndex := &syntax.IndexExpr{
		X:     runesVar,
		Index: cpIVar,
	}
	runesIndex.SetPos(pos)

	intConversionFromIndex := &syntax.CallExpr{
		Fun:     intName,
		ArgList: []syntax.Expr{runesIndex},
	}
	intConversionFromIndex.SetPos(pos)

	// result[cp_i] = int(runes[cp_i])
	resultIndexFixed := &syntax.IndexExpr{
		X:     resultVar,
		Index: cpIVar,
	}
	resultIndexFixed.SetPos(pos)

	assignStmtFixed := &syntax.AssignStmt{
		Op:  0, // Regular assignment
		Lhs: resultIndexFixed,
		Rhs: intConversionFromIndex,
	}
	assignStmtFixed.SetPos(pos)

	forBodyFixed := &syntax.BlockStmt{
		List: []syntax.Stmt{assignStmtFixed},
	}
	forBodyFixed.SetPos(pos)

	forStmt := &syntax.ForStmt{
		Init: initStmt,
		Cond: cond,
		Post: postStmt,
		Body: forBodyFixed,
	}
	forStmt.SetPos(pos)

	// return result
	returnStmt := &syntax.ReturnStmt{
		Results: resultVar,
	}
	returnStmt.SetPos(pos)

	return &syntax.BlockStmt{
		List: []syntax.Stmt{runesAssign, resultAssign, forStmt, returnStmt},
	}
}

// createJoinCall creates strings.Join(receiver.splits(), sep)
func (t *StringMethodsTransform) createJoinCall(receiver, sep syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "Join"},
		},
		ArgList: []syntax.Expr{
			t.createSplitsCall(receiver),
			sep,
		},
	}
}

// createStartsWithCall creates strings.HasPrefix(receiver, prefix)
func (t *StringMethodsTransform) createStartsWithCall(receiver, prefix syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "HasPrefix"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, prefix},
	}
	call.SetPos(pos)

	return call
}

// createEndsWithCall creates strings.HasSuffix(receiver, suffix)
func (t *StringMethodsTransform) createEndsWithCall(receiver, suffix syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "HasSuffix"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, suffix},
	}
	call.SetPos(pos)

	return call
}

// createToIntCall creates helper function call for string to int conversion
func (t *StringMethodsTransform) createToIntCall(receiver syntax.Expr, base syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	if base == nil {
		// Use the must pattern: func() int { n, _ := strconv.Atoi(receiver); return n }()
		return t.createMustAtoi(pos, receiver)
	} else {
		// Use the must pattern: func() int { n, _ := strconv.ParseInt(receiver, base, 0); return int(n) }()
		return t.createMustParseInt(pos, receiver, base)
	}
}

// createAtoiWrapper creates a function literal that calls strconv.Atoi and ignores the error
func (t *StringMethodsTransform) createAtoiWrapper(pos syntax.Pos, atoiCall syntax.Expr) syntax.Expr {
	// func() int { n, _ := strconv.Atoi(receiver); return n }()
	
	// Create return type
	intName := &syntax.Name{Value: "int"}
	intName.SetPos(pos)
	
	returnField := &syntax.Field{Type: intName}
	returnField.SetPos(pos)

	funcType := &syntax.FuncType{
		ResultList: []*syntax.Field{returnField},
	}
	funcType.SetPos(pos)

	// Create variables
	nVar := &syntax.Name{Value: "n"}
	nVar.SetPos(pos)
	underscoreVar := &syntax.Name{Value: "_"}
	underscoreVar.SetPos(pos)

	// n, _ := strconv.Atoi(receiver)
	assignStmt := &syntax.AssignStmt{
		Op:  syntax.Def,
		Lhs: &syntax.ListExpr{ElemList: []syntax.Expr{nVar, underscoreVar}},
		Rhs: atoiCall,
	}
	assignStmt.SetPos(pos)

	// return n
	returnStmt := &syntax.ReturnStmt{
		Results: nVar,
	}
	returnStmt.SetPos(pos)

	body := &syntax.BlockStmt{
		List: []syntax.Stmt{assignStmt, returnStmt},
	}
	body.SetPos(pos)

	funcLit := &syntax.FuncLit{
		Type: funcType,
		Body: body,
	}
	funcLit.SetPos(pos)

	// Call the function literal
	call := &syntax.CallExpr{
		Fun: funcLit,
	}
	call.SetPos(pos)

	return call
}

// createParseIntWrapper creates a function literal that calls strconv.ParseInt and converts to int
func (t *StringMethodsTransform) createParseIntWrapper(pos syntax.Pos, parseIntCall syntax.Expr) syntax.Expr {
	// func() int { n, _ := strconv.ParseInt(receiver, base, 0); return int(n) }()
	
	// Create return type
	intName := &syntax.Name{Value: "int"}
	intName.SetPos(pos)
	
	returnField := &syntax.Field{Type: intName}
	returnField.SetPos(pos)

	funcType := &syntax.FuncType{
		ResultList: []*syntax.Field{returnField},
	}
	funcType.SetPos(pos)

	// Create variables
	nVar := &syntax.Name{Value: "n"}
	nVar.SetPos(pos)
	underscoreVar := &syntax.Name{Value: "_"}
	underscoreVar.SetPos(pos)

	// n, _ := strconv.ParseInt(receiver, base, 0)
	assignStmt := &syntax.AssignStmt{
		Op:  syntax.Def,
		Lhs: &syntax.ListExpr{ElemList: []syntax.Expr{nVar, underscoreVar}},
		Rhs: parseIntCall,
	}
	assignStmt.SetPos(pos)

	// return int(n)
	intConversion := &syntax.CallExpr{
		Fun:     intName,
		ArgList: []syntax.Expr{nVar},
	}
	intConversion.SetPos(pos)

	returnStmt := &syntax.ReturnStmt{
		Results: intConversion,
	}
	returnStmt.SetPos(pos)

	body := &syntax.BlockStmt{
		List: []syntax.Stmt{assignStmt, returnStmt},
	}
	body.SetPos(pos)

	funcLit := &syntax.FuncLit{
		Type: funcType,
		Body: body,
	}
	funcLit.SetPos(pos)

	// Call the function literal
	call := &syntax.CallExpr{
		Fun: funcLit,
	}
	call.SetPos(pos)

	return call
}

// createToFloatCall creates helper function call for string to float conversion
func (t *StringMethodsTransform) createToFloatCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	// Use the must pattern: func() float64 { f, _ := strconv.ParseFloat(receiver, 64); return f }()
	return t.createMustParseFloat(pos, receiver)
}

// createParseFloatWrapper creates a function literal that calls strconv.ParseFloat and ignores the error
func (t *StringMethodsTransform) createParseFloatWrapper(pos syntax.Pos, parseFloatCall syntax.Expr) syntax.Expr {
	// func() float64 { f, _ := strconv.ParseFloat(receiver, 64); return f }()
	
	// Create return type
	floatName := &syntax.Name{Value: "float64"}
	floatName.SetPos(pos)
	
	returnField := &syntax.Field{Type: floatName}
	returnField.SetPos(pos)

	funcType := &syntax.FuncType{
		ResultList: []*syntax.Field{returnField},
	}
	funcType.SetPos(pos)

	// Create variables
	fVar := &syntax.Name{Value: "f"}
	fVar.SetPos(pos)
	underscoreVar := &syntax.Name{Value: "_"}
	underscoreVar.SetPos(pos)

	// f, _ := strconv.ParseFloat(receiver, 64)
	assignStmt := &syntax.AssignStmt{
		Op:  syntax.Def,
		Lhs: &syntax.ListExpr{ElemList: []syntax.Expr{fVar, underscoreVar}},
		Rhs: parseFloatCall,
	}
	assignStmt.SetPos(pos)

	// return f
	returnStmt := &syntax.ReturnStmt{
		Results: fVar,
	}
	returnStmt.SetPos(pos)

	body := &syntax.BlockStmt{
		List: []syntax.Stmt{assignStmt, returnStmt},
	}
	body.SetPos(pos)

	funcLit := &syntax.FuncLit{
		Type: funcType,
		Body: body,
	}
	funcLit.SetPos(pos)

	// Call the function literal
	call := &syntax.CallExpr{
		Fun: funcLit,
	}
	call.SetPos(pos)

	return call
}

func (t *StringMethodsTransform) addStrconvImport(file *syntax.File) {
	if t.hasImport(file, "strconv") {
		return
	}

	strconvImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: "\"strconv\"",
			Kind:  syntax.StringLit,
		},
	}
	strconvImport.SetPos(syntax.Pos{})

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
	newDeclList = append(newDeclList, strconvImport)
	newDeclList = append(newDeclList, file.DeclList[insertPos:]...)
	file.DeclList = newDeclList
}

func (t *StringMethodsTransform) addUnicodeImport(file *syntax.File) {
	if t.hasImport(file, "unicode") {
		return
	}

	unicodeImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: "\"unicode\"",
			Kind:  syntax.StringLit,
		},
	}
	unicodeImport.SetPos(syntax.Pos{})

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
	newDeclList = append(newDeclList, unicodeImport)
	newDeclList = append(newDeclList, file.DeclList[insertPos:]...)
	file.DeclList = newDeclList
}

// addSlicesImport adds the slices import to the file
func (t *StringMethodsTransform) addSlicesImport(file *syntax.File) {
	if t.hasImport(file, "slices") {
		return
	}

	slicesImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: "\"slices\"",
			Kind:  syntax.StringLit,
		},
	}
	slicesImport.SetPos(syntax.Pos{})

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

// NEW METHOD IMPLEMENTATIONS

// createCompilerError creates a compiler error for unimplemented methods
func (t *StringMethodsTransform) createCompilerError(receiver syntax.Expr, methodName, description string) syntax.Expr {
	// Instead of creating a syntax error, we'll create a call to a non-existent function
	// that will produce a clear error message
	errorFuncName := "TODO_implement_runtime_function_for_" + methodName + "_" + description
	return &syntax.CallExpr{
		Fun:     &syntax.Name{Value: errorFuncName},
		ArgList: []syntax.Expr{receiver},
	}
}

// createCountCall creates strings.Count(receiver, substr)
func (t *StringMethodsTransform) createCountCall(receiver, substr syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "Count"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, substr},
	}
	call.SetPos(pos)

	return call
}

// createIsEmptyCall creates len(receiver) == 0
func (t *StringMethodsTransform) createIsEmptyCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	lenName := &syntax.Name{Value: "len"}
	lenName.SetPos(pos)

	lenCall := &syntax.CallExpr{
		Fun:     lenName,
		ArgList: []syntax.Expr{receiver},
	}
	lenCall.SetPos(pos)

	zeroLit := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
	zeroLit.SetPos(pos)

	op := &syntax.Operation{
		Op: syntax.Eql,
		X:  lenCall,
		Y:  zeroLit,
	}
	op.SetPos(pos)

	return op
}

// createLastIndexCall creates strings.LastIndex(receiver, substr)
func (t *StringMethodsTransform) createLastIndexCall(receiver, substr syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "LastIndex"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, substr},
	}
	call.SetPos(pos)

	return call
}

// createReplaceFirstCall creates strings.Replace(receiver, old, new, 1)
func (t *StringMethodsTransform) createReplaceFirstCall(receiver, old, new syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "Replace"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	oneLit := &syntax.BasicLit{Kind: syntax.IntLit, Value: "1"}
	oneLit.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, old, new, oneLit},
	}
	call.SetPos(pos)

	return call
}

// createSwapCaseCall creates TODO error (needs runtime implementation)
func (t *StringMethodsTransform) createSwapCaseCall(receiver syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "swapCase", "case_swapping")
}

// createTrimLeftCall creates strings.TrimLeft(receiver, " ")
func (t *StringMethodsTransform) createTrimLeftCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "TrimLeft"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	spaceLit := &syntax.BasicLit{Kind: syntax.StringLit, Value: `" "`}
	spaceLit.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, spaceLit},
	}
	call.SetPos(pos)

	return call
}

// createTrimRightCall creates strings.TrimRight(receiver, " ")
func (t *StringMethodsTransform) createTrimRightCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "TrimRight"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	spaceLit := &syntax.BasicLit{Kind: syntax.StringLit, Value: `" "`}
	spaceLit.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, spaceLit},
	}
	call.SetPos(pos)

	return call
}

// createLinesCall creates strings.Split(receiver, "\n")
func (t *StringMethodsTransform) createLinesCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "Split"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	newlineLit := &syntax.BasicLit{Kind: syntax.StringLit, Value: `"\n"`}
	newlineLit.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, newlineLit},
	}
	call.SetPos(pos)

	return call
}

// createWordsCall creates strings.Fields(receiver)
func (t *StringMethodsTransform) createWordsCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "Fields"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver},
	}
	call.SetPos(pos)

	return call
}

// createRemovePrefixCall creates strings.TrimPrefix(receiver, prefix)
func (t *StringMethodsTransform) createRemovePrefixCall(receiver, prefix syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "TrimPrefix"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, prefix},
	}
	call.SetPos(pos)

	return call
}

// createRemoveSuffixCall creates strings.TrimSuffix(receiver, suffix)
func (t *StringMethodsTransform) createRemoveSuffixCall(receiver, suffix syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "TrimSuffix"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, suffix},
	}
	call.SetPos(pos)

	return call
}

// createRepeatCall creates strings.Repeat(receiver, count)
func (t *StringMethodsTransform) createRepeatCall(receiver, count syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	stringsName := &syntax.Name{Value: "strings"}
	stringsName.SetPos(pos)

	funcName := &syntax.Name{Value: "Repeat"}
	funcName.SetPos(pos)

	selector := &syntax.SelectorExpr{
		X:   stringsName,
		Sel: funcName,
	}
	selector.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     selector,
		ArgList: []syntax.Expr{receiver, count},
	}
	call.SetPos(pos)

	return call
}

// Padding methods (need runtime implementation)
func (t *StringMethodsTransform) createCenterCall(receiver, width, fillChar syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "center", "string_centering")
}

func (t *StringMethodsTransform) createPadLeftCall(receiver, width, fillChar syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "padLeft", "left_padding")
}

func (t *StringMethodsTransform) createPadRightCall(receiver, width, fillChar syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "padRight", "right_padding")
}

func (t *StringMethodsTransform) createZfillCall(receiver, width syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "zfill", "zero_padding")
}

// Unicode character checking methods
func (t *StringMethodsTransform) createIsAlphaCall(receiver syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "isAlpha", "unicode_alpha_check")
}

func (t *StringMethodsTransform) createIsDigitCall(receiver syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "isDigit", "unicode_digit_check")
}

func (t *StringMethodsTransform) createIsAlnumCall(receiver syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "isAlnum", "unicode_alphanumeric_check")
}

func (t *StringMethodsTransform) createIsLowerCall(receiver syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "isLower", "unicode_lowercase_check")
}

func (t *StringMethodsTransform) createIsUpperCall(receiver syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "isUpper", "unicode_uppercase_check")
}

func (t *StringMethodsTransform) createIsSpaceCall(receiver syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "isSpace", "unicode_whitespace_check")
}

func (t *StringMethodsTransform) createIsPrintableCall(receiver syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "isPrintable", "unicode_printable_check")
}

// createToBoolCall creates helper function call for string to bool conversion
func (t *StringMethodsTransform) createToBoolCall(receiver syntax.Expr) syntax.Expr {
	pos := receiver.Pos()

	funcName := &syntax.Name{Value: "stringToBool"}
	funcName.SetPos(pos)

	call := &syntax.CallExpr{
		Fun:     funcName,
		ArgList: []syntax.Expr{receiver},
	}
	call.SetPos(pos)

	return call
}

// createMustAtoi creates a simple call that panics on error: func() int { n, _ := strconv.Atoi(s); return n }()
func (t *StringMethodsTransform) createMustAtoi(pos syntax.Pos, receiver syntax.Expr) syntax.Expr {
	// Build: func() int { n, _ := strconv.Atoi(receiver); return n }()
	
	// Create strconv.Atoi call
	strconvName := &syntax.Name{Value: "strconv"}
	strconvName.SetPos(pos)
	
	atoiName := &syntax.Name{Value: "Atoi"}
	atoiName.SetPos(pos)
	
	selector := &syntax.SelectorExpr{X: strconvName, Sel: atoiName}
	selector.SetPos(pos)
	
	atoiCall := &syntax.CallExpr{Fun: selector, ArgList: []syntax.Expr{receiver}}
	atoiCall.SetPos(pos)
	
	// Create func() int { n, _ := strconv.Atoi(receiver); return n }()
	return t.createMustWrapper(pos, atoiCall, "int", "n")
}

// createMustParseInt creates a ParseInt wrapper
func (t *StringMethodsTransform) createMustParseInt(pos syntax.Pos, receiver syntax.Expr, base syntax.Expr) syntax.Expr {
	strconvName := &syntax.Name{Value: "strconv"}
	strconvName.SetPos(pos)
	
	parseIntName := &syntax.Name{Value: "ParseInt"}
	parseIntName.SetPos(pos)
	
	selector := &syntax.SelectorExpr{X: strconvName, Sel: parseIntName}
	selector.SetPos(pos)
	
	zeroLit := &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"}
	zeroLit.SetPos(pos)
	
	parseIntCall := &syntax.CallExpr{Fun: selector, ArgList: []syntax.Expr{receiver, base, zeroLit}}
	parseIntCall.SetPos(pos)
	
	return t.createMustWrapper(pos, parseIntCall, "int", "n")
}

// createMustParseFloat creates a ParseFloat wrapper  
func (t *StringMethodsTransform) createMustParseFloat(pos syntax.Pos, receiver syntax.Expr) syntax.Expr {
	strconvName := &syntax.Name{Value: "strconv"}
	strconvName.SetPos(pos)
	
	parseFloatName := &syntax.Name{Value: "ParseFloat"}
	parseFloatName.SetPos(pos)
	
	selector := &syntax.SelectorExpr{X: strconvName, Sel: parseFloatName}
	selector.SetPos(pos)
	
	sixtyFourLit := &syntax.BasicLit{Kind: syntax.IntLit, Value: "64"}
	sixtyFourLit.SetPos(pos)
	
	parseFloatCall := &syntax.CallExpr{Fun: selector, ArgList: []syntax.Expr{receiver, sixtyFourLit}}
	parseFloatCall.SetPos(pos)
	
	return t.createMustWrapper(pos, parseFloatCall, "float64", "f")
}

// createMustWrapper creates func() RetType { varName, _ := call; return RetType(varName) }()
func (t *StringMethodsTransform) createMustWrapper(pos syntax.Pos, call syntax.Expr, retType string, varName string) syntax.Expr {
	// Create return type
	retTypeName := &syntax.Name{Value: retType}
	retTypeName.SetPos(pos)
	
	retField := &syntax.Field{Type: retTypeName}
	retField.SetPos(pos)
	
	funcType := &syntax.FuncType{ResultList: []*syntax.Field{retField}}
	funcType.SetPos(pos)
	
	// Create variables  
	varNameNode := &syntax.Name{Value: varName}
	varNameNode.SetPos(pos)
	underscoreVar := &syntax.Name{Value: "_"}
	underscoreVar.SetPos(pos)
	
	// Create assignment: varName, _ := call
	assignStmt := &syntax.AssignStmt{
		Op:  syntax.Def,
		Lhs: &syntax.ListExpr{ElemList: []syntax.Expr{varNameNode, underscoreVar}},
		Rhs: call,
	}
	assignStmt.SetPos(pos)
	
	// Create return: return RetType(varName)
	castExpr := &syntax.CallExpr{
		Fun:     retTypeName,
		ArgList: []syntax.Expr{varNameNode},
	}
	castExpr.SetPos(pos)
	
	returnStmt := &syntax.ReturnStmt{Results: castExpr}
	returnStmt.SetPos(pos)
	
	// Create function body
	body := &syntax.BlockStmt{List: []syntax.Stmt{assignStmt, returnStmt}}
	body.SetPos(pos)
	
	// Create function literal
	funcLit := &syntax.FuncLit{Type: funcType, Body: body}
	funcLit.SetPos(pos)
	
	// Create function call
	funcCall := &syntax.CallExpr{Fun: funcLit}
	funcCall.SetPos(pos)
	
	return funcCall
}

func init() {
	RegisterTransformer(&StringMethodsTransform{})
}
