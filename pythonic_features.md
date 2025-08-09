# Python-like Features in Goo Language

## **Missing Python-like Features**

### **1. List Comprehensions**
```python
# Python
squares = [x*x for x in range(10)]
filtered = [x for x in numbers if x > 5]
```
Goo currently has `.filter()` and `.apply()` but no comprehension syntax.

### **2. Dictionary/Map Comprehensions**
```python
# Python  
squares = {x: x*x for x in range(5)}
```

### **3. Tuple Operations**
```python
# Python
a, b = b, a  # tuple unpacking/swapping
x, y, z = get_coordinates()  # multiple assignment
```

### **4. String Slicing with Step**
```python
# Python
"hello"[::-1]  # reverse with step
"hello"[::2]   # every 2nd character
```
Goo has basic slicing but no step parameter.

### **5. Range Function**
```python
# Python
for i in range(10):        # 0 to 9
for i in range(5, 10):     # 5 to 9  
for i in range(0, 10, 2):  # 0,2,4,6,8
```

### **6. Enumerate Function**
```python
# Python
for i, item in enumerate(items):
```

### **7. Zip Function**
```python
# Python
for a, b in zip(list1, list2):
```

### **8. List Methods**
- `.extend()` - extend with another list
- `.insert(index, item)` - insert at position
- `.remove(item)` - remove first occurrence
- `.pop(index)` - remove and return item
- `.count(item)` - count occurrences
- `.index(item)` - find index (with error if not found)

### **9. Set Operations**
```python
# Python
{1, 2, 3} & {2, 3, 4}  # intersection
{1, 2, 3} | {2, 3, 4}  # union
```

### **10. Generator Expressions**
```python
# Python
sum(x*x for x in range(10))
```

### **11. String Formatting**
```python
# Python
f"Hello {name}, you are {age} years old"
"Hello {} {}".format(first, last)
```

### **12. Context Managers (with statement)**
```python
# Python
with open("file.txt") as f:
    content = f.read()
```

## **Already Implemented Python-like Features**

### **1. String Methods and Operations**

**String Methods** (from `test_string_methods.goo`):
- `.first()` - get first character
- `.last()` - get last character  
- `.size()` / `.length()` - string length
- `.reverse()` - reverse string
- `.contains(substr)` - check substring existence
- `.indexOf(substr)` - find substring position (-1 if not found)
- `.from(index)` - substring from index
- `.to(index)` - substring to index
- `.sub(start, end)` - substring (inclusive start, exclusive end)
- `.replace(old, new)` - string replacement
- `.toUpper()` / `.upper()` / `.upperCase()` - uppercase conversion
- `.toLower()` / `.lower()` / `.lowerCase()` - lowercase conversion
- `.capitalize()` / `.title()` - capitalize first letter
- `.trim()` - trim whitespace
- `.join(separator)` - join characters with separator
- `.startsWith(prefix)` - check prefix
- `.endsWith(suffix)` - check suffix
- `.split(delimiter)` - split into array
- `.splits()` - split into character array

**String Operations**:
- String concatenation: `"a" + "b"` and `"a" + 1` (auto-conversion)
- String interpolation: `"left" variable "right"` syntax
- String indexing and slicing: `str[1:]`, `str[:2]`, etc.
- Unicode character access: `"你好"#2 == '好'`, `变量#-1 == '量'`

### **2. List/Array Methods**

