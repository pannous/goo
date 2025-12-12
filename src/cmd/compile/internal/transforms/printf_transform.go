// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// PrintfTransform converts printf calls to fmt.Printf and put calls to fmt.Println, and adds fmt import
type PrintfTransform struct{}

type printfVisitor struct {
	transform *PrintfTransform
	ctx       *TransformContext
	changed   bool
}

func (t *PrintfTransform) Name() string {
	return "printf_transform"
}

func (t *PrintfTransform) Priority() int {
	return 110 // Run after class transform (100), to catch printf in expanded class methods
}

func (t *PrintfTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &printfVisitor{transform: t, ctx: ctx}

	// Use the visitor pattern to walk all nodes
	syntax.Walk(file, visitor)

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
			// Request fmt import via ImportManager
			RequestFmtImport()
		}
	}
	
	// Continue visiting child nodes
	return v
}

// convertPrintfCall converts printf() and put() calls to fmt.Printf()
func (v *printfVisitor) convertPrintfCall(call *syntax.CallExpr) bool {
	// Check if this is a printf or put function call
	if name, ok := call.Fun.(*syntax.Name); ok {
		switch name.Value {
		case "printf":
			// Convert printf() to fmt.Printf()
			v.convertToFmtPrintf(call, name)
			return true
		case "put":
			// Convert put() to fmt.Println()
			v.convertToFmtPrintln(call, name)
			return true
		}
	}

	return false
}

// convertToFmtPrintf converts a call to fmt.Printf
func (v *printfVisitor) convertToFmtPrintf(call *syntax.CallExpr, name *syntax.Name) {
	pos := call.Pos()
	
	// Create fmt identifier
	fmtName := &syntax.Name{Value: "fmt"}
	fmtName.SetPos(pos)
	
	// Create Printf identifier
	printfName := &syntax.Name{Value: "Printf"}
	printfName.SetPos(pos)
	
	// Create fmt.Printf selector
	fmtPrintf := &syntax.SelectorExpr{
		X:   fmtName,
		Sel: printfName,
	}
	fmtPrintf.SetPos(pos)
	
	// Replace the function with fmt.Printf
	call.Fun = fmtPrintf
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

// Note: Import handling now done via ImportManager in transform.go

func init() {
	RegisterTransformer(&PrintfTransform{})
}