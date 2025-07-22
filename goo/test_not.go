package main

import "fmt"

func main() {
	a := true
	b := false

	// Test basic not operator
	result := not a
	fmt.Println("not true =", result)
	
	result2 := not b
	fmt.Println("not false =", result2)
	
	// Test in conditional
	if not a {
		fmt.Println("not a is true")
	} else {
		fmt.Println("not a is false")
	}
	
	if not b {
		fmt.Println("not b is true")
	} else {
		fmt.Println("not b is false")
	}
}