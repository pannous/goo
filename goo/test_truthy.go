package main

import "fmt"

func main() {
	// Test numeric truthiness
	if 0 {
		fmt.Println("FAIL: 0 should be falsy")
	} else {
		fmt.Println("PASS: 0 is falsy")
	}

	if 42 {
		fmt.Println("PASS: 42 is truthy")
	} else {
		fmt.Println("FAIL: 42 should be truthy")
	}

	// Test string truthiness
	if "" {
		fmt.Println("FAIL: empty string should be falsy")
	} else {
		fmt.Println("PASS: empty string is falsy")
	}

	if "hello" {
		fmt.Println("PASS: non-empty string is truthy")
	} else {
		fmt.Println("FAIL: non-empty string should be truthy")
	}

	// Test slice truthiness
	var nilSlice []int
	if nilSlice {
		fmt.Println("FAIL: nil slice should be falsy")
	} else {
		fmt.Println("PASS: nil slice is falsy")
	}

	emptySlice := []int{}
	if emptySlice {
		fmt.Println("FAIL: empty slice should be falsy")
	} else {
		fmt.Println("PASS: empty slice is falsy")
	}

	nonEmptySlice := []int{1, 2, 3}
	if nonEmptySlice {
		fmt.Println("PASS: non-empty slice is truthy")
	} else {
		fmt.Println("FAIL: non-empty slice should be truthy")
	}

	// Test boolean (should work normally)
	if true {
		fmt.Println("PASS: true is truthy")
	} else {
		fmt.Println("FAIL: true should be truthy")
	}

	if false {
		fmt.Println("FAIL: false should be falsy")
	} else {
		fmt.Println("PASS: false is falsy")
	}
}
