package main

import "fmt"

func main() {
	// Test the original issue
	explicit := {"x": 10, "y": 20}
	implicit := map[x:10 y:20]
	
	fmt.Printf("explicit: %v\n", explicit)
	fmt.Printf("implicit: %v\n", implicit)
	
	// The moment of truth!
	equal := explicit == implicit
	fmt.Printf("explicit == implicit: %v\n", equal)
	
	// Test different values
	different := map[a:1 b:2]
	fmt.Printf("explicit == different: %v\n", explicit == different)
	
	// Test same content, different syntax
	same1 := map{"x": 10, "y": 20}
	same2 := {x: 10, y: 20}
	same3 := map[x:10 y:20]
	
	fmt.Printf("same1 == same2: %v\n", same1 == same2)
	fmt.Printf("same2 == same3: %v\n", same2 == same3)
	fmt.Printf("same1 == same3: %v\n", same1 == same3)
	
	// Test empty maps
	empty1 := map{}
	empty2 := {}
	empty3 := map[]
	
	fmt.Printf("empty1 == empty2: %v\n", empty1 == empty2)
	fmt.Printf("empty2 == empty3: %v\n", empty2 == empty3)
	fmt.Printf("empty1 == empty3: %v\n", empty1 == empty3)
}