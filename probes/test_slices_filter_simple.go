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
	
	// Test what slices.Filter expects
	filtered := slices.Filter(users, func(u User) bool {
		return u.Age > 18
	})
	
	fmt.Printf("filtered: %v\n", filtered)
}