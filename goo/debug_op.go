package main

import (
	"fmt"
	"cmd/compile/internal/ir"
)

func main() {
	fmt.Printf("OCASE value: %d\n", int(ir.OCASE))
	fmt.Printf("OCASE string: %s\n", ir.OCASE.String())
	fmt.Printf("OpStringNames length: %d\n", len(ir.OpStringNames))
	if int(ir.OCASE) < len(ir.OpStringNames) {
		fmt.Printf("OpStringNames[OCASE]: %s\n", ir.OpStringNames[ir.OCASE])
	} else {
		fmt.Printf("OCASE index %d is out of range for OpStringNames\n", int(ir.OCASE))
	}
}