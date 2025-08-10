# Feature Edge Case Testing Tracker

## Status Legend
- ✅ **Tested & Working** - Feature has comprehensive edge case tests
- ⚠️ **Needs Edge Cases** - Basic feature works, needs edge case testing  
- ❌ **Broken** - Feature fails even basic cases
- 🔧 **In Progress** - Currently being fixed
- ☐ **Not Implemented** - Feature not yet implemented

---

## Core Language Features

### ✅ Truthy/Falsey If
- **Basic Test**: `test_truthy.goo` - covers numbers, strings, slices, maps, pointers, channels
- **Edge Cases Needed**: None identified
- **Status**: Well tested

### ⚠️ Try-Catch Error Handling  
- **Basic Test**: `test_try_catch.goo` - simple panic/catch
- **Edge Cases Needed**:
  - Nested try-catch blocks
  - Try-catch with return values
  - Try-catch in different scopes (function, loop, conditional)
  - Multiple panic types (string, error, custom types)

### ⚠️ Try Assignment (Error Propagation)
- **Basic Test**: `test_try_assign.goo`, `test_try_propagation.goo`
- **Edge Cases Needed**:
  - Try assignment in nested expressions
  - Multiple try assignments in same statement
  - Try assignment with complex function calls
  - Try assignment with type conversions

### ✅ Comment and Shebang Support
- **Basic Test**: `test_shebang.goo`, `test_comments.goo`
- **Edge Cases Needed**: None identified
- **Status**: Well tested

### ⚠️ Hash Comments and Preprocessor
- **Basic Test**: Basic `#` comments work
- **Edge Cases Needed**:
  - `#if DEBUG` conditional compilation testing
  - Nested preprocessor directives
  - Edge cases with `#end` matching

### ✅ Operator Synonyms
- **Basic Test**: `test_and_or.goo`, `test_not.goo`
- **Edge Cases Needed**: None identified - covers `and`, `or`, `not` operators
- **Status**: Well tested

### ✅ No Main Function Required
- **Basic Test**: `test_implicit_main.goo`
- **Edge Cases Needed**: None identified
- **Status**: Well tested

### ✅ Printf Auto-Import
- **Basic Test**: `test_printf.goo`, `test_simple_printf.goo`
- **Edge Cases Needed**: None identified
- **Status**: Well tested

### ⚠️ Typeof Operator
- **Basic Test**: `test_typeof.goo`
- **Edge Cases Needed**:
  - Typeof with complex expressions
  - Typeof with custom types/structs
  - Typeof with nil values
  - Typeof with interface{} values

### ✅ Check Assertions
- **Basic Test**: `test_assert.goo`, `test_check_reverse.goo`
- **Edge Cases Needed**: None identified
- **Status**: Well tested

---

## Data Structures and Types

### ⚠️ Simple Lists/Arrays
- **Basic Test**: `test_list.goo`, `test_array_1indexed.goo`
- **Edge Cases Needed**:
  - Empty array edge cases: `[]` vs `[]int{}`
  - Mixed type arrays: `[1, "hello", true]`
  - Nested arrays: `[[1, 2], [3, 4]]`
  - Very large arrays (performance/memory)

### ✅ Hash Index Access (1-based)
- **Basic Test**: `test_hash_index.goo`
- **Edge Cases Tested**: `probes/edge_case_hash_index.goo`
  - ✅ Hash index with negative numbers: `arr#-1`, `arr#-2` 
  - ✅ Hash index with expressions: `arr#(i+1)`
  - ✅ Hash index with variables: `arr#variable`
  - ☐ Hash index bounds checking: `arr#999` (out of bounds) - not tested yet
- **Status**: Working with comprehensive edge case coverage
- **Fix Applied**: Added array_index_transform.go for negative indexing support

### ✅ Lambda Expressions
- **Basic Test**: `test_lambda.goo`, `test_lambda_arg.goo`
- **Edge Cases Needed**: None identified - covers complex cases
- **Status**: Well tested

