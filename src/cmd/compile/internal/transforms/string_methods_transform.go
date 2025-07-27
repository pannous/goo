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
}

func (t *StringMethodsTransform) Name() string {
	return "string_methods"
}

// transformStringMethod transforms string method calls to standard library calls
func (t *StringMethodsTransform) transformStringMethod(receiver syntax.Expr, methodName string, args []syntax.Expr) syntax.Expr {
	switch methodName {
	// Basic string info
	case "reverse":
		return t.createReverseCall(receiver)
	case "first":
		return t.createFirstCall(receiver)
	case "last":
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

	// Search methods
	case "contains", "includes":
		if len(args) == 1 {
			return t.createContainsCall(receiver, args[0])
		}
	case "indexOf", "find":
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
	case "replace", "replaceAll":
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
			var fillChar syntax.Expr = &syntax.BasicLit{Kind: syntax.StringLit, Value: `" "`}
			if len(args) == 2 {
				fillChar = args[1]
			}
			return t.createCenterCall(receiver, args[0], fillChar)
		}
	case "ljust", "padLeft":
		if len(args) >= 1 {
			var fillChar syntax.Expr = &syntax.BasicLit{Kind: syntax.StringLit, Value: `" "`}
			if len(args) == 2 {
				fillChar = args[1]
			}
			return t.createPadLeftCall(receiver, args[0], fillChar)
		}
	case "rjust", "padRight":
		if len(args) >= 1 {
			var fillChar syntax.Expr = &syntax.BasicLit{Kind: syntax.StringLit, Value: `" "`}
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
	println("StringMethodsTransform.Transform called")

	visitor := &methodVisitor{transform: t, ctx: ctx}
	syntax.Walk(file, visitor)

	// Add required imports if needed and transformations were made
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
						// We need to replace the parent node, but in the visitor pattern
						// we can't easily do this. For now, let's convert slices to function calls
						v.transform.handleSliceTransformation(call, expr)
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
						"reverse", "sub", "substring", "slice",
					}
					unicodeMethods := []string{
						"isAlpha", "isDigit", "isNumeric", "isAlphaNumeric", "isAlnum",
						"isLower", "isUpper", "isSpace", "isPrintable",
					}
					strconvMethods := []string{
						"toInt", "parseInt", "toFloat", "parseFloat", "toBool", "parseBool",
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
				}
			}
		}
	}
	return v
}

// createReverseCall creates strings.ReverseString(receiver)
func (t *StringMethodsTransform) createReverseCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "ReverseString"},
		},
		ArgList: []syntax.Expr{receiver},
	}
}

// createFirstCall creates receiver[0:1] for first character
func (t *StringMethodsTransform) createFirstCall(receiver syntax.Expr) syntax.Expr {
	// Create a slice expression receiver[0:1]
	return &syntax.CallExpr{
		Fun: &syntax.Name{Value: "string"},
		ArgList: []syntax.Expr{
			&syntax.IndexExpr{
				X:     receiver,
				Index: &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"},
			},
		},
	}
}

// createLastCall creates string(receiver[len(receiver)-1]) for last character
func (t *StringMethodsTransform) createLastCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.Name{Value: "string"},
		ArgList: []syntax.Expr{
			&syntax.IndexExpr{
				X: receiver,
				Index: &syntax.Operation{
					Op: syntax.Sub,
					X: &syntax.CallExpr{
						Fun:     &syntax.Name{Value: "len"},
						ArgList: []syntax.Expr{receiver},
					},
					Y: &syntax.BasicLit{Kind: syntax.IntLit, Value: "1"},
				},
			},
		},
	}
}

// createLenCall creates len(receiver)
func (t *StringMethodsTransform) createLenCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun:     &syntax.Name{Value: "len"},
		ArgList: []syntax.Expr{receiver},
	}
}

// createContainsCall creates strings.Contains(receiver, arg)
func (t *StringMethodsTransform) createContainsCall(receiver, arg syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "Contains"},
		},
		ArgList: []syntax.Expr{receiver, arg},
	}
}

// createIndexCall creates strings.Index(receiver, arg)
func (t *StringMethodsTransform) createIndexCall(receiver, arg syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "Index"},
		},
		ArgList: []syntax.Expr{receiver, arg},
	}
}

// createFromCall creates receiver[arg:] for substring from index
func (t *StringMethodsTransform) createFromCall(receiver, arg syntax.Expr) syntax.Expr {
	return &syntax.SliceExpr{
		X:     receiver,
		Index: [3]syntax.Expr{arg, nil, nil},
	}
}

