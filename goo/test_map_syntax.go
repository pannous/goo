package main

import "fmt"

func main() {
	// Test 1: map{"a": 1, "b": 2} syntax
	z1 := map{"a": 1, "b": 2}
	fmt.Printf("map{} syntax: %v\n", z1)

	// Test 2: {a: 1, b: 2} syntax (symbol keys to strings)
	z2 := {a: 1, b: 2}
	fmt.Printf("{} syntax: %v\n", z2)

	// Test 3: map[active:true age:30 name:Alice] syntax
	z3 := map[active: true, age: 30, name: "Alice"]
	fmt.Printf("map[] syntax: %v\n", z3)

	// Test mixed types
	z4 := map[count: 42, enabled: true, title: "test"]
	fmt.Printf("mixed types: %v\n", z4)
}