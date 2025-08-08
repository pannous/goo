package main

import (
	"fmt"
	"strings"
)

func main() {
	result := strings.Contains("hello", "ell")
	fmt.Printf("Regular Go: %t\n", result)
}