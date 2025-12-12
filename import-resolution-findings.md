# Transform-Added Import Resolution: Investigation Findings

## Problem Statement
Transformers that add imports (e.g., string methods adding `import "strings"`) cause "could not import X (file not found)" errors.

## Root Cause Analysis

### Architecture
1. **Go Build Process**: 
   - Parse source → Find imports → Build dependencies → Create importcfg → Invoke compiler
2. **Goo Transform Process**:
   - Compiler receives source → Parse → **Apply Transforms** (add imports) → Type-check

**The Issue**: Transforms add imports INSIDE the compiler, but dependency resolution happens BEFORE the compiler runs. The go command never builds these transform-added packages.


## What Works: ImportManager
Successfully implemented centralized import management:
- `ImportManager` collects import requests from all transformers
- `RequestStringsImport()`, `RequestFmtImport()`, etc. provide clean API
- Imports are added to AST correctly
- Code is cleaner with no duplication

## Solutions

### Option 1: Wrapper Script (Recommended for now)
Pre-build common transform dependencies before compilation:
```bash
#!/bin/bash
# Pre-build packages that transforms commonly add
go build -o /dev/null strings fmt slices strconv 2>/dev/null
# Now run actual compilation
go "$@"
```

### Option 2: Two-Pass Compilation
1. First pass: Run transforms, collect needed imports
2. Communicate imports back to go command
3. Second pass: Full compilation with all dependencies

Requires: Modified go command or build tool integration.

### Option 3: Move Transforms Before Go Command
Make transforms a separate preprocessing step that runs before `go build`.

Requires: Major architectural change.

## Current Status
- ✅ ImportManager successfully adds imports to AST
- ✅ Transforms work correctly  
- ❌ Import resolution fails for transform-added packages
- ❌ go list recursion causes toolchain version conflicts

## Recommendation
For production use, implement wrapper script (Option 1) that pre-builds common packages.
For long-term solution, investigate Option 2 or 3.
