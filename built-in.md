# How to Register Built-in Functions in Go Compiler

This guide explains how to add user-accessible built-in functions to the Go compiler, based on the successful implementation of `typeMatches`.

## Overview

There are two types of functions in the Go compiler:
- **Internal runtime functions**: Only accessible to compiler internals (like `truthyAndOp`)
- **User-accessible builtin functions**: Available in user code (like `len`, `cap`, `make`, `typeMatches`)

This guide covers implementing user-accessible builtin functions.

## Implementation Steps

### 1. Runtime Function Declaration
**File**: `/opt/other/go/src/cmd/compile/internal/typecheck/_builtin/runtime.go`

Add the function declaration:
```go
func typeMatches(interface{}, string) bool
```

**Important**: After modifying this file, run `go generate` in the typecheck directory:
```bash
cd /opt/other/go/src/cmd/compile/internal/typecheck
go generate
```
This updates `builtin.go` with the new function declaration.

### 2. Runtime Implementation
**File**: `/opt/other/go/src/runtime/truthy.go` (or appropriate runtime file)

Implement the actual function:
```go
func typeMatches(value interface{}, typeName string) bool {
    return isTypeOf(value, typeName)
}
```

### 3. Types2 Builtin Declaration
**File**: `/opt/other/go/src/cmd/compile/internal/types2/universe.go`

Add builtin constant:
```go
const (
    // ... existing constants ...
    _TypeMatches // typeMatches(value, typeName) (runtime type checking)
    // ... more constants ...
)
```

Add to `predeclaredFuncs` array:
```go
var predeclaredFuncs = [...]struct{ name string; nargs int; variadic bool; kind exprKind }{
    // ... existing functions ...
    _TypeMatches: {"typeMatches", 2, false, expression},
    // ... more functions ...
}
```

### 4. Types2 Type Checking
**File**: `/opt/other/go/src/cmd/compile/internal/types2/builtins.go`

Add case in the main builtin function switch:
```go
case _TypeMatches:
    // typeMatches(value, typeName) bool
    if !check.assignment(x, Typ[String], "argument to typeMatches") {
        return
    }
    if len(args) != 2 {
        check.errorf(x, WrongArgCount, invalidArg+"typeMatches expects exactly 2 arguments")
        return
    }
    // Type checking logic here
    x.mode = value
    x.typ = Typ[Bool]
```

### 5. IR Operation Declaration
**File**: `/opt/other/go/src/cmd/compile/internal/ir/node.go`

Add the IR operation:
```go
const (
    // ... existing operations ...
    OTYPEMATCHES // typeMatches(value, typeName) (runtime type checking)
    // ... more operations ...
)
```

### 6. IR Handling
**File**: `/opt/other/go/src/cmd/compile/internal/typecheck/universe.go`

Register the builtin in IR:
```go
var builtinFuncs = [...]struct {
    name string
    op   ir.Op
}{
    // ... existing functions ...
    {"typeMatches", ir.OTYPEMATCHES},
    // ... more functions ...
}
```

### 7. SetOp Method Support
**File**: `/opt/other/go/src/cmd/compile/internal/ir/expr.go`

Add to appropriate SetOp method:
```go
func (n *BinaryExpr) SetOp(op Op) {
    switch op {
    // ... existing cases ...
    case OTYPEMATCHES:
        n.op = op
    // ... more cases ...
    default:
        panic("invalid SetOp: " + op.String())
    }
}
```

### 8. Code Generation
**File**: `/opt/other/go/src/cmd/compile/internal/walk/builtin.go`

Add case for code generation:
```go
case ir.OTYPEMATCHES:
    return walkTypeMatches(n, init)
```

Implement the walk function:
```go
func walkTypeMatches(n *ir.CallExpr, init *ir.Nodes) ir.Node {
    // Implementation for generating runtime call
}
```

## Key Points

1. **Order matters**: Follow the exact sequence above
2. **Generate builtin.go**: Always run `go generate` after modifying runtime.go
3. **SetOp is critical**: Missing SetOp cases cause "cannot SetOp" errors
4. **Test thoroughly**: Add tests in `/opt/other/go/probes/` first
5. **Rebuild compiler**: Use `/opt/other/go/src/build-compiler.sh` after changes

## Architecture Flow

```
User Code → Syntax → Types2 → IR → Walk → Runtime
           ↓       ↓       ↓    ↓     ↓
         Parser  TypeChk  IRGen Code  Actual
                                Gen   Impl
```

## Example: typeMatches Implementation

The `typeMatches` function was implemented following this exact pattern:

1. ✅ Added runtime declaration in `_builtin/runtime.go`
2. ✅ Added runtime implementation in `runtime/truthy.go`  
3. ✅ Added types2 constant `_TypeMatches` in `universe.go`
4. ✅ Added types2 type checking in `builtins.go`
5. ✅ Added IR operation `OTYPEMATCHES` in `node.go`
6. ✅ Added IR registration in `typecheck/universe.go`
7. ✅ Added SetOp support in `expr.go` (this was the missing piece!)

Result: `typeMatches(1, "int")` returns `true`, enabling the `is` operator to work properly.

## Debugging Tips

- **"undefined: function"** → Missing runtime declaration or types2 registration
- **"cannot SetOp OPERATION"** → Missing SetOp case in expr.go
- **Type errors** → Check types2 type checking logic
- **Runtime errors** → Check actual runtime implementation

## Files to Modify (Summary)

1. `/opt/other/go/src/cmd/compile/internal/typecheck/_builtin/runtime.go` - Declaration
2. `/opt/other/go/src/runtime/*.go` - Implementation  
3. `/opt/other/go/src/cmd/compile/internal/types2/universe.go` - Types2 constant
4. `/opt/other/go/src/cmd/compile/internal/types2/builtins.go` - Type checking
5. `/opt/other/go/src/cmd/compile/internal/ir/node.go` - IR operation
6. `/opt/other/go/src/cmd/compile/internal/typecheck/universe.go` - IR registration
7. `/opt/other/go/src/cmd/compile/internal/ir/expr.go` - SetOp support
8. `/opt/other/go/src/cmd/compile/internal/walk/builtin.go` - Code generation