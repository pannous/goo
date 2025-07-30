# Auto-Import Scope Resolution Investigation

## Problem
Auto-import system is broken. Manual imports work fine, but automatic injection fails with either:
- `could not import strings/slices (open : no such file or directory)` 
- `undefined: strings/slices`

## What Works
✅ Manual imports: `import "strings"` + `strings.ToUpper()` works perfectly
✅ Transform detection: String/list methods are detected and transformed correctly  
✅ Package compilation: Standard library packages exist and compile fine

## Root Cause Analysis

### Current Status
- **Types2 auto-inject**: Added `injectStringsImportIfNeeded()` and `injectSlicesImportIfNeeded()`
- **Transform detection**: Fixed to look for pre-transform method calls (`"hello".toUpper()` not `strings.ToUpper()`)
- **Execution order**: Auto-import runs in types2 phase, transforms run later ✅

### Failed Attempts

#### Attempt 1: Fix execution order (COMPLETED)
**Problem**: Auto-import looked for post-transform calls but ran before transforms
**Fix**: Modified detection to look for original method calls like `list.contains()`, `"hello".toUpper()`
**Result**: Detection works, but import injection still fails

#### Attempt 2: Disable conflicting transform-level imports (COMPLETED)  
**Problem**: Transform-level imports conflicted with types2-level imports
**Fix**: Commented out transform-level import injection
**Result**: Error changed from "could not import" to "undefined: strings"

### Current Investigation Focus - BREAKTHROUGH!

**ROOT CAUSE DISCOVERED**: The transforms run **BEFORE** the types2 resolver, not after!

#### Execution Order (ACTUAL):
1. **Transform stage** - `"hello".toUpper()` → `strings.ToUpper("hello")` 
2. **Types2/Resolver stage** - `injectStringsImportIfNeeded()` looks for post-transform calls

#### Evidence:
```
DEBUG: Found method call: ToUpper    # Already transformed!
DEBUG: Found method call: Println    # From printf transform
```

The detection logic should look for **post-transform** calls like `strings.ToUpper()`, not pre-transform calls!

#### Attempt 3: Fix detection logic for post-transform calls (COMPLETED)
**Problem**: Detection looked for pre-transform calls but transforms run before types2  
**Fix**: Changed detection to look for `strings.ToUpper()` instead of `"hello".toUpper()`
**Result**: Detection now works, import injection called, but compilation still fails

#### Current Status:  
- ✅ Detection works: `DEBUG: Found strings method call: ToUpper`
- ✅ Injection called: `DEBUG: Injecting strings import` 
- ✅ Package resolution works: `DEBUG: Strings import successful`
- ❌ Final compilation fails: `could not import strings (open : no such file or directory)`

**HYPOTHESIS**: The programmatic import injection works at symbol table level but doesn't persist to the actual import processing stage where files are read from disk.

#### Attempt 4: Add imports to file.DeclList (PARTIAL SUCCESS)
**Problem**: Programmatic imports worked at symbol table level but not for actual compilation  
**Fix**: Added import declarations to `file.DeclList` so later compilation stages can find them
**Result**: 
- ✅ DeclList addition works: `DEBUG: Added strings import to file.DeclList at position 1`
- ❌ Still "could not import strings" 
- ❌ New "strings redeclared in this block" error (duplicate import)

**KEY INSIGHT**: The DeclList addition works but there's still a fundamental GOROOT/path issue preventing the actual package files from being found during compilation.

## Final Challenge: Import Configuration Generation

**BREAKTHROUGH with `-x` flag!** 

**Root Cause Found**: Auto-injected imports aren't included in the `importcfg` file that tells the compiler where to find packages.

**Evidence:**
- **Manual import**: `packagefile strings=/tmp/go-cache/...` ✅
- **Auto import**: Missing from importcfg entirely ❌

The import configuration file generation happens **before** our auto-import injection in the types2 phase. The build system doesn't know about our programmatic imports when it creates the importcfg.

**Latest Investigation - Transform-Level vs Build-Level Auto-Import**

**Finding 1**: Transform-level auto-import works at AST level but fails at importcfg level
- String and slice transforms successfully add imports to AST 
- But importcfg is generated before transforms run
- Result: `could not import slices (open : no such file or directory)`

**Finding 2**: Manual imports work perfectly
- Manual `import "slices"` → `packagefile slices=/tmp/go-cache/d6/...` in importcfg ✅
- Auto imports missing from importcfg entirely ❌

**Current Status**: Both pkg.go and exec.go auto-import detection not being called for single-file execution. Single-file execution may bypass normal package loading paths.

**Architecture Issue**: Need to either:
1. Ensure transforms run before importcfg generation (complex dependency resolution change)  
2. Hook importcfg generation to include transform-detected auto-imports
3. Find the actual single-file execution path and add auto-import logic there

**Progress**: Successfully identified the exact problem and have a working manual test case to compare against.

## Debugging Commands Used

```bash
# Test manual vs auto import
./bin/go run /tmp/test_manual_strings.goo  # Works
./bin/go run /opt/other/go/test/goo/test_strings_auto_import.goo  # Fails

# Check standard library exists  
ls -la /opt/other/go/src/strings/  # Exists
ls -la /opt/other/go/src/slices/   # Exists

# Rebuild toolchain
GOROOT=/opt/other/go ./make.bash  # Standard library compiled successfully
```

## Key Files Modified
- `/opt/other/go/src/cmd/compile/internal/types2/resolver.go` - Added auto-import functions
- `/opt/other/go/src/cmd/compile/internal/transforms/string_methods_transform.go` - Disabled transform-level imports