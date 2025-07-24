package main

func main() {
	z := []rune{'a', 'b', 'c'}
	
	// This works
	a := z#1
	println("z#1 =", a)
	
	// This fails  
	// b := z#1 == 'a'
	
	// Let's try step by step
	first := z#1
	second := first == 'a'
	println("step by step:", second)
}