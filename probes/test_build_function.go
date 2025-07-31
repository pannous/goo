package main

import (
	"fmt"
	"go/build"
)

func main() {
	testPaths := []string{
		"helper.goo",
		"./helper.goo", 
		"../helper.goo",
		"fmt",
		".",
	}
	
	for _, path := range testPaths {
		fmt.Printf("IsLocalImport(%q) = %t\n", path, build.IsLocalImport(path))
	}
}