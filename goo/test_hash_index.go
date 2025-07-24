package main

func main() {
	// Test that regular 0-indexed access still works
	z := []rune{'a', 'b', 'c'}
	
	// 0-indexed (traditional Go)
	check z[0] == 'a'
	check z[1] == 'b'
	check z[2] == 'c'
	
	// 1-indexed (new Goo feature)
	check z#1 == 'a'
	check z#2 == 'b'
	check z#3 == 'c'
	
	// Both should give same results
	check z[0] == z#1
	check z[1] == z#2
	check z[2] == z#3
}