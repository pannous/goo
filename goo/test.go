package main

import "fmt"

func main() {
	// Test numeric truthiness
	if 0 {
		fmt.Println("FAIL: 0 should be falsy")
	} else {
		fmt.Println("PASS: 0 is falsy")
	}
}
