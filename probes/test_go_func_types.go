package main

import (
	"fmt"
	"slices"
)

type User struct {
	Name string
	Age  int
}

func main() {
	users := []User{{Name: "Bob", Age: 17}, {Name: "Alice", Age: 20}}
	
	// Test 1: Named parameters (what our transformer generates)
	var namedFunc func(u User) bool = func(u User) bool {
		return u.Age > 18
	}
	
	// Test 2: Anonymous parameters (what Go expects)
	var anonFunc func(User) bool = func(u User) bool {
		return u.Age > 18
	}
	
	fmt.Printf("namedFunc type: %T\n", namedFunc)
	fmt.Printf("anonFunc type: %T\n", anonFunc)
	
	// Test which one works with slices.Filter
	filtered1 := slices.Filter(users, anonFunc)  // Should work
	fmt.Printf("filtered with anonFunc: %v\n", filtered1)
	
	filtered2 := slices.Filter(users, namedFunc)  // Might fail
	fmt.Printf("filtered with namedFunc: %v\n", filtered2)
}