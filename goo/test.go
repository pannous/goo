package main

import "fmt"
func main() {
	if true {
		fmt.Println("This is true")
	} else {
		fmt.Println("This is false")
	}
	//assert(true)
	//assert(false) // ./test.go:11:2: assert(false) failed OK
	write := fmt.Printf
	// Test numeric truthiness
	//x := tuple[int] (1,2,3)
	y1 := []int{1,2,3}
	write("List y1: %v", y1)
	y := [1,2,3]
	write("List y: %v", y)
	printf("List y: %v", y)
	printf("Type of y:", typeof(y))
	write("%v",y)

	//z := map[string]int{"a": 1, "b": 2}
	z := map{"a": 1, "b": 2}
	//z := map{a: 1, b: 2}
	z["a"] = 10
	write("%v",z)
	printf(z)
	printf(typeof(z))
	printf("Map z:", z)
	printf("Multiple", "args", 123, true)

	if 0 {
		printf("FAIL: 0 should be falsy")
	} else {
		printf("PASS: 0 is falsy")
	}
}
