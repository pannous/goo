package main

import "fmt"

func main() {
	// Test without commas - the goal!
	m1 := map[x: 10 y: 20]
	fmt.Printf("No commas: %v\n", m1)

	// Test mixed - some commas, some not
	m2 := map[a: 1, b: 2 c: 3]
	fmt.Printf("Mixed commas: %v\n", m2)

	// Test with commas (backward compatibility)
	m3 := map[active: true, age: 30, name: "Alice"]
	fmt.Printf("With commas: %v\n", m3)

	// Single element without comma
	m4 := map[single: "value"]
	fmt.Printf("Single: %v\n", m4)

	// Multiple elements, no commas
	m5 := map[count: 42 enabled: true title: "test" score: 98.5]
	fmt.Printf("Multiple no commas: %v\n", m5)
}