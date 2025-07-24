package main

func main() {
	// Test 1-indexed array access
	z := []rune{'a', 'b', 'c'}
	check z#1 == 'a' // First element
	check z#2 == 'b' // Second element
	check z#3 == 'c' // Third element
	
	// Test with numbers
	nums := []int{10, 20, 30, 40}
	check nums#1 == 10
	check nums#4 == 40
	
	// Test with expressions
	idx := 2
	check z#idx == 'b'
}