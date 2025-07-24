package main

func main() {
	// Test 1-indexed array access with various operators
	z := []rune{'a', 'b', 'c'}
	nums := []int{10, 20, 30, 40}
	
	// Test with comparison operators (should have correct precedence)
	check z#1 == 'a'  // First element
	check z#2 == 'b'  // Second element  
	check z#3 == 'c'  // Third element
	check z#1 != 'b'  // Not equal
	check nums#1 < 15  // Less than
	check nums#2 > 15  // Greater than
	check nums#1 <= 10 // Less than or equal
	check nums#2 >= 20 // Greater than or equal
	
	// Test with arithmetic operators (# should have higher precedence)
	check nums#1 + 5 == 15    // Addition
	check nums#2 - 5 == 15    // Subtraction  
	check nums#1 * 2 == 20    // Multiplication
	check nums#2 / 2 == 10    // Division
	
	// Test with logical operators
	check (z#1 == 'a') && (z#2 == 'b')  // AND
	check (z#1 == 'a') || (z#2 == 'x')  // OR
	check !(z#1 == 'x')                 // NOT
	
	// Test with expressions as index
	idx := 2
	check z#idx == 'b'
	check z#(1+1) == 'b'
	
	// Test with function calls
	check z#len("x") == 'a' // len("x") == 1
	
	// Test with parentheses
	check (z#1) == 'a'
	check z#(1) == 'a'
	
	// Test with whitespace variations  
	check z #1 == 'a'
	check z# 1 == 'a'
	check z # 1 == 'a'
	
	// Test in different contexts
	// After semicolon
	_ = 1; check z#1 == 'a'
	
	// With tab
	check z#1	== 'a'
	
	// With newline
	check z#1 == 
		'a'
	
	// Start of line
	check
	z#1 == 'a'
}