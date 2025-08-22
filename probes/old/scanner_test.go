package main

import (
	"fmt"
	"strings"
	"cmd/compile/internal/syntax"
)

func main() {
	source := `package main

#if DEBUG
	fmt.Println("Debug mode enabled")
#end

#if FEATURE_X
	func specialFeature() {
		fmt.Println("Special feature enabled")
	}
#end

# This is a regular comment
# Not a directive

func main() {
	fmt.Println("Hello, World!")
}`

	var comments []string
	errh := func(line, col uint, msg string) {
		if strings.HasPrefix(msg, "/") {
			// This is a comment
			fmt.Printf("Comment at line %d: %s\n", line, msg)
			comments = append(comments, msg)
		} else {
			fmt.Printf("Error at line %d, col %d: %s\n", line, col, msg)
		}
	}

	var s syntax.Scanner
	r := strings.NewReader(source)
	s.Init(r, errh, syntax.CommentsAndDirectives, "test.goo")

	for {
		s.Next()
		if s.Tok() == syntax.EOF {
			break
		}
	}

	fmt.Printf("\nFound %d comments/directives:\n", len(comments))
	for _, comment := range comments {
		fmt.Printf("  %s\n", comment)
	}
}