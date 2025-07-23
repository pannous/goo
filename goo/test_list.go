package main

func main() {
	// Test [1,2,3] slice literal syntax
	z := [1, 2, 3]
	printf("Slice z:", z)
	printf("Type of z:", typeof(z))
	
	// Test accessing elements
	printf("First element:", z[0])
	printf("Second element:", z[1])
	
	// Test mixed types
	mixed := ["hello", 42, true]
	printf("Mixed slice:", mixed)
	printf("Type of mixed:", typeof(mixed))
	
	// Test empty slice (needs explicit type)
	empty := []int{}
	printf("Empty slice:", empty)
}