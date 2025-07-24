# The Goo Programming Language

Goo is an open source programming language that makes it easy to build simple, reliable, and efficient software.

Goo is an up-to-date fork of Go with the following modifications:
<!--
just like most ugliness in the world appears when you add a five to json(5) 
so does adding a little o to Go[o] make everything a little more beautiful
-->
🗹 truthy/falsey if  
🗹 # comment and shebang support  
🗹 goo file extension  
🗹 and or operators for && ||
🗹 ¬ / not operator keyword for `!`  
🗹 ≠ operator for `!=`  
🗹 ø keyword for `nil`
🗹 no Main needed ☐  implicit package main  
🗹 printf as synonym for fmt.Println  with fmt as auto-import (similar to OPRINTLN|OPRINT?)
🗹 typeof(x)  compile-time or runtime reflect.TypeOf(x).String()?
🗹 check 1>2 // check keyword: if not truthy($condition) { panic($condition.text) } 
🗹 z := map{"a": 1, "b": 2}  => map[any]any{…}
🗹 z := {a: 1, b: 2}  // symbol keys to strings => z := {"a": 1, "b": 2}
🗹 z := [1,2,3]  // []any{1,2,3} or []int{1,2,3}
🗹 z := ['a', 'b', 'c'] ; z#1 == 'a'  // 1-indexed array access using # operator
🗹 Get rid of generated cancer files like op_string.go  token_string.go by stringer cancer tool 🤮🦀🤮
☐ for keyword := keywords  => for _, keyword := range keywords {
☐ String methods "abc".contains("a")
☐ return void, e.g. return print("ok")  
☐ go command should default to run, so go test.go should work  
☐ def as synonym for func, e.g. def main() { ... }  
☐ void(!) as synonym for func, e.g. void main(){}
☐ public() -> Public() calls OK // as compiler plugin?
    Rust allows snake_case to call CamelCase methods via compiler desugaring, but warns.
    Automatically detect if there is an uppercased public function available, if there is no private function with lowercase name.
☐ silent/implicit error propagation
☐ for loops    
☐ enums via struct or const ( ILLEGAL Token = iota
☐ class via struct  
☐ imported and not used only warning 
☐ cross off all done tasks from this list  
☐ any other pain points you and I might have   
☐ map can only be compared to nil

## New Features

### 1-Indexed Array Access (`#` operator)

Goo supports 1-indexed array access using the `#` operator as an alternative to traditional 0-indexed bracket notation:

```go
z := []rune{'a', 'b', 'c'}
first := z#1   // Gets 'a' (first element)
second := z#2  // Gets 'b' (second element) 
third := z#3   // Gets 'c' (third element)

// Works with any expression as index
idx := 2
char := z#idx  // Gets 'b'
char := z#(1+1) // Gets 'b'

// Correct precedence with operators
check z#1 == 'a'        // Parses as (z#1) == 'a'
result := z#1 + 5       // Works with arithmetic
valid := z#1 < 'z'      // Works with comparisons
```

**Implementation:** The `#` operator is converted at parse time to `[index-1]`, so `z#1` becomes `z[0]`, maintaining full compatibility with Go's type system and performance characteristics.

**Context-sensitive parsing:** The `#` character is treated as:
- **1-indexed operator** when following an identifier/expression: `z#1`
- **Comment start** when at beginning of line: `# This is a comment`

![Gopher image](https://golang.org/doc/gopher/fiveyears.jpg)
*Gopher image by [Renee French][rf], licensed under [Creative Commons 4.0 Attribution license][cc4-by].*

Go's canonical Git repository is located at https://go.googlesource.com/go.
There is a mirror of the repository at https://github.com/golang/go.

Unless otherwise noted, the Go source files are distributed under the
BSD-style license found in the LICENSE file.

### Download and Install

#### Binary Distributions

Official binary distributions are available at https://github.com/pannous/goo/releases.

Download the archive file appropriate for your installation and operating system. Extract it to `/usr/local` (you may need to run the command as root or through sudo):

```
tar -C /usr/local -xzf goo1.x.x.linux-amd64.tar.gz
```

Add `/usr/local/goo/bin` to your PATH environment variable. You can do this by adding this line to your `$HOME/.profile` or `/etc/profile` (for a system-wide installation):

```
export PATH=$PATH:/usr/local/goo/bin
```

#### Install From Source

```
git clone --recursive https://github.com/pannous/goo
cd goo/src
./make.bash
```

https://go.dev/doc/install/source for more source installation instructions.

[cc4-by]: https://creativecommons.org/licenses/by/4.0/
