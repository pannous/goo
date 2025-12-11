package main

import "fmt"

// Test that normal Go code is not affected by check/as keywords
func main() {
	// Use "as" and "check" as regular identifiers
	as := "as_value"
	check := "check_value"

	fmt.Println(as, check)
}
