package main

import (
	"fmt"
	"reflect"
)

func main() {
	// Test the current issue
	explicit := {"x": 10, "y": 20}
	implicit := map[x:10 y:20]
	
	fmt.Printf("explicit: %v (type: %T)\n", explicit, explicit)
	fmt.Printf("implicit: %v (type: %T)\n", implicit, implicit)
	
	// Current solution using reflect.DeepEqual
	equal := reflect.DeepEqual(explicit, implicit)
	fmt.Printf("reflect.DeepEqual: %v\n", equal)
	
	// Test with different content
	different := map[a:1 b:2]
	equalDifferent := reflect.DeepEqual(explicit, different)
	fmt.Printf("Different maps equal: %v\n", equalDifferent)
	
	// Test with same content, different syntax
	same1 := map{"x": 10, "y": 20}
	same2 := {x: 10, y: 20}
	same3 := map[x:10 y:20]
	
	fmt.Printf("same1 == same2: %v\n", reflect.DeepEqual(same1, same2))
	fmt.Printf("same2 == same3: %v\n", reflect.DeepEqual(same2, same3))
	fmt.Printf("same1 == same3: %v\n", reflect.DeepEqual(same1, same3))
}