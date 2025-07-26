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
	transform         *StringMethodsTransform
	ctx               *TransformContext
	changed           bool
	needsStringsImport bool
}

func (t *StringMethodsTransform) Name() string {
	return "string_methods"
}

func (t *StringMethodsTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	println("StringMethodsTransform.Transform called")

	visitor := &methodVisitor{transform: t, ctx: ctx}
	syntax.Walk(file, visitor)

	// Add strings import if needed and transformations were made
	if visitor.needsStringsImport && !t.hasImport(file, "strings") {
		println("Adding strings import")
		t.addStringsImport(file)
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
					if methodName == "contains" || methodName == "indexOf" {
						v.needsStringsImport = true
					}
				}
			}
		}
	}

	return v
}

// transformStringMethod transforms string method calls to standard library calls
func (t *StringMethodsTransform) transformStringMethod(receiver syntax.Expr, methodName string, args []syntax.Expr) syntax.Expr {
	switch methodName {
	case "reverse":
		return t.createReverseCall(receiver)
	case "first":
		return t.createFirstCall(receiver)
	case "last":
		return t.createLastCall(receiver)
	case "size", "length":
		return t.createLenCall(receiver)
	case "contains":
		if len(args) == 1 {
			return t.createContainsCall(receiver, args[0])
		}
	case "indexOf":
		if len(args) == 1 {
			return t.createIndexCall(receiver, args[0])
		}
	case "from":
		if len(args) == 1 {
			return t.createFromCall(receiver, args[0])
		}
	case "to":
		if len(args) == 1 {
			return t.createToCall(receiver, args[0])
		}
	case "sub":
		if len(args) == 2 {
			return t.createSubCall(receiver, args[0], args[1])
		}
	}
	return nil
}

// createReverseCall creates a function to reverse a string
func (t *StringMethodsTransform) createReverseCall(receiver syntax.Expr) syntax.Expr {
	// For now, create a simple inline reverse using range and string concatenation
	// This would need a proper reverse function in practice
	return &syntax.CallExpr{
		Fun: &syntax.Name{Value: "reverseString"},
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
				X: receiver,
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
						Fun: &syntax.Name{Value: "len"},
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
		Fun: &syntax.Name{Value: "len"},
		ArgList: []syntax.Expr{receiver},
	}
}

// createContainsCall creates strings.Contains(receiver, arg)
func (t *StringMethodsTransform) createContainsCall(receiver, arg syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X: &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "Contains"},
		},
		ArgList: []syntax.Expr{receiver, arg},
	}
}

// createIndexCall creates strings.Index(receiver, arg)
func (t *StringMethodsTransform) createIndexCall(receiver, arg syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{
		Fun: &syntax.SelectorExpr{
			X: &syntax.Name{Value: "strings"},
			Sel: &syntax.Name{Value: "Index"},
		},
		ArgList: []syntax.Expr{receiver, arg},
	}
}

// createFromCall creates receiver[arg:] for substring from index
func (t *StringMethodsTransform) createFromCall(receiver, arg syntax.Expr) syntax.Expr {
	return &syntax.SliceExpr{
		X: receiver,
		Index: [3]syntax.Expr{arg, nil, nil},
	}
}

// createToCall creates receiver[:arg] for substring to index
func (t *StringMethodsTransform) createToCall(receiver, arg syntax.Expr) syntax.Expr {
	return &syntax.SliceExpr{
		X: receiver,
		Index: [3]syntax.Expr{nil, arg, nil},
	}
}

// createSubCall creates receiver[start:end] for substring with start and end
func (t *StringMethodsTransform) createSubCall(receiver, start, end syntax.Expr) syntax.Expr {
	return &syntax.SliceExpr{
		X: receiver,
		Index: [3]syntax.Expr{start, end, nil},
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
	// Convert slice operations to function calls that can work within the visitor pattern
	// This is a workaround since we can't easily replace nodes in the visitor
	
	// For now, we'll modify the call to use a helper function
	// In a real implementation, you'd want to add these helpers to a runtime package
	if slice.Index[0] != nil && slice.Index[1] == nil {
		// from(string, index)
		call.Fun = &syntax.Name{Value: "substringFrom"}
		call.ArgList = []syntax.Expr{slice.X, slice.Index[0]}
	} else if slice.Index[0] == nil && slice.Index[1] != nil {
		// to(string, index)  
		call.Fun = &syntax.Name{Value: "substringTo"}
		call.ArgList = []syntax.Expr{slice.X, slice.Index[1]}
	} else if slice.Index[0] != nil && slice.Index[1] != nil {
		// sub(string, start, end)
		call.Fun = &syntax.Name{Value: "substring"}
		call.ArgList = []syntax.Expr{slice.X, slice.Index[0], slice.Index[1]}
	}
}

func init() {
	println("Registering string methods transformer!")
	RegisterTransformer(&StringMethodsTransform{})
}