// createToCall creates receiver[:arg] for substring to index
func (t *StringMethodsTransform) createToCall(receiver, arg syntax.Expr) syntax.Expr {
	return &syntax.SliceExpr{
		X:     receiver,
		Index: [3]syntax.Expr{nil, arg, nil},
	}
}

// createSubCall creates strings.Substring(receiver, start, end)
func (t *StringMethodsTransform) createSubCall(receiver, start, end syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "Substring"},
		},
		ArgList: []syntax.Expr{receiver, start, end},
	}
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

// handleSliceTransformation converts a SliceExpr back to a CallExpr for the visitor pattern
func (t *StringMethodsTransform) handleSliceTransformation(call *syntax.CallExpr, slice *syntax.SliceExpr) {
	println("JUST USE INDEX for substring operations \"hello\"[1:3] -> \"hello\".substring(1, 3)")
	// Convert slice operations to function calls that can work within the visitor pattern
	// This is a workaround since we can't easily replace nodes in the visitor

	// For now, we'll modify the call to use a helper function
	// In a real implementation, you'd want to add these helpers to a runtime package
	if slice.Index[0] != nil && slice.Index[1] == nil {
		// from(string, index)
		call.Fun = &syntax.Name{Value: "substring"}
		call.ArgList = []syntax.Expr{slice.X, slice.Index[0], &syntax.BasicLit{Kind: syntax.IntLit, Value: "-1"}}
	} else if slice.Index[0] == nil && slice.Index[1] != nil {
		// to(string, index)
		call.Fun = &syntax.Name{Value: "substring"}
		call.ArgList = []syntax.Expr{slice.X, &syntax.BasicLit{Kind: syntax.IntLit, Value: "-1"}, slice.Index[1]}
	} else if slice.Index[0] != nil && slice.Index[1] != nil {
		// sub(string, start, end)
		call.Fun = &syntax.Name{Value: "substring"}
		call.ArgList = []syntax.Expr{slice.X, slice.Index[0], slice.Index[1]}
	}
}

// createReplaceCall creates strings.ReplaceAll(receiver, old, new)
func (t *StringMethodsTransform) createReplaceCall(receiver, old, new syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "ReplaceAll"},
		},
		ArgList: []syntax.Expr{receiver, old, new},
	}
}

// createToUpperCall creates strings.ToUpper(receiver)
func (t *StringMethodsTransform) createToUpperCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "ToUpper"},
		},
		ArgList: []syntax.Expr{receiver},
	}
}

// createToLowerCall creates strings.ToLower(receiver)
func (t *StringMethodsTransform) createToLowerCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "ToLower"},
		},
		ArgList: []syntax.Expr{receiver},
	}
}

// createCapitalizeCall creates strings.Title(receiver)
func (t *StringMethodsTransform) createCapitalizeCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "Title"},
		},
		ArgList: []syntax.Expr{receiver},
	}
}

// createTrimCall creates strings.TrimSpace(receiver)
func (t *StringMethodsTransform) createTrimCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "TrimSpace"},
		},
		ArgList: []syntax.Expr{receiver},
	}
}

// createSplitCall creates strings.Split(receiver, sep)
func (t *StringMethodsTransform) createSplitCall(receiver, sep syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "Split"},
		},
		ArgList: []syntax.Expr{receiver, sep},
	}
}

// createSplitsCall creates strings.Split(receiver, "")
func (t *StringMethodsTransform) createSplitsCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "Split"},
		},
		ArgList: []syntax.Expr{
			receiver,
			&syntax.BasicLit{Kind: syntax.StringLit, Value: `""`},
		},
	}
}

// createRunesCall creates []rune(receiver)
func (t *StringMethodsTransform) createRunesCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.ArrayType{
			Elem: &syntax.Name{Value: "rune"},
		},
		ArgList: []syntax.Expr{receiver},
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
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "HasPrefix"},
		},
		ArgList: []syntax.Expr{receiver, prefix},
	}
}

// createEndsWithCall creates strings.HasSuffix(receiver, suffix)
func (t *StringMethodsTransform) createEndsWithCall(receiver, suffix syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "HasSuffix"},
		},
		ArgList: []syntax.Expr{receiver, suffix},
	}
}

