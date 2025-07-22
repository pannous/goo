package main

import "fmt"

func main() {
	a := true
	b := false
	
	// Test traditional ! operator
	fmt.Println("!a =", !a)
	fmt.Println("!b =", !b)
	
	// Test new not keyword
	fmt.Println("not a =", negate a)
	fmt.Println("not b =", negate b)
	
	// Test in conditional
	if negate a {
		fmt.Println("not a is true")
	} else {
		fmt.Println("not a is false")
	}
	
	if negate b {
		fmt.Println("not b is true")
	} else {
		fmt.Println("not b is false")
	}
}