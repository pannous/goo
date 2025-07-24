package main

import (
	"fmt"
	"go/token"
)

func main() {
	fmt.Printf("FUNC: %d\n", token.FUNC)
	fmt.Printf("VAR: %d\n", token.VAR)
	fmt.Printf("FN: %d\n", token.FN)
}