// createToIntCall creates helper function call for string to int conversion
func (t *StringMethodsTransform) createToIntCall(receiver syntax.Expr, base syntax.Expr) syntax.Expr {
	if base == nil {
		// Call runtime helper: stringToInt(receiver)
		return &syntax.CallExpr{
			Fun:     &syntax.Name{Value: "stringToInt"},
			ArgList: []syntax.Expr{receiver},
		}
	} else {
		// Call runtime helper: stringToIntBase(receiver, base)
		return &syntax.CallExpr{
			Fun:     &syntax.Name{Value: "stringToIntBase"},
			ArgList: []syntax.Expr{receiver, base},
		}
	}
}

// createToFloatCall creates helper function call for string to float conversion
func (t *StringMethodsTransform) createToFloatCall(receiver syntax.Expr) syntax.Expr {
	// Call runtime helper: stringToFloat(receiver)
	return &syntax.CallExpr{
		Fun:     &syntax.Name{Value: "stringToFloat"},
		ArgList: []syntax.Expr{receiver},
	}
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
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "Count"},
		},
		ArgList: []syntax.Expr{receiver, substr},
	}
}

// createIsEmptyCall creates len(receiver) == 0
func (t *StringMethodsTransform) createIsEmptyCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.Operation{
		Op: syntax.Eql,
		X: &syntax.CallExpr{
			Fun:     &syntax.Name{Value: "len"},
			ArgList: []syntax.Expr{receiver},
		},
		Y: &syntax.BasicLit{Kind: syntax.IntLit, Value: "0"},
	}
}

// createLastIndexCall creates strings.LastIndex(receiver, substr)
func (t *StringMethodsTransform) createLastIndexCall(receiver, substr syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "LastIndex"},
		},
		ArgList: []syntax.Expr{receiver, substr},
	}
}

// createReplaceFirstCall creates strings.Replace(receiver, old, new, 1)
func (t *StringMethodsTransform) createReplaceFirstCall(receiver, old, new syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "Replace"},
		},
		ArgList: []syntax.Expr{receiver, old, new, &syntax.BasicLit{Kind: syntax.IntLit, Value: "1"}},
	}
}

// createSwapCaseCall creates TODO error (needs runtime implementation)
func (t *StringMethodsTransform) createSwapCaseCall(receiver syntax.Expr) syntax.Expr {
	return t.createCompilerError(receiver, "swapCase", "case_swapping")
}

// createTrimLeftCall creates strings.TrimLeft(receiver, " ")
func (t *StringMethodsTransform) createTrimLeftCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "TrimLeft"},
		},
		ArgList: []syntax.Expr{receiver, &syntax.BasicLit{Kind: syntax.StringLit, Value: `" "`}},
	}
}

// createTrimRightCall creates strings.TrimRight(receiver, " ")
func (t *StringMethodsTransform) createTrimRightCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "TrimRight"},
		},
		ArgList: []syntax.Expr{receiver, &syntax.BasicLit{Kind: syntax.StringLit, Value: `" "`}},
	}
}

// createLinesCall creates strings.Split(receiver, "\n")
func (t *StringMethodsTransform) createLinesCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "Split"},
		},
		ArgList: []syntax.Expr{receiver, &syntax.BasicLit{Kind: syntax.StringLit, Value: `"\n"`}},
	}
}

// createWordsCall creates strings.Fields(receiver)
func (t *StringMethodsTransform) createWordsCall(receiver syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "Fields"},
		},
		ArgList: []syntax.Expr{receiver},
	}
}

// createRemovePrefixCall creates strings.TrimPrefix(receiver, prefix)
func (t *StringMethodsTransform) createRemovePrefixCall(receiver, prefix syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "TrimPrefix"},
		},
		ArgList: []syntax.Expr{receiver, prefix},
	}
}

// createRemoveSuffixCall creates strings.TrimSuffix(receiver, suffix)
func (t *StringMethodsTransform) createRemoveSuffixCall(receiver, suffix syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "TrimSuffix"},
		},
		ArgList: []syntax.Expr{receiver, suffix},
	}
}

// createRepeatCall creates strings.Repeat(receiver, count)
func (t *StringMethodsTransform) createRepeatCall(receiver, count syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X:   &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "Repeat"},
		},
		ArgList: []syntax.Expr{receiver, count},
	}
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
	// Call runtime helper: stringToBool(receiver)
	return &syntax.CallExpr{
		Fun:     &syntax.Name{Value: "stringToBool"},
		ArgList: []syntax.Expr{receiver},
	}
}

func init() {
	RegisterTransformer(&StringMethodsTransform{})
}
