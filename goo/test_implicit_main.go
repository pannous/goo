package main
// ok via go run goo/test_implicit_main.go but not in GoLang :(

print("Hello, ")    // writes to stderr
println("world!")   // adds newline

x := 421
print("The answer is: ", x, "\n")
