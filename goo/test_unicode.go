package main

import "fmt"

// Test Unicode identifiers
var δ = 42
var 变量 = "Chinese variable"
var переменная = "Russian variable" 
var π = 3.14159

func 函数() string {
    return "Unicode function name"
}

func αβγ() int {
    return δ + 10
}

func main() {
    fmt.Println("Testing Unicode identifiers:")
    fmt.Printf("δ = %d\n", δ)
    fmt.Printf("变量 = %s\n", 变量)
    fmt.Printf("переменная = %s\n", переменная)
    fmt.Printf("π = %f\n", π)
    fmt.Printf("函数() = %s\n", 函数())
    fmt.Printf("αβγ() = %d\n", αβγ())
}