**List Methods** (from `test_list_methods.goo`):
- `.first()` - get first element
- `.last()` - get last element
- `.size()` / `.length()` - array length
- `.contains(item)` - check membership
- `.indexOf(item)` - find element position (-1 if not found)
- `.sort()` - sort in place
- `.slice(start, end)` - slice operation
- `.copy()` - create copy
- `.append(item)` - add element
- `.join(separator)` - join elements (for string arrays)
- `.filter(lambda)` - filter with lambda expression
- `.apply(lambda)` - map operation with lambda (equivalent to Python's map)

**List Operations**:
- Array indexing: `arr[1]` and `arr#2` (1-based indexing option)
- Array slicing: `arr[1:]`, `arr[:2]`, `arr[1:2]`
- Array literals: `[1, 2, 3]`

### **3. Control Flow Features**

**For-in Loops** (from `test_for_in_key_value.goo`):
- Single variable iteration: `for item in array`
- Key-value iteration: `for key, value in map`
- Index-value iteration: `for i, item in array`

**While Loops** (from `test_while_loops.goo`):
- Basic while: `while condition { ... }`
- While-true with break: `while true { ... break ... }`
- While with for-in syntax: `while item in array`
- While with key-value: `while key, value in map`

**Exception Handling** (from `test_try_catch.goo`):
- Try-catch blocks: `try { ... } catch err { ... }`

### **4. Operators and Syntax Sugar**

**Logical Operators** (from `test_and_or.goo`):
- `and` operator (equivalent to `&&`)
- `or` operator (equivalent to `||`) 
- `not` operator (equivalent to `!`)

**Membership Operator** (`in` operator):
- String containment: `"hello" in "hello world"`
- Array membership: `2 in [1,2,3]`
- Map key membership: `key in map`
- Works with variables and literals

**Truthiness** (from `test_truthy.goo`):
- Numbers: 0 is falsy, non-zero is truthy
- Strings: empty string is falsy, non-empty is truthy
- Slices: nil/empty slices are falsy, non-empty are truthy
- Maps: nil maps are falsy, non-nil maps are truthy (even if empty)
- Pointers: nil pointers are falsy, non-nil are truthy
- Channels: nil channels are falsy, created channels are truthy

### **5. Lambda Expressions**

**Lambda Syntax** (from `test_lambda.goo`):
- Basic lambdas: `x => x * 2`
- Complex expressions: `x => (x + 1) * 2 - 1`
- Lambda arguments to functions: `apply(x => x + 1, 5)`
- Variable assignment: `double := x => x * 2`

### **6. Class and Object Features**

**Class Definition** (from `test_class.goo`):
- `class` keyword instead of `struct`
- Object literal syntax: `{name: "Alice", age: 30}`
- Dot notation for field access

### **7. Function Definition Features**

**Function Definition** (from `test_def.goo`):
- `def` keyword as alternative to `func`
- Auto-return: `def meaning() int {42}` (no explicit return needed)

### **8. Enums**

**Enum Support** (from `test_enum.goo`):
- `enum Status { OK, BAD }` syntax
- Automatic constant generation

### **9. Additional Syntax Features**

**Map Dot Notation** (from `test_map_dot_notation.goo`):
- Access map values with dot notation: `user.name` instead of `user["name"]`
- Works with `map[string]T` types

**Simplified Output** (from `test_put.goo`):
- `put()` function for simple output (like Python's print)

**Check Assertions** (from `test_assert.goo`):
- `check` statements for assertions

**String Interpolation** (from `test_interpolation.goo`):
- Space-separated interpolation: `"result:" (2 + 3) "total"`

### **10. Import and Module Features**

- Auto-import functionality for common packages
- Bare import syntax support
- Helper modules support

## **Implementation Priority Recommendations**

### **Phase 1: Core Missing Features (High Impact)**
1. **Range function** - Foundational for many Python patterns
2. **List slicing with step** - `list[::-1]` for reverse, `list[::2]` for every nth
3. **Multiple assignment** - `a, b = b, a` tuple unpacking

### **Phase 2: Enhanced List Operations (Medium Impact)**  
4. **Missing list methods** - `.pop()`, `.insert()`, `.remove()`, `.extend()`
5. **Enumerate function** - `for i, item in enumerate(list)`

### **Phase 3: Advanced Features (Nice to Have)**
6. **List comprehensions** - `[x*2 for x in nums if x > 5]`
7. **String formatting** - f-string style or format method
8. **Zip function** - `for a, b in zip(list1, list2)`

This roadmap focuses on high-value features that maintain Go's performance while adding Python's ergonomics.