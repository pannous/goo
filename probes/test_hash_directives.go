package main

import (
	"fmt"
	"strings"
	"cmd/compile/internal/syntax"
)

func main() {
	// Test cases for hash directives
	testCases := []string{
		"#if DEBUG",
		"#end",
		"# regular comment",
		"#if FEATURE_X",
		"#end // with comment",  
		"#ifnot", // Should be treated as regular comment
		"#endurance", // Should be treated as regular comment
	}

	for _, src := range testCases {
		fmt.Printf("Testing: %s\n", src)
		
		var comments []string
		var s scanner
		s.init(strings.NewReader(src), func(line, col uint, msg string) {
			if strings.HasPrefix(msg, "#") {
				fmt.Printf("  -> Directive/Comment: %s at line %d, col %d\n", msg, line, col)
				comments = append(comments, msg)
			} else {
				fmt.Printf("  -> Error: %s at line %d, col %d\n", msg, line, col)
			}
		}, directives, "test.goo")

		s.next()
		
		fmt.Printf("  Token: %s\n", s.tok)
		fmt.Printf("  Found %d comments/directives\n\n", len(comments))
	}
}