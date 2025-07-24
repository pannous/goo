package main

import "fmt"

func main() {
	// Test with identical maps
	m1 := map[x:10]
	m2 := map[x:10]
	
	fmt.Printf("m1: %v (addr: %p)\n", m1, m1)
	fmt.Printf("m2: %v (addr: %p)\n", m2, m2)
	fmt.Printf("m1 == m2: %v\n", m1 == m2)
	
	// Test same reference
	m3 := m1
	fmt.Printf("m1 == m3: %v\n", m1 == m3)
	
	// Test nil comparison (should still work)
	var nilMap map[any]any
	fmt.Printf("m1 == nil: %v\n", m1 == nilMap)
	fmt.Printf("nilMap == nil: %v\n", nilMap == nilMap)
}