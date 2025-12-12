# The Goo Programming Language

```bash
brew tap pannous/goo https://github.com/pannous/goo
brew install goo
```

Goo is an open source programming language that makes it easy to build simple, reliable, and efficient software.  
  
Goo is an up-to-date fork of [Go](https://github.com/golang/go) with the following syntactic sugar on top:  
<!--  
just like most ugliness in the world appears when you add a five to json(5)  
so does adding a little o to Go[o] make everything a little more beautiful  
-->  
✅ if x {put("truthy")}   
✅ enum Status { OK, BAD } with generated .String() method  
✅ 3 ** 2 = 9  
✅ τ - π ≈ 3.14159  
✅ # comment and shebang support  
✅ #if DEBUG put("better than compiler tags!") #end  
✅ ø / ≠ / ¬ / not operator keyword for `nil` `!`  
✅ and or operators for && ||  
✅ no Main needed   ☐  implicit package main  
✅ printf as synonym for fmt.Println  with fmt as auto-import  
✅ typeof(x)  compile-time or runtime reflect.TypeOf(x).String()?  
✅ check 1>2   check keyword:  
✅ if $condition { panic($condition.text) } else { println("check OK", $condition.text) }  
✅ simple_list := [1,2,3]  // []any{1,2,3} or []int{1,2,3}  
✅ xs := ['a', 'b', 'c'] ; xs#1 == 'a'  // 1-indexed array access using # operator  
✅ [1, 2, 3].apply(x=>x * 2) == [2, 4, 6]   // 🙌 lambdas!  
✅ type check operator: 1 is int, [1, 2, 3] is []int, "hello" is string, 'a' is rune == True  
✅ try f()  -->   if err := f(); err != nil { panic(err) or return err }  
✅ try val := f()  -->  { val, err := f(); if err != nil { return err } }  
✅ try { x } catch e { y } =>  func() {defer func() {if e := recover(); e != nil {y} }() x } // x, y blocks :  
✅ try { panic("X") } catch x { printf("Caught: %v\n",x) }  // Todo catch returned errors?  
✅ go command `go test.go` --> defaults to `go run test.go`  
✅ go eval "2 ** 3" => 8  
✅ def as synonym for func, e.g. def main() { ... }  
✅ allow unused imports: as warning!  
✅ {a: 1, b: 2}  => map[string]int{"a": 1, "b": 2} auto-type inference  
✅ {a: 1, b: 2} == {"a": 1, "b": 2}   // symbol keys to strings  
✅ z := {a: 1, b: 2}; z.a == 1 and z.b == 2  // dot access to map keys  
✅ map[active:true age:30 name:Alice]  // read back print("%v") format  
✅ x:={a:1,b:2}; put(x) => fmt.Printf("%v\n",x)  
✅ [1,2]==[1,2]   test_list_comparison.goo  
✅ check "a"+1 == "a1"  
✅ check "a" == 'a'  
✅ check not x =>  !truthy(x)  
✅ declared and not used  make this a warning only (with flag to reenable error)  
✅ String methods "abc".contains("a")   reverse(), split(), join() …  
✅ 3.14 as string == "3.14"  
✅ 3.14 as int … semantic cast conversions  
✅ class Person {name string, age int}   person := Person{name: "Alice", age: 30}
✅ imported and not used only warning  
✅ return void, e.g. return print("ok") HARD  
✅ for i in 0…5 {put(i)}  // range syntax  
✅ "你" == '你'  
✅ def modify!(xs []int) { xs#1=0 } // modify in place enforced by "!" !  
✅ import "helper"  / "helper.goo" // allow local imports (for go run)  
✅ 1 in [1,2,3]   and  'e' in "hello"  // in operator for lists and strings and maps and iterators  
✅ Got rid of generated cancer files like op_string.go  token_string.go by stringer cancer 🤮🦀🤮  
✅ Universal for-in syntax:  
✅ for item in slice { ... }      // Values  
✅ for char in "hello" { ... }    // Characters  
✅ for key in myMap { ... }       // Keys  
✅ for item in iterator() { ... } // Iterator values  
✅ for k, v in myMap { ... }      // Key-value pairs  
✅ for i, v in slice { ... }      // Index-value pairs  
✅ for k, v in iterator() { ... } // Iterator pairs  
✅ while keyword as plain synonym for 'for'  
✅ check 500ms + 5s == 5500ms  
✅ 3**3 == 27  
✅ for i in 0…5 {put(i)}  // range loops now working!  
✅ goo file extension  
✅ func test() int { 42 } => func test() int { return 42 }  auto return  
☐ func test(){ return 42 } => func test() int { return 42 }  auto return (+ type inference)  
☐ func test(){ 42 } => func test() int { return 42 }  auto return (+ type inference)  
☐ check keyword works great, now let it emit debug message, e.g.  check 1>0  "check OK 1>0" via builtin println  
☐ runtime disable gc for extreme (resume?) performance, e.g. via `go run -gc=off test.go`  
☐ try x vs optional chaining via ?. operator, e.g. x?.y?.z => if not err{y.z}?  
☐ void(!) as synonym for func, e.g. void main(){} BAD  
☐ public() -> Public() calls OK // as compiler plugin?  
    Rust allows snake_case to call CamelCase methods via compiler desugaring, but warns.  
    Automatically detect if there is an uppercased public function available, if there is no private function with lowercase name.  
☐ silent/implicit error propagation  
instanceOf(a, reflect.TypeOf((*T)(nil)).Elem()) // or generated static checks  
  
☐ plugin.Open() is for loading .so files at runtime  
𐄂 AAA Game Engine Core? Never  
  
  
☐ while event := sdl.PollEvent(){} =>  for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {  
  
☐ GPU Intrinsics: forward []int{} vectors to GPU (simple primitive SIMD/CUDA/Metal/OpenCL adapters)  
☐ optional braces for function calls put 42 => put(42)      ambiguity resolution (e.g. put 42 + 3 vs put(42) + 3)  
  
  
x := 1  
y := "test"  
string interpolation:  
myAutoConcat := "The value of x is " x " and the value of y is " y  
myString := fmt.Sprint("The value of x is ", x, " and the value of y is ", y)  
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
  
<!--  
You can modify CLAUDE.md while the agent is working, it will pick up the changes!! "I see from the updated CLAUDE.md that…"  
-->  
☐ any other pain points you and I might have  
☐ cross off all done tasks from this list  
