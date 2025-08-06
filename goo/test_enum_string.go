package main

import "fmt"

enum State { OK, ERROR, PENDING }

func main() {
	fmt.Println("Testing enum String() method:")
	
	// Test normal cases
	var s State
	s = OK
	fmt.Printf("OK.String() = %v\n", s)
	
	s = ERROR
	fmt.Printf("ERROR.String() = %s\n", s.String())
	
	s = PENDING
	fmt.Printf("PENDING.String() = %s\n", s.String())
	
	// Test unknown value (should return "UNKNOWN")
	s = State(999)
	fmt.Printf("State(999).String() = %s\n", s.String())
	
	// Test direct string formatting
	fmt.Printf("Direct formatting: %s\n", OK)
}