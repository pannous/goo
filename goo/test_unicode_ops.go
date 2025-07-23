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
	
	// Test individual operators
	fmt.Printf("a ≠ b = %v\n", a ≠ b)
	fmt.Printf("¬(a > b) = %v\n", ¬(a > b))
}