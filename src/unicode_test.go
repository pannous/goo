package main

import "fmt"
import "strings"

func main() {
	s := "m²"
	fmt.Printf("Length: %d\n", len(s))
	fmt.Printf("s[:1]: '%s'\n", s[:1])
	fmt.Printf("s[:len(s)-1]: '%s'\n", s[:len(s)-1])
	
	// Test the actual condition
	baseUnit := "m²"
	otherUnit := "m"
	fmt.Printf("HasSuffix('%s', '²'): %v\n", baseUnit, strings.HasSuffix(baseUnit, "²"))
	prefix := baseUnit[:len(baseUnit)-1]
	fmt.Printf("Prefix: '%s'\n", prefix)  
	fmt.Printf("'%s' == '%s': %v\n", otherUnit, prefix, otherUnit == prefix)
}