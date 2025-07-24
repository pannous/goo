package main

import "fmt"

func main() {
	z := map{"a": 1, "b": 2}
	printf("printf output:", z)
	print("print output:", z)
	fmt.Println("fmt.Println output:", z)
}