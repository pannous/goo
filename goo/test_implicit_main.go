#!/opt/other/go/bin/go run 
package main
// ok via bin/go run goo/test_implicit_main.go but not in GoLang :(
func helper() int {
	return 42
}
print("Hello, ")    // writes to stderr
println("world!")   // adds newline
printf("The answer is: ", helper()) // UN-formatted output

x := 421
print("The answer is: ", x, "\n")
