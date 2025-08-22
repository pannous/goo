package main

import "fmt"

func main() {
	x := 42
	y := 0
	
	// Test that runtime functions exist and are linked
	fmt.Println("Testing direct calls to runtime...")
	
	// For testing, let's see if we can call these at runtime
	// (This won't work directly since they're internal runtime functions)
	
	// Instead, let's test the behavior via the and operator
	result1 := x != 0
	fmt.Println("42 != 0:", result1)
	
	result2 := y != 0  
	fmt.Println("0 != 0:", result2)
}