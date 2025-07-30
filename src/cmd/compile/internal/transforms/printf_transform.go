// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// PrintfTransform converts printf/put calls to fmt.Println and adds fmt import
type PrintfTransform struct{}

type printfVisitor struct {
	transform      *PrintfTransform
	ctx            *TransformContext
	changed        bool
	needsFmtImport bool
}

func (t *PrintfTransform) Name() string {
	return "printf_transform"
}

func (t *PrintfTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &printfVisitor{transform: t, ctx: ctx}
	
	// Use the visitor pattern to walk all nodes
	syntax.Walk(file, visitor)
	
	// Add fmt import if we made changes and it's needed
	if visitor.needsFmtImport && !t.hasImport(file, "fmt") {
		t.addFmtImport(file)
		visitor.changed = true
	}
	
	return visitor.changed
}

// Visit implements syntax.Visitor
func (v *printfVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}
	
	// Look for function calls that might be printf or put
	switch n := node.(type) {
	case *syntax.CallExpr:
		if v.convertPrintfCall(n) {
			v.changed = true
			v.needsFmtImport = true
		}
	}
	
	// Continue visiting child nodes
	return v
}

// convertPrintfCall converts printf() and put() calls to fmt.Println()
func (v *printfVisitor) convertPrintfCall(call *syntax.CallExpr) bool {
	// Check if this is a printf or put function call
	if name, ok := call.Fun.(*syntax.Name); ok {
		switch name.Value {
		case "printf":
			// Convert printf() to fmt.Println()
			v.convertToFmtPrintln(call, name)
			return true
		case "put":
			// Convert put() to fmt.Println()  
			v.convertToFmtPrintln(call, name)
			return true
		}
	}
	
	return false
}

// convertToFmtPrintln converts a call to fmt.Println
func (v *printfVisitor) convertToFmtPrintln(call *syntax.CallExpr, name *syntax.Name) {
	pos := call.Pos()
	
	// Create fmt identifier
	fmtName := &syntax.Name{Value: "fmt"}
	fmtName.SetPos(pos)
	
	// Create Println identifier
	printlnName := &syntax.Name{Value: "Println"}
	printlnName.SetPos(pos)
	
	// Create fmt.Println selector
	fmtPrintln := &syntax.SelectorExpr{
		X:   fmtName,
		Sel: printlnName,
	}
	fmtPrintln.SetPos(pos)
	
	// Replace the function with fmt.Println
	call.Fun = fmtPrintln
}

// hasImport checks if the file already imports the specified package
func (t *PrintfTransform) hasImport(file *syntax.File, pkgName string) bool {
	quotedName := "\"" + pkgName + "\""
	for _, decl := range file.DeclList {
		if importDecl, ok := decl.(*syntax.ImportDecl); ok {
			if importDecl.Path != nil && importDecl.Path.Value == quotedName {
				return true
			}
		}
	}
	return false
}

// addFmtImport adds the fmt package import
func (t *PrintfTransform) addFmtImport(file *syntax.File) {
	fmtImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: "\"fmt\"",
			Kind:  syntax.StringLit,
		},
	}

	// Find where to insert the import (after existing imports)
	var insertPos int
	for i, decl := range file.DeclList {
		if _, ok := decl.(*syntax.ImportDecl); ok {
			insertPos = i + 1
		} else {
			break
		}
	}

	// Insert the import
	newDeclList := make([]syntax.Decl, 0, len(file.DeclList)+1)
	newDeclList = append(newDeclList, file.DeclList[:insertPos]...)
	newDeclList = append(newDeclList, fmtImport)
	newDeclList = append(newDeclList, file.DeclList[insertPos:]...)
	file.DeclList = newDeclList
}

func init() {
	RegisterTransformer(&PrintfTransform{})
}