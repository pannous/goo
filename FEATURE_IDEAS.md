if name, ok := receiver.(*syntax.Name); ok {
if name ?= receiver.(*syntax.Name) { ???

What are some features that are missing on the list according to your Opinion

⏺ Looking at the README.md list, here are some significant missing features that would make Goo even more
  powerful and developer-friendly:

  High-Impact Missing Features:

  1. Pattern Matching / Switch Expressions

  // Current Go switch is verbose
  switch x := getValue().(type) {
  case int: return x * 2
  case string: return len(x)
  default: return 0
  }

  // Better: Pattern matching
  result := match getValue() {
    int(x) => x * 2
    string(s) => len(s)
    _ => 0
  }

  2. Destructuring Assignment

  // Instead of:
  user := getUser()
  name := user.Name
  age := user.Age

  // Clean destructuring:
  {name, age} := getUser()
  [first, second, ...rest] := getList()

  3. Null Coalescing Operator ??

  // Instead of:
  var result string
  if user != nil && user.Name != "" {
      result = user.Name
  } else {
      result = "Anonymous"
  }

  // Clean:
  result := user?.Name ?? "Anonymous"

  4. Range Expressions & Iterators

  // Missing: Easy range creation
  for i in 0..10 { ... }           // Range syntax
  for i in 0..<10 { ... }          // Exclusive range
  for x in fibonacci(10) { ... }   // Custom iterators

  5. Async/Await (Alternative to Goroutines)

  // More familiar async pattern alongside goroutines
  async func fetchData() string { ... }

  result := await fetchData()  // Instead of channels

  6. Union Types

  // Instead of interface{}
  type Result = string | int | error

  func process() Result {
      if condition { return "success" }
      if other { return 42 }
      return errors.New("failed")
  }

  7. Tuple Types & Multiple Return Sugar

  // Instead of: func divide(a, b int) (int, error)
  func divide(a, b int) (int, error) { ... }

  // Cleaner tuple syntax:
  func divide(a, b int) -> (int, error) { ... }
  (result, err) := divide(10, 2)

  8. Immutability Keywords

  // Missing: Built-in immutability
  const user = User{Name: "Alice"}  // Deep immutability
  let data = [1, 2, 3]              // Immutable collections

  9. Smart Casting (Type Guards)

  // Instead of repeated type assertions
  if user, ok := x.(User); ok {
      if admin, ok := user.(Admin); ok {
          admin.doAdminStuff()
      }
  }

  // Smart casting:
  if x is User && x is Admin {
      x.doAdminStuff()  // x automatically cast to Admin
  }

  10. Collection Methods/LINQ-style Operations

  // Missing: Built-in collection operations
  users.filter(u => u.Age > 18)
       .apply(u => u.Name)
       .sort()
       .first()

  // Or: Method chaining on slices
  names := users.Filter(adult).Map(getName).Sort().Take(5)

  Quality of Life Features:

  11. Default Parameters

  func greet(name string, prefix = "Hello") string {
      return prefix + " " + name
  }

  12. Named Parameters

  createUser(name: "Alice", age: 30, admin: true)

  13. Trailing Commas Everywhere

  // Should work in all contexts
  func call(
      a int,
      b string, // <- trailing comma
  ) { }

  14. Conditional Compilation Tags

  // More ergonomic than build tags
  #if DEBUG
      println("Debug mode")
  #endif

  My Top 3 Recommendations:

  1. Null Coalescing ?? - Natural complement to your ?. operator
  2. Pattern Matching - Would make Goo feel truly modern
  3. Union Types - Solve Go's biggest type system limitation

  These would make Goo feel like a "Go 2.0" with modern language features while keeping Go's simplicity and
  performance!

  Which of these resonates most with your vision for Goo?