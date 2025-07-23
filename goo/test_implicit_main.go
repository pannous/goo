package main

import "fmt"

// This file tests implicit main function generation
// Without the explicit main function, these statements
// would be wrapped in an implicit main() function

fmt.Println("Hello from implicit main!")
fmt.Println("No explicit main function needed")

x := 42
fmt.Printf("The answer is: %d\n", x)