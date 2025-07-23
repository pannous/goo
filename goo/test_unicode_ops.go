package main

import "fmt"

func main() {
	a := 5
	b := 10
	
	// Test ≠ operator (should work like !=)
	if a ≠ b {
		fmt.Println("a ≠ b is true")
	}
	
	// Test ¬ operator (should work like !)
	if ¬(a == b) {
		fmt.Println("¬(a == b) is true")
	}
	
	// Test operators in expressions
	fmt.Printf("a ≠ b = %v\n", a ≠ b)
	fmt.Printf("¬(a > b) = %v\n", ¬(a > b))
	
	// Test with strings
	str1 := "hello"
	str2 := "world"
	if str1 ≠ str2 {
		fmt.Printf("%s ≠ %s is true\n", str1, str2)
	}
}