package main

import "fmt"

func main() {
	write := fmt.Println
	// Test numeric truthiness
	//x := tuple[int] (1,2,3)
	//y := list[int][1,2,3]
	//z := map[string]int{"a": 1, "b": 2}
	z := map{"a": 1, "b": 2}
	z["a"] = 10
	write(z)
	write(type(z))
	write(typeof(z))
	//printf(z)

	if 0 {
		write("FAIL: 0 should be falsy")
	} else {
		write("PASS: 0 is falsy")
	}
}
