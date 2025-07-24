package main

import "fmt"

func main() {
	z := map{"a": 1, "b": 2}
	fmt.Println("fmt.Println output:", z) // map[a:1 b:2] OK
	printf("printf output:", z) // should be map[a:1 b:2] too, not pointer!!
}