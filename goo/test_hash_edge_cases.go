package main

import "fmt"

# Comment at start of line
func main() {
	# Indented comment
	x := "string with # inside"
	y := `raw string with # inside`
	fmt.Println(x, y)

	# Final comment
}
