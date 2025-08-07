# Transformer Architecture Guide

## Overview

The Go compiler transformer system uses a centralized visitor pattern to efficiently transform AST nodes. This guide explains the new NodeTransformer architecture that eliminates code duplication and improves performance.

## Architecture Principles

### 1. **DRY Principle (Don't Repeat Yourself)**
- The central visitor does ALL AST traversal
- Transformers only handle their specific node types
- No duplication of tree walking logic

### 2. **Single Responsibility Principle**
- Central visitor: Traverses the AST once
- Transformers: Handle only their specific patterns
- PostProcess: Manages file-level changes (imports, etc.)

### 3. **Performance First**
- **Before**: N transformers × Full AST walk = N traversals
- **After**: 1 AST walk + targeted pattern matching = 1 traversal

## NodeTransformer Interface

```go
type NodeTransformer interface {
    // CanHandle returns true if this transformer can handle the given node
    CanHandle(node syntax.Node, ctx *TransformContext) bool
    
    // TransformNode transforms the given node and returns the modified node or nil if no change
    TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node
    
    // PostProcess is called after all transformations to handle file-level changes like imports
    PostProcess(file *syntax.File, ctx *TransformContext) bool
    
    // Standard methods
    Name() string
    Priority() int
}
```

## Implementation Patterns

### ✅ **GOOD - Simple CanHandle Pattern**

```go
func (t *IsOperatorTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
    // Only handle IS operations directly - central visitor finds them
    if op, ok := node.(*syntax.Operation); ok {
        return op.Op == syntax.IS
    }
    return false
}

func (t *IsOperatorTransform) TransformNode(node syntax.Node, ctx *TransformContext) syntax.Node {
    if op, ok := node.(*syntax.Operation); ok && op.Op == syntax.IS {
        return t.convertIsToFunctionCall(op)
    }
    return nil
}
```

### ❌ **BAD - Complex Traversal Pattern (DON'T DO THIS)**

```go
func (t *BadTransform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
    // BAD: Don't duplicate traversal logic!
    switch n := node.(type) {
    case *syntax.ExprStmt:
        if op, ok := n.X.(*syntax.Operation); ok && op.Op == syntax.IS {
            return true
        }
    case *syntax.AssignStmt:
        if op, ok := n.Rhs.(*syntax.Operation); ok && op.Op == syntax.IS {
            return true
        }
    // ... 50+ lines of duplicated traversal logic
    }
    return false
}
```

## Central Visitor Logic

The `CentralTransformVisitor` handles all tree traversal and calls transformers on relevant nodes:

```go
func (v *CentralTransformVisitor) tryTransformLeafNode(node syntax.Node) syntax.Node {
    // Only call transformers on "leaf" nodes they actually care about
    switch node.(type) {
    case *syntax.Operation, *syntax.CallExpr, *syntax.CompositeLit, *syntax.FuncDecl:
        return v.tryTransformNode(node)
    }
    return nil
}
```

## Common Node Types for Transformers

### Operations (`*syntax.Operation`)
- String concatenation: `op.Op == syntax.Add`
- Null coalescing: `op.Op == syntax.NullCoalesce`
- Type checking: `op.Op == syntax.IS`
- Boolean negation: `op.Op == syntax.Not`

### Function Calls (`*syntax.CallExpr`)
- Printf transformation: `name.Value == "printf"`
- Built-in function calls
- Method calls

### Composite Literals (`*syntax.CompositeLit`)
- Empty lists: `n.Type == nil && len(n.ElemList) == 0`
- Array/slice literals

### Function Declarations (`*syntax.FuncDecl`)
- Auto-return insertion
- Function signature modifications

## PostProcess Pattern

Use PostProcess for file-level changes that need to happen after all node transformations:

```go
func (t *StringConcatTransform) PostProcess(file *syntax.File, ctx *TransformContext) bool {
    // Add fmt import if we used fmt.Sprintf transformations
    if t.needsFmtImport && !t.hasImport(file, "fmt") {
        t.addFmtImport(file)
        t.needsFmtImport = false
        return true
    }
    return false
}
```

## Migration Guide

### From Old Transformer to NodeTransformer

1. **Simplify CanHandle**: Only check the specific node type you care about
2. **Simplify TransformNode**: Handle only your specific transformation
3. **Add PostProcess**: Move import management here
4. **Remove traversal logic**: Let the central visitor handle it

### Example Migration

**Before:**
```go
// 80+ lines of visitor pattern with traversal logic
func (v *visitor) Visit(node syntax.Node) syntax.Visitor {
    switch n := node.(type) {
    case *syntax.ExprStmt:
        if op, ok := n.X.(*syntax.Operation); ok && op.Op == syntax.IS {
            n.X = t.transform(op)
        }
    // ... many more cases
    }
}
```

**After:**
```go
// 5 lines focused on the actual transformation
func (t *Transform) CanHandle(node syntax.Node, ctx *TransformContext) bool {
    if op, ok := node.(*syntax.Operation); ok {
        return op.Op == syntax.IS
    }
    return false
}
```

## Performance Benefits

- **Single AST traversal** instead of N separate traversals
- **O(M)** complexity instead of **O(N×M)** where N=transformers, M=nodes
- **Focused pattern matching** - transformers only called on relevant nodes
- **Reduced memory allocation** from fewer tree walks

## Best Practices

1. **Keep CanHandle simple** - 2-3 lines maximum
2. **Handle only your specific node type** - don't traverse
3. **Use PostProcess for imports** - don't modify files in TransformNode
4. **Preserve position information** - always call `SetPos()` on new nodes
5. **Test both interfaces** - NodeTransformer and legacy fallback

## Examples in Codebase

- `string_concat_transform.go` - Handles `syntax.Add` operations
- `is_operator_transform.go` - Handles `syntax.IS` operations  
- `null_coalesce_transform.go` - Handles `syntax.NullCoalesce` operations
- `falsey_transform.go` - Handles `syntax.Not` operations
- `empty_list_transform.go` - Handles empty `syntax.CompositeLit`
- `auto_return_transform.go` - Handles `syntax.FuncDecl`

This architecture eliminates code duplication while maintaining high performance and clean separation of concerns.