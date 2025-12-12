# Detailed Regression Analysis

## Summary
After merge from upstream Go: 75/118 tests passing (down from 114/122 pre-merge)
**Current status**: 77/118 after fixing transformer invocation

## Root Causes Identified

### 1. Transformer Invocation Issue ✅ FIXED
**Problem**: Transforms weren't running on .goo files
**Cause**: `isToolchainPath()` filtered out "command-line-arguments" package, which is what `go run file.goo` uses
**Fix**: Modified `isToolchainPath()` to never skip .goo files regardless of package name
**Impact**: +2 tests (75→77)
**File**: `src/cmd/compile/internal/transforms/transform.go`

### 2. Import Resolution for Transform-Added Imports ❌ NOT FIXED
**Problem**: When transforms add imports (e.g., strings, fmt), those packages can't be found
**Cause**: Multi-part issue:
  1. Transforms correctly add ImportDecl to AST
  2. `updateImportConfigForTransforms()` tries to resolve them via `resolveStandardLibraryPackage()`
  3. `resolveStandardLibraryPackage()` looks for `$GOROOT/pkg/$GOOS_$GOARCH/strings.a` which doesn't exist in modern Go
  4. `openPackage()` tried to open empty string when PackageFile[path] didn't exist
  5. Even with fallback to ImportDirs, packages aren't built because they weren't in original source

**Partial Fix Applied**: Modified `openPackage()` in import.go to check if path exists in PackageFile before opening
**Why It Doesn't Fully Work**: The real issue is that Go's build system doesn't know to compile packages that were only added by transforms
**Impact**: 0 additional tests fixed
**Files**:
  - `src/cmd/compile/internal/noder/import.go` (partial fix)
  - `src/cmd/compile/internal/noder/unified.go` (investigated but can't fix here)

**Real Solution**: ImportManager (exists on branch with commit 09efd3f8a0)
  - Centralized import coordination
  - Triggers package building for transform-added dependencies
  - Was developed but never merged to main branch
  - Would need to be ported to current codebase

### 3. Truthy Transform Scope ❌ NOT AN INTEGRATION ISSUE
**Problem**: test_truthy.goo and test_truthy_and.goo failing
**Investigation**: truthy_and_transform only handles `x and y` operations, not plain `if non_boolean {}` statements
**Attempted Fix**: Added if-condition wrapping in truthy() calls
**Result**: Created invalid AST (incorrect BlockStmt handling)
**Decision**: Reverted - this is a transformer feature gap, not integration issue
**Note**: User guidance was "transformer should usually not be changed because it worked before"

## Test Failure Categories

### Import-Related Failures (majority of 41 remaining failures)
These likely need ImportManager solution:
- test_strings_auto_import.goo
- test_string_methods.goo
- test_string_methods_todo.goo
- test_manual_strings.goo
- test_in_operator_strings.goo
- test_list_methods.goo
- And many others that use transform-added imports

### Other Failures
- test_typeof.goo - returns "interface {}" instead of "untyped int"
- test_truthy.goo - needs truthy() wrapping for if conditions (transformer feature)
- test_truthy_and.goo - same as above
- Various iterator/list/string tests - likely import-related

## What Changed in Merge

### Files Modified in src/cmd/compile/internal/noder/
1. **unified.go**:
   - `updateImportConfigForTransforms` was gutted to "do nothing for now"
   - Restored to pre-merge state but still doesn't work (see Import Resolution above)

2. **reader.go**: Cosmetic renames (defi => def)

3. **writer.go**: interface{} => any, new(expr) support

4. **doc.go**: Comment fixes

### What Didn't Change
- Transformer implementations (truthy_and_transform, string_methods_transform, etc.)
- Transform application order
- Most import resolution code

## Recommendations

### Short Term
1. **Import other branch's ImportManager** (commit 09efd3f8a0)
   - This is the proper solution for transform-added imports
   - Needs careful merge as it's on a diverged branch
   - Would likely fix 20-30 of the failing tests

2. **Document known limitations**
   - Update CLAUDE.md with import resolution constraints
   - Note which tests require ImportManager

### Medium Term
1. **Investigate typeof issue separately**
   - Why does typeof(42) return "interface{}" now?
   - Check if type inference changed in merge

2. **Review truthy transform scope**
   - Should it handle plain if-conditions or just `and` operations?
   - If needed, implement properly (not attempted quick fix)

### Long Term
1. **Upstream sync strategy**
   - Document custom changes that conflict with upstream
   - Create integration test suite to catch regressions
   - Consider upstreaming some Goo features

## Key Learnings

1. **Integration vs Transformer Issues**: Most failures are integration (how transforms interact with compiler), not transformer bugs
2. **Import Timing**: Imports added by transforms need special handling - can't rely on normal package resolution
3. **Build System Awareness**: Go's build system needs to know about transform-added dependencies before type checking
4. **Branch Divergence**: ImportManager solution exists but wasn't on main branch - need better branch management

## Files to Review for ImportManager Port

From commit 09efd3f8a0:
```
feat(arch): implement centralized import resolution system

New Centralized Import Manager
- Created ImportManager class that collects all import requests from transformers
- Global coordinator that deduplicates and applies imports in single step
- Eliminates scattered import handling across individual transformers
```

This would be the proper fix for the majority of remaining test failures.
