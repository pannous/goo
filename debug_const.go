package main

type Status int
const (
	OK Status = iota
	ERROR
)

func main() {
	println("OK =", OK)
	println("ERROR =", ERROR)
}