### ✅ Type Checking (is operator)
- **Basic Test**: `test_is_operator.goo`
- **Edge Cases Needed**: None identified
- **Status**: Well tested

---

## Object-Oriented Features

### ⚠️ Def Function Synonym
- **Basic Test**: `test_def.goo`, `test_def_simple.goo`
- **Edge Cases Needed**:
  - Def with complex parameter types
  - Def with variadic parameters: `def test(args ...string)`
  - Def with generic type parameters
  - Def with receiver methods: `def (r Receiver) method()`

### ⚠️ Enum Types
- **Basic Test**: `test_enum.goo`, `test_enum_string.goo`
- **Edge Cases Needed**:
  - Enum with explicit values: `enum Status { OK = 1, BAD = 2 }`
  - Enum with string values: `enum Color { RED = "red" }`
  - Enum comparisons and conversions
  - Enum in switch statements

### ⚠️ Object Literals and Maps
- **Basic Test**: `test_map.goo`, `test_map_dot_notation.goo`
- **Edge Cases Needed**:
  - Nested object literals: `{user: {name: "Alice", age: 30}}`
  - Object literals with computed keys
  - Mixed key types in maps
  - Object literal type inference edge cases

### ⚠️ Map Dot Access
- **Basic Test**: `test_map_dot_notation.goo`, `test_map_dot_comprehensive.goo`
- **Edge Cases Needed**:
  - Dot access with non-string keys
  - Dot access with special characters in keys
  - Dot access assignment: `user.name = "Bob"`
  - Chained dot access: `user.address.street`

---

## String and Character Operations

### ⚠️ String Methods  
- **Basic Test**: `test_string_methods.goo`
- **Edge Cases Tested**: `probes/edge_case_string_methods.goo` (partial)
  - ✅ String methods with empty strings work: `"".size()` returns 0
  - ✅ String methods with Unicode work: `"你好".reverse()`, `"你好".size()`
  - ❌ **Method chaining broken**: `"hello".toUpper().reverse()` fails
    - Root cause: Go string methods return Go strings, not Goo-enhanced strings
    - Fix needed: String methods should return chainable objects
  - ☐ String methods with very long strings - not tested yet
- **Critical Issue Found**: Method chaining not supported

### ⚠️ Type Casting (as operator)  
- **Basic Test**: `test_as_cast.goo`, `test_as_cast_convert.goo`
- **Edge Cases Needed**:
  - Invalid casts: `"hello" as int` (should error gracefully)
  - Casting with nil values: `nil as string`
  - Casting complex types: `[]int{1,2} as []any`
  - Casting with interfaces

### ⚠️ String Concatenation
- **Basic Test**: `test_string_concat.goo`
- **Edge Cases Needed**:
  - String + number auto-conversion edge cases
  - String + bool: `"result: " + true`
  - String + nil handling
  - Very long string concatenations

### ⚠️ String-Character Equality
- **Basic Test**: `test_string_char_comparison.goo`
- **Edge Cases Needed**:
  - Unicode character comparisons: `"你" == '你'`
  - Multi-byte character edge cases
  - Empty string vs null character: `"" == '\0'`

---

## Control Flow

### ⚠️ For-In Loops (Universal Syntax)
- **Basic Test**: `test_for_in_key_value.goo` (just fixed!)
- **Edge Cases Needed**:
  - For-in with nil collections: `for x in nil_slice`
  - For-in with empty collections: `for x in []int{}`
  - For-in with interface{} slices
  - For-in nested loops: `for x in arr { for y in x }`

### ⚠️ While Loops
- **Basic Test**: `test_while_loops.goo`  
- **Edge Cases Needed**:
  - While with complex conditions
  - While true with nested breaks/continues
  - While with for-in syntax combinations

