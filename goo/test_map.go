package main

func main() {
	//z := map[string]int{"a": 1, "b": 2}
	z := map{"a": 1, "b": 2}
	//z := map{a: 1, b: 2}
	z["a"] = 10
	printf(z)
	printf(typeof(z))
	printf("Map z:", z)
	printf("Multiple", "args", 123, true)
}
