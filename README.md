# The Goo Programming Language  
  
Goo is an open source programming language that makes it easy to build simple, reliable, and efficient software.  
  
Goo is an up-to-date fork of Go with the following modifications:  
<!--  
just like most ugliness in the world appears when you add a five to json(5)   
so does adding a little o to Go[o] make everything a little more beautiful  
-->  
✅ truthy/falsey if    
✅ # comment and shebang support    
✅ goo file extension    
✅ ø / ≠ / ¬ / not operator keyword for `nil` `!`    
✅ and or operators for && ||  
✅ no Main needed ☐  implicit package main    
✅ printf as synonym for fmt.Println  with fmt as auto-import (similar to OPRINTLN|OPRINT?)  
✅ typeof(x)  compile-time or runtime reflect.TypeOf(x).String()?  
✅ check 1>2 // check keyword: if not truthy($condition) { panic($condition.text) } else { println("check OK", $condition.text) }  
✅ z := [1,2,3]  // []any{1,2,3} or []int{1,2,3}  
✅ z := ['a', 'b', 'c'] ; z#1 == 'a'  // 1-indexed array access using # operator  
✅ Get rid of generated cancer files like op_string.go  token_string.go by stringer cancer 🤮🦀🤮  
✅ go command default to run => `go test.go` OK
✅ def as synonym for func, e.g. def main() { ... }  
✅ allow unused imports: as warning!  
✅ z := map{"a": 1, "b": 2}  => map[any]any{…}  
✅ z := {a: 1, b: 2}  // symbol keys to strings => z := {"a": 1, "b": 2}  
✅ map[active:true age:30 name:Alice]   
✅ test_list_comparison.goo [1,2]==[1,2]  
✅ x:={a:1,b:2}; put(x) => fmt.Printf("%v\n",x)
✅ enum Status { OK, BAD } with generated .String() method 

☐ import "helper.go"
☐ runtime disable gc for extreme (resume?) performance, e.g. via `go run -gc=off test.go`
☐ GPU Intrinsics: forward []int{} vectors to GPU (simple primitive SIMD/CUDA/Metal/OpenCL adapters)
☐ optional braces for function calls put 42 => put(42)   HARD?
☐ optional chaining via ?. operator, e.g. x?.y?.z => if not err{y.z}?
☐ check keyword works great, now let it emit debug message, e.g.  check 1>0  "check OK 1>0" via builtin println   
☐ map can only be compared to nil {a: 1, b: 2} == {b: 2, a: 1} HARD  
☐ for loops  :    
☐ for keyword := keywords  => for _, keyword := range keywords { __
☐ String methods "abc".contains("a")  1. real 2. by compiler 
☐ return void, e.g. return print("ok") HARD    
☐ void(!) as synonym for func, e.g. void main(){} BAD  
☐ public() -> Public() calls OK // as compiler plugin?  
    Rust allows snake_case to call CamelCase methods via compiler desugaring, but warns.  
    Automatically detect if there is an uppercased public function available, if there is no private function with lowercase name.  
☐ silent/implicit error propagation  
☐ a is Type for type assertion, e.g. if a is int {} => if _, ok := a.(int); ok { ... }
☐ func test() int { 42 } => func test() int { return 42 }  auto return 
☐ func test(){ 42 } => func test() int { return 42 }  auto return (+ type inference)
☐ class via struct (!)    
☐ plugin.Open() is for loading .so files at runtime
☐ imported and not used only warning   
☐ cross off all done tasks from this list    
☐ any other pain points you and I might have     
𐄂 AAA Game Engine Core? Never

  
x := 1
y := "test"
myString := fmt.Sprint("The value of x is ", x, " and the value of y is ", y)
myAutoConcat := "The value of x is " x " and the value of y is " y
myTemplate := `The value of x is ${x} and the value of y is ${y}!`

  
![Gopher image](https://golang.org/doc/gopher/fiveyears.jpg)  
*Gopher image by Renee French, licensed under Creative Commons 4.0 Attribution license  
  
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
export GOROOT=/usr/local/goo/  
```  
  
#### Install From Source  
  
```  
git clone --recursive https://github.com/pannous/goo  
cd goo/src  
./make.bash  
```

https://go.dev/doc/install/source for more source installation instructions.  

### Test new features  
```  
./bin/go run goo/test.goo
```
All new features tested in goo [folder](https://github.com/pannous/goo/tree/master/goo)

Todo: Web Demo  
