package main

import "fmt"

func main() {
	// Test all existing syntaxes still work
	fmt.Println("=== Testing all syntax variations ===")
	
	// Original syntaxes
	z1 := map{"a": 1, "b": 2}
	z2 := {a: 1, b: 2}
	fmt.Printf("map{}: %v\n", z1)
	fmt.Printf("{}: %v\n", z2)

	// New bracket syntax - with commas (backward compatible)
	z3 := map[active: true, age: 30, name: "Alice"]
	fmt.Printf("map[] with commas: %v\n", z3)

	// New bracket syntax - without commas (new feature!)
	z4 := map[x: 10 y: 20 z: 30]
	fmt.Printf("map[] no commas: %v\n", z4)

	// Edge cases
	empty := map[]
	trailing := map[a: 1, b: 2,]
	trailingNoComma := map[a: 1 b: 2]
	
	fmt.Printf("Empty: %v\n", empty)
	fmt.Printf("Trailing comma: %v\n", trailing)
	fmt.Printf("No trailing comma: %v\n", trailingNoComma)

	fmt.Printf("All types: %T %T %T %T\n", z1, z2, z3, z4)
}