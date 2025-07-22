package main

import "fmt"

func main() {
	a := true
	
	// Test basic not operator
	result := not a
	fmt.Println("not true =", result)
	
	b := false
	result2 := not b
	fmt.Println("not false =", result2)
}