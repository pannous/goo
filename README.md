# The Goo Programming Language

Goo is an open source programming language that makes it easy to build simple, reliable, and efficient software.

Goo is an up-to-date fork of Go with the following modifications:
<!--
just like most ugliness in the world appears when you add a five to json(5) 
so does adding a little o to Go[o] make it a little more beautiful
-->
🗹 truthy/falsey if  
🗹 # comment and shebang support  
☐ not operator keyword for `!`
☐ and or not operators for && || !
☐ print alias for fmt.Println    
☐ goo file extension
☐ assert  
☐ return void, e.g. return print("ok")  
☐ silent/implicit error propagation    
☐ for loops    
☐ enums  
☐ any other pain points you and I might have  

![Gopher image](https://golang.org/doc/gopher/fiveyears.jpg)
*Gopher image by [Renee French][rf], licensed under [Creative Commons 4.0 Attribution license][cc4-by].*

Go's canonical Git repository is located at https://go.googlesource.com/go.
There is a mirror of the repository at https://github.com/golang/go.

Unless otherwise noted, the Go source files are distributed under the
BSD-style license found in the LICENSE file.

### Download and Install

#### Binary Distributions

TODO

#### Install From Source

```
git clone --recursive https://github.com/pannous/goo
cd goo/src
./make.bash
```

https://go.dev/doc/install/source for more source installation instructions.

[cc4-by]: https://creativecommons.org/licenses/by/4.0/
