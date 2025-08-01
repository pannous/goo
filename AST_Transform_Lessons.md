# AST Transformation Lessons: Visitor Pattern vs Custom Walker

## The Problem: Interface Conversion Panic During AST Modification

When implementing AST transformations in Go compilers, a common error pattern emerges:

```
panic: interface conversion: types.Object is nil, not *ir.Name
```

This document explains why this happens and provides safe patterns for AST transformation.

## Root Cause Analysis

### The Failing Pattern: In-Place Modification During Visitor Traversal

```go
// ❌ DANGEROUS: Modifying AST during syntax.Walk traversal
func (v *visitor) Visit(node syntax.Node) syntax.Visitor {
    switch n := node.(type) {
    case *syntax.VarDecl:
        if n.Values != nil {
            if transformed := v.transform.transformExpr(n.Values, v); transformed != n.Values {
                n.Values = transformed  // ❌ MODIFYING DURING TRAVERSAL
                v.changed = true
            }
        }
    }
    return v  // ❌ CONTINUES TRAVERSAL WITH MODIFIED AST
}
```

### Why This Causes Compiler Panics

1. **AST Traversal State Corruption**
   - `syntax.Walk` maintains internal state about node relationships
   - Modifying nodes during traversal corrupts this state
   - Child node relationships become inconsistent

2. **Type Information Desynchronization**
   - The type checker (`types2` package) builds symbol tables during AST processing
   - When nodes are replaced mid-traversal, type information becomes stale
   - Original node has type info, replacement node has none

3. **IR Generation Phase Failure**
   - Later compilation phases expect typed AST nodes
   - When converting AST to IR (Intermediate Representation), missing type info causes:
     ```
     panic: interface conversion: types.Object is nil, not *ir.Name
     ```
   - The compiler expects a typed object but finds `nil`

### Execution Flow Leading to Panic

```
1. syntax.Walk starts traversing AST
2. Visit() method called on node A
3. Visit() replaces node A with node B during traversal
4. syntax.Walk continues with corrupted state
5. Type checker processes mixed old/new node references
6. Type information becomes inconsistent
7. IR generation phase encounters node B with no type info
8. PANIC: Expected *ir.Name, got nil
```

## The Safe Pattern: Custom Walker with Controlled Traversal

```go
// ✅ SAFE: Custom walker with controlled traversal order
func (t *Transform) Transform(file *syntax.File, ctx *TransformContext) bool {
    visitor := &visitor{transform: t, ctx: ctx, file: file}
    
    // Walk declarations explicitly
    for _, decl := range file.DeclList {
        t.walkAndTransform(decl, visitor)
    }
    
    return visitor.changed
}

func (t *Transform) walkAndTransform(node syntax.Node, visitor *visitor) {
    switch n := node.(type) {
    case *syntax.FuncDecl:
        if n.Body != nil {
            t.transformStmtList(n.Body.List, visitor)
        }
    case *syntax.VarDecl:
        if n.Values != nil {
            n.Values = t.transformExpr(n.Values, visitor)  // ✅ SAFE REPLACEMENT
        }
    }
}

func (t *Transform) transformStmt(stmt syntax.Stmt, visitor *visitor) {
    switch s := stmt.(type) {
    case *syntax.AssignStmt:
        if s.Rhs != nil {
            s.Rhs = t.transformExpr(s.Rhs, visitor)  // ✅ SAFE REPLACEMENT
        }
    }
}
```

### Why Custom Walker Works

1. **Controlled Traversal Order**
   - Explicitly controls when and how to visit child nodes
   - No reliance on external traversal state

2. **Post-Order Transformation**
   - Transforms child expressions first
   - Replaces parent references only after children are processed
   - Maintains consistent AST structure

3. **No State Corruption**
   - Doesn't interfere with compiler's internal traversal mechanisms
   - Each transformation is atomic and complete

## Alternative Safe Patterns

### Pattern 1: Two-Pass Transformation

```go
// Pass 1: Collect nodes to transform
type transformTask struct {
    parent interface{}
    field  string
    oldNode syntax.Expr
    newNode syntax.Expr
}

func (v *visitor) Visit(node syntax.Node) syntax.Visitor {
    // Only collect, don't modify
    if needsTransform(node) {
        v.tasks = append(v.tasks, createTask(node))
    }
    return v
}

// Pass 2: Apply transformations
func applyTransformations(tasks []transformTask) {
    for _, task := range tasks {
        // Safe to modify now that traversal is complete
        setField(task.parent, task.field, task.newNode)
    }
}
```

