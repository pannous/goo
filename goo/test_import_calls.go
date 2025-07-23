package main

import "fmt"

func main() {
	// Test calling an imported function
	fmt.Println("Hello from imported function!")
	
	// Test calling with multiple arguments
	fmt.Printf("Number: %d, String: %s\n", 42, "test")
}