package main

func main() {
	// Test numeric truthiness
	//x := tuple[int] (1,2,3)
	//y := list[int][1,2,3]
	//z := map[string]int{"a": 1, "b": 2}
	z := map{"a": 1, "b": 2}
	//z := map{a: 1, b: 2}
	z["a"] = 10
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
