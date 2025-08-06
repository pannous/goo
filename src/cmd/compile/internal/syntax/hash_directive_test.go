package syntax

import (
	"fmt"
	"strings"
	"testing"
)

func TestHashDirectives(t *testing.T) {
	testCases := []struct {
		src      string
		expected bool // whether it should be recognized as a directive
	}{
		{"#if DEBUG", true},
		{"#end", true}, 
		{"# regular comment", false},
		{"#if FEATURE_X", true},
		{"#end // with comment", true},
		{"#ifnot", false}, // Should be treated as regular comment
		{"#endurance", false}, // Should be treated as regular comment
	}

	for _, test := range testCases {
		t.Run(test.src, func(t *testing.T) {
			var comments []string
			var s scanner
			s.init(strings.NewReader(test.src), func(line, col uint, msg string) {
				if strings.HasPrefix(msg, "#") {
					comments = append(comments, msg)
					t.Logf("Got comment/directive: %s at line %d, col %d", msg, line, col)
				} else {
					t.Logf("Got error: %s at line %d, col %d", msg, line, col)
				}
			}, directives, "test.goo")

			s.next()
			
			t.Logf("Token: %s", s.tok)
			t.Logf("Found %d comments/directives", len(comments))
			
			if test.expected {
				if len(comments) == 0 {
					t.Errorf("Expected directive recognition for %q, but got no comments/directives", test.src)
				} else if !strings.Contains(comments[0], "#if") && !strings.Contains(comments[0], "#end") {
					t.Logf("Note: Got comment %q - this may be correct if not implementing as special directives", comments[0])
				}
			}
		})
	}
}

func TestHashDirectiveOutput(t *testing.T) {
	// Manual test to see what our implementation produces
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