### ⚠️ Range Syntax (for i in 0…5)
- **Basic Test**: `test_ellipsis.goo` (just implemented)
- **Edge Cases Tested**: `probes/edge_case_range_loops.goo`, `probes/debug_range_context.goo`
  - ✅ Range expressions work: `nums := 0…3` creates `[0,1,2,3]`
  - ✅ Range with variables: `start…end` works in expressions
  - ❌ **Direct for-loop syntax broken**: `for i in 0…5` fails with parser error
    - Root cause: Parser can't handle ellipsis in for-in identifier context
    - Workaround: `nums := 0…5; for i in nums` works perfectly
  - ☐ Range with negative numbers: `for i in -5…5` - not tested yet
  - ☐ Large ranges: `for i in 1…100000` - not tested yet
- **Status**: Expressions work, direct for-loop syntax needs parser fix

---

## Advanced Features

### ⚠️ Auto Return
- **Basic Test**: `test_auto_return.goo`
- **Edge Cases Needed**:
  - Auto return with complex expressions
  - Auto return with conditional statements
  - Auto return type inference with generics
  - Auto return with multiple possible types

### ⚠️ Class Support
- **Basic Test**: `test_class.goo`, `test_class_methods.goo`
- **Edge Cases Needed**:
  - Class inheritance patterns
  - Class with interface implementations
  - Class method overriding
  - Class constructors and initialization

### ⚠️ Modify-in-Place Functions (!)
- **Basic Test**: `test_modify.goo`
- **Edge Cases Needed**:
  - Modify with different data types
  - Modify with nested structures
  - Modify with concurrent access patterns
  - Modify error handling

### ⚠️ Local Imports
- **Basic Test**: `test_import_folder.goo` (currently failing)
- **Edge Cases Needed**:
  - Circular import detection
  - Relative path imports
  - Import with different file extensions
  - Import resolution conflicts

### ⚠️ In Operator
- **Basic Test**: `test_in_operator_strings.goo`, `test_in_operator_slices.goo`
- **Edge Cases Needed**:
  - In operator with nil collections
  - In operator with interface{} types
  - In operator with custom types
  - In operator performance with large collections

---

## Not Yet Implemented Features (from README ☐)

### ☐ Range Loops (0…5)
- **Status**: Partially working (ellipsis expressions work, but README says for-loops don't)
- **Test Needed**: `for i in 0…5` syntax

### ☐ Auto Return Type Inference
- **Status**: Not implemented
- **Need**: `func test() { 42 }` → `func test() int { return 42 }`

### ☐ Check Debug Messages
- **Status**: Not implemented  
- **Need**: `check 1>0` should emit "check OK 1>0"

### ☐ Optional Chaining (?.)
- **Status**: Not implemented
- **Need**: `x?.y?.z` syntax

---

## Edge Case Testing Results (Today's Session)

### Issues Found and Fixed:
1. ✅ **Array Negative Indexing**: `arr#-1` was double-subtracting → Fixed with array_index_transform.go
2. ✅ **For-In Map Keys**: `for key in myMap` returned values instead of keys → Fixed in in_loop_transform.go  
3. ❌ **Method Chaining**: `"hello".toUpper().reverse()` fails - Go strings don't have Goo methods
4. ❌ **Direct Range For-Loops**: `for i in 0…5` fails with parser error - workaround available

### Test Count Progress:
- **Before**: 123/130 tests passing
- **After**: 133/143 tests passing  
- **Improvement**: +10 passing tests, +13 total tests

## Summary  
- **Well Tested**: 8 features (improved from 6)
- **Need Edge Cases**: 16 features (improved from 18)
- **Critical Issues Found**: 2 features need fixes (method chaining, direct range for-loops)
- **Not Implemented**: 4 features

## Next Steps
1. **Priority Fixes**:
   - Fix method chaining for string operations
   - Fix direct range for-loop syntax parsing
2. **Additional Edge Cases**: 
   - Try-catch nested scenarios
   - Map operations with complex types
   - Type casting invalid scenarios
3. **Continue systematic testing of remaining ⚠️ features**