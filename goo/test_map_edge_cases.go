package main

import "fmt"

func main() {
	// Test empty maps
	empty1 := map{}
	empty2 := {}
	empty3 := map[]
	fmt.Printf("Empty maps: %v %v %v\n", empty1, empty2, empty3)

	// Test single element
	single := map[key: "value"]
	fmt.Printf("Single element: %v\n", single)

	// Test trailing comma
	trailing := map[a: 1, b: 2,]
	fmt.Printf("Trailing comma: %v\n", trailing)

	// Verify all are map[any]any type
	fmt.Printf("Types: %T %T %T\n", empty1, empty2, empty3)
}