package main

import "fmt"

enumerate Token {
	ILLEGAL
	EOF
	IDENT
	NUMBER
}

func main() {
	fmt.Printf("ILLEGAL = %v\n", ILLEGAL)
	fmt.Printf("EOF = %v\n", EOF)  
	fmt.Printf("IDENT = %v\n", IDENT)
	fmt.Printf("NUMBER = %v\n", NUMBER)
	
	var t Token = EOF
	fmt.Printf("Token value: %v\n", t)
}