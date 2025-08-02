# Gofmt Supported Syntactic Features

This document lists the Goo syntactic features tested with `gofmt` and their actual support status.

**Test Results: 10/28 features currently supported by gofmt**

## Comments and Shebang
✅ `#` hash comments  
✅ `#!/usr/bin/env goo` shebang support  
✅ Traditional `//` and `/* */` comments  

## Operators and Keywords
✅ `and` operator 
✅ `or` operator 
❌ `not` operator (parser error: expected ';')
❌ `ø` / `≠` / `¬` Unicode operators 
❌ `in` operator (parser error: unexpected token)
❌ `as` operator (parser error: unexpected token)

## Function Definition
❌ `def` keyword (parser error: expected 'func')
❌ Function modifier `!` (parser error: unexpected token)
✅ Auto-return for single expression functions (standard Go syntax)
❌ Void return support (requires transformation)

## Control Flow
❌ Truthy/falsey `if` statements (requires transformation)
❌ `check` keyword (parser error: expected statement)
❌ `try`/`catch` blocks (parser error: unexpected token)

## Data Structures
❌ Array literal syntax `[1,2,3]` with type inference (parser error)
❌ 1-indexed array access with `#` operator (parser error: unexpected '#')
❌ Map literal syntax `{a: 1, b: 2}` with symbol keys (parser error)
❌ Map dot notation access `z.a` (requires semantic analysis)
❌ Map type inference (requires transformation)

## String Operations
✅ String concatenation with mixed types (standard Go parsing)
✅ String methods `"abc".contains("a")` (standard Go method call)
❌ String interpolation (requires custom parsing)
❌ Unicode string/rune equality (requires transformation)

## Lambda and Higher-Order Functions
❌ Lambda syntax `x => x * 2` (parser error: unexpected '=>')
❌ List methods `.apply()`, `.filter()`, `.map()` (requires transformation)

## Type System
✅ `typeof(x)` function (parsed as regular function call)
❌ `class` via type struct (parser error: unexpected 'class')
❌ `enum` declarations (parser error: unexpected 'enum')

## Built-in Functions
✅ `printf()` function (parsed as regular function call)
❌ `put()` function (requires import transformation)

## Import System
❌ Local imports `import "helper.goo"` (requires custom handling)
❌ Auto-import for common packages (requires transformation)
❌ Unused imports as warnings only (compiler feature, not gofmt)

## Compilation Features
❌ No main function required (compiler feature, not gofmt)
❌ Unused variables as warnings only (compiler feature, not gofmt)
❌ Default `go run` behavior (go command feature, not gofmt)

## Advanced Features
❌ List comparison `[1,2] == [1,2]` (requires transformation)
❌ Mixed type operations with automatic conversion (requires transformation)  


## Analysis

The test results reveal that `gofmt` currently supports only **basic syntactic elements** that don't require:
1. **Custom tokens** (like `def`, `check`, `not`, `in`, `as`, `#`, `=>`)
2. **Semantic transformations** (type inference, auto-imports)
3. **Compiler-specific features** (warnings vs errors)

### What Works:
- **Comments**: Hash comments, shebang, traditional comments
- **Basic parsing**: Simple identifier usage (`and`, `or` as identifiers)
- **Standard Go syntax**: Auto-return, string concatenation, method calls
- **Function calls**: `printf()`, `typeof()` (parsed as regular calls)

### What Doesn't Work:
- **All custom Goo keywords and operators** (parser doesn't recognize them)
- **All syntactic sugar** (requires transformation pipeline)
- **All semantic features** (type inference, auto-imports, etc.)

## Conclusion

The current `gofmt` implementation can handle **basic .goo files** that use mostly standard Go syntax with hash comments. However, **most Goo-specific features cause parser errors** because the internal syntax parser expects standard Go tokens.

To fully support Goo syntax in `gofmt`, we would need to:
1. Extend the lexer to recognize Goo tokens (`def`, `check`, `not`, `in`, `as`, etc.)
2. Modify the parser to handle Goo syntax constructs
3. Add a formatting mode that preserves Goo syntax in output

## Current Recommendation

For now, `gofmt` should be used primarily on:
- Standard Go files (`.go`)
- Simple Goo files using basic Go syntax with hash comments
- Files where Goo features have already been transformed to Go

**Most Goo-specific syntax requires the full compilation pipeline to transform before formatting.**

## Usage
```bash
# Format a single .goo file
gofmt file.goo

# Format with diff output
gofmt -d file.goo

# Format in place
gofmt -w file.goo

# Format directory recursively
gofmt -w ./
```