### Pattern 2: AST Reconstruction

```go
// Build new AST instead of modifying existing one
func (t *Transform) transformExpr(expr syntax.Expr) syntax.Expr {
    switch e := expr.(type) {
    case *syntax.Operation:
        if e.Op == syntax.In {
            return t.createNewCallExpr(e)  // Returns new node
        }
        // Recursively transform children
        return &syntax.Operation{
            Op: e.Op,
            X:  t.transformExpr(e.X),
            Y:  t.transformExpr(e.Y),
        }
    }
    return expr
}
```

## Best Practices for AST Transformation

### ✅ DO

1. **Use Custom Walkers for Complex Transformations**
   - Provides full control over traversal order
   - Prevents state corruption
   - Easier to debug and maintain

2. **Transform in Post-Order**
   - Process children before parents
   - Ensures consistent AST structure
   - Avoids orphaned references

3. **Set Position Information**
   ```go
   newNode := &syntax.CallExpr{...}
   newNode.SetPos(oldNode.Pos())  // Preserve source positions
   ```

4. **Validate Transformations**
   - Ensure transformed AST is well-formed
   - Check that all references are valid
   - Add debug logging for complex transformations

### ❌ DON'T

1. **Modify AST During syntax.Walk Traversal**
   - Leads to state corruption and panics
   - Unpredictable behavior in complex cases

2. **Ignore Position Information**
   - Breaks error reporting and debugging
   - Source maps become inaccurate

3. **Create Circular References**
   - Can cause infinite loops in later phases
   - Always ensure AST remains a DAG (Directed Acyclic Graph)

## Debugging AST Transformation Issues

### Common Error Patterns

1. **`panic: interface conversion: types.Object is nil, not *ir.Name`**
   - Cause: Node replacement during traversal
   - Solution: Use custom walker or two-pass approach

2. **`<unknown line number>: internal compiler error`**
   - Cause: Missing position information
   - Solution: Always call `SetPos()` on new nodes

3. **Infinite compilation loops**
   - Cause: Circular AST references
   - Solution: Validate AST structure after transformation

### Debugging Tools

```go
// Add debug logging
func (t *Transform) transformExpr(expr syntax.Expr) syntax.Expr {
    fmt.Printf("Transforming: %s at %v\n", syntax.String(expr), expr.Pos())
    result := t.doTransform(expr)
    fmt.Printf("Result: %s\n", syntax.String(result))
    return result
}

// Validate AST structure
func validateAST(node syntax.Node) {
    syntax.Walk(node, &validationVisitor{})
}
```

## Historical Context: The In Operator Transform Bug

This lesson was learned while debugging the in operator transform in the Goo compiler. The issue was introduced in commit `da064afeef` when switching from a working custom walker to a problematic visitor pattern.

### Timeline of the Bug

1. **Working State**: Custom walker correctly transformed `"a" in "b"` to `strings.Contains("b", "a")`
2. **Bug Introduction**: Switched to visitor pattern with in-place modification
3. **Symptoms**: Internal compiler errors, type conversion panics
4. **Root Cause**: AST modification during `syntax.Walk` traversal
5. **Resolution**: Reverted to custom walker approach

### Key Files Involved

- `src/cmd/compile/internal/transforms/in_operator_transform.go`: The transform implementation
- `goo/test_in_operator_auto_import.goo`: Test case that exposed the bug
- `goo/test_in_operator_strings.goo`: Working test case with manual imports

## Conclusion

AST transformation is a powerful but delicate operation. The Go compiler's multi-phase architecture (parsing → type checking → IR generation → optimization → code generation) requires careful coordination between phases.

**Key Takeaway**: Never modify AST nodes during visitor pattern traversal. Use custom walkers, two-pass approaches, or AST reconstruction instead.

The extra complexity of custom walkers is worth the reliability and debuggability they provide, especially for non-trivial transformations.

---

*Document created: 2025-08-01*  
*Context: Debugging Go compiler AST transformation panics*  
*Related: In operator transform implementation, visitor pattern issues*