package main

import "fmt"

func main() {
	fmt.Println("Hello, stringer-free world!")
	
	// Test some control flow that would trigger the branch operations
	for i := 0; i < 3; i++ {
		if i == 1 {
			continue
		}
		fmt.Printf("i = %d\n", i)
	}
	
	fmt.Println("Success!")
}