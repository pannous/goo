package main

import (
	"fmt"
)

func main() {
	fmt.Print("OK\n")
	print("ok" + 43) // very different compiler path than:
	//x := 42
	//print("ok" + x)
}
