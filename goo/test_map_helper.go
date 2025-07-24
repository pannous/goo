package main

import "fmt"

// Custom helper function for deep map equality
func mapsEqual(a, b map[any]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		if vb, ok := b[k]; !ok || va != vb {
			return false
		}
	}
	return true
}

func main() {
	// Test the original issue with helper function
	explicit := {"x": 10, "y": 20}
	implicit := map[x:10 y:20]
	
	fmt.Printf("explicit: %v\n", explicit)
	fmt.Printf("implicit: %v\n", implicit)
	
	// Using helper function
	equal := mapsEqual(explicit, implicit)
	fmt.Printf("mapsEqual(explicit, implicit): %v\n", equal)
	
	// Test different values
	different := map[a:1 b:2]
	fmt.Printf("mapsEqual(explicit, different): %v\n", mapsEqual(explicit, different))
	
	// Test same content, different syntax
	same1 := map{"x": 10, "y": 20}
	same2 := {x: 10, y: 20}
	same3 := map[x:10 y:20]
	
	fmt.Printf("mapsEqual(same1, same2): %v\n", mapsEqual(same1, same2))
	fmt.Printf("mapsEqual(same2, same3): %v\n", mapsEqual(same2, same3))
	fmt.Printf("mapsEqual(same1, same3): %v\n", mapsEqual(same1, same3))
	
	// Test empty maps
	empty1 := map{}
	empty2 := {}
	empty3 := map[]
	
	fmt.Printf("mapsEqual(empty1, empty2): %v\n", mapsEqual(empty1, empty2))
	fmt.Printf("mapsEqual(empty2, empty3): %v\n", mapsEqual(empty2, empty3))
	fmt.Printf("mapsEqual(empty1, empty3): %v\n", mapsEqual(empty1, empty3))
}