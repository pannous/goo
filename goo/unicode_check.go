package main

import (
	"fmt"
	"unicode"
)

func main() {
	fmt.Printf("≠ IsLetter: %v, IsSymbol: %v\n", unicode.IsLetter('≠'), unicode.IsSymbol('≠'))
	fmt.Printf("¬ IsLetter: %v, IsSymbol: %v\n", unicode.IsLetter('¬'), unicode.IsSymbol('¬'))
}