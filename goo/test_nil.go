package main

import "fmt"

func main() {
	var ptr *int
	if ptr == ø {
		fmt.Println("ptr is nil using ø")
	}
	
	var slice []int
	if slice == ø {
		fmt.Println("slice is nil using ø")
	}
	
	var m map[string]int
	if m == ø {
		fmt.Println("map is nil using ø")
	}
	
	// Test assignment
	ptr = ø
	if ptr == ø {
		fmt.Println("ptr assigned to ø works")
	}
	
	// Test return
	result := getPtr()
	if result == ø {
		fmt.Println("function returned ø")
	}
}

func getPtr() *int {
	return ø
}