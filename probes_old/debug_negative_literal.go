package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

func main() {
	src := `package main
func main() {
	x := -1
}`
	
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		panic(err)
	}
	
	ast.Inspect(f, func(n ast.Node) bool {
		if unary, ok := n.(*ast.UnaryExpr); ok {
			fmt.Printf("UnaryExpr: Op=%v, X=%v\n", unary.Op, unary.X)
			if basic, ok := unary.X.(*ast.BasicLit); ok {
				fmt.Printf("  BasicLit: Kind=%v, Value=%s\n", basic.Kind, basic.Value)
			}
		}
		if basic, ok := n.(*ast.BasicLit); ok {
			fmt.Printf("BasicLit: Kind=%v, Value=%s\n", basic.Kind, basic.Value)
		}
		return true
	})
}