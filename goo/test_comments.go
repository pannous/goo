#!/opt/other/go/bin/go run
#!/usr/bin/env goo run  // TODO <<< ^^
# This is a hash comment
package main

import "fmt"

func main() {
	fmt.Println("Comment #support working!")
	// Regular Go comment
	/* Block comment */
	# Another hash comment
	fmt.Println("All #comment types work!")
}