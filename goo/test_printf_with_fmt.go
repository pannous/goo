package main

import "fmt"

func main() {
	z := map{"a": 1, "b": 2}
	
	fmt.Println("=== Testing printf (should use fmt.Printf when fmt is imported) ===")
	printf("printf output:", z)
	
	fmt.Println("=== Direct fmt.Printf for comparison ===")  
	fmt.Printf("fmt.Printf output: %v\n", z)
}