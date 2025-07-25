package main

import "fmt"

func main() {
	// Test current slice comparison behavior
	list1 := []int{1, 2, 3}
	list2 := []int{1, 2, 3}
	
	fmt.Printf("list1: %v\n", list1)
	fmt.Printf("list2: %v\n", list2)
	
	// This should currently give a compilation error
	equal := list1 == list2
	fmt.Printf("list1 == list2: %v\n", equal)
}