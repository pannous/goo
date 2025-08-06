package main

import (
	"fmt"
	"strings"
	"cmd/compile/internal/syntax"
)

func main() {
	// Test different dot patterns
	testCases := []string{
		".",     // single dot
		"..",    // double dot (our new token)
		"...",   // triple dot (existing)
		".5",    // number starting with dot
	}

	for _, test := range testCases {
		fmt.Printf("Testing: %q\n", test)
		
		var s syntax.Scanner
		s.Init(strings.NewReader(test), nil, 0, "test.go")
		s.Next()
		
		fmt.Printf("  Token: %s\n", s.Tok().String())
		if s.Tok() == syntax._Literal {
			fmt.Printf("  Literal: %q\n", s.Lit())
		}
		fmt.Println()
	}
}