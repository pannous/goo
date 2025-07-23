package main

import "fmt"

func main() {
	a := 5
	b := 10
	
	// Test ¬ operator with comparison
	if ¬(a == b) {
		fmt.Println("¬(a == b) works")
	}
}