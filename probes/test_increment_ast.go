package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

func main() {
	// Parse a simple for loop with increment
	src := `package main
func main() {
	for i := 0; i < 10; i++ {
		println(i)
	}
}`
	
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		panic(err)
	}
	
	ast.Inspect(file, func(n ast.Node) bool {
		if forStmt, ok := n.(*ast.ForStmt); ok {
			fmt.Printf("Found for statement\n")
			if forStmt.Post != nil {
				fmt.Printf("Post statement type: %T\n", forStmt.Post)
				if incStmt, ok := forStmt.Post.(*ast.IncDecStmt); ok {
					fmt.Printf("Increment statement: %s %s\n", 
						incStmt.X, incStmt.Tok)
				}
			}
		}
		return true
	})
}