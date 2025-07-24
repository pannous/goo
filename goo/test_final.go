package main

func main() {
	// Test 1-indexed array access
	z := []rune{'a', 'b', 'c'}
	nums := []int{10, 20, 30, 40}
	
	// Basic functionality  
	check z#1 == 'a'  // First element
	check z#2 == 'b'  // Second element
	check z#3 == 'c'  // Third element
	
	// With numbers
	check nums#1 == 10
	check nums#4 == 40
	
	// With expressions as index
	idx := 2
	check z#idx == 'b'
	check z#(1+1) == 'b'
	
	// Precedence with various operators
	check nums#1 + 5 == 15    // Addition
	check nums#2 - 5 == 15    // Subtraction  
	check nums#1 * 2 == 20    // Multiplication
	check nums#2 / 2 == 10    // Division
	check nums#1 < 15         // Comparison
	check nums#2 > 15         // Comparison
	
	// Logical operators
	check (z#1 == 'a') && (z#2 == 'b')  // AND
	check (z#1 == 'a') || (z#2 == 'x')  // OR
	
	// Different whitespace contexts
	check z #1 == 'a'
	check z# 1 == 'a'  
	check z # 1 == 'a'
}