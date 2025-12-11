# Goo Upstream Merge Strategy

## Current State (as of 2025-12-11)

### Merged Versions
- ✅ **go1.25.5** - Fully merged from `upstream/release-branch.go1.25`
  - Commit: 15ced7c6a0 (Merge remote-tracking branch 'upstream/release-branch.go1.25')
  - Date: 2024-10-18
  - All commits through d5bfdcbc47 integrated

### Local Version
- **go1.26.1** (custom build based on go1.25.5 + Goo enhancements)

### Upstream Branches Status
- `upstream/release-branch.go1.25` - ✅ Up to date (d5bfdcbc47)
- `upstream/master` - Development branch (~1605 commits ahead of go1.25.5)

## Architecture Changes for Clean Merges

### Goo-Specific Features Isolation

Successfully refactored to minimize merge conflicts:

#### 1. Transformer Architecture (NEW - Dec 2025)
All Goo-specific syntax extensions now handled via transformer pipeline:

**Location**: `src/cmd/compile/internal/transforms/` (37 transformers, 13,414 LoC)

**Key Transformers**:
- `check_transform.go` - Desugars `check` statements (NEW Dec 2025)
- `as_cast_transform.go` - Converts `as` casts to type assertions
- `lambda_transform.go` - Lambda expressions
- `string_methods_transform.go` - String method sugar (2,293 LoC)
- `list_methods_transform.go` - List operations (1,629 LoC)
- `in_operator_transform.go` - `in` operator (1,289 LoC)
- `try_catch_transform.go` - Exception handling
- 30+ more transformers

**Integration Point**: `src/cmd/compile/internal/noder/unified.go:204`
```go
transforms.ApplyTransformations(files)
```

#### 2. Scanner/Parser Extensions

**Location**: `src/cmd/compile/internal/syntax/`

**Key Files**:
- `scanner.go` - Conditional keyword recognition (transformsEnabled())
- `parser.go` - Parses Goo-specific syntax to AST nodes
- `tokens.go` - Custom token definitions (_As, _Check, _Lambda, etc.)
- `nodes.go` - AST node definitions (AsCastExpr, CheckStmt, etc.)

**Conditional Behavior**:
```go
if s.transformsEnabled() {
    switch litStr {
    case "as":
        s.tok = _As
        return
    case "check":
        s.tok = _Check
        return
    // ... other Goo keywords
    }
}
```

#### 3. Minimal Core Compiler Changes

**Modified Files** (vs upstream go1.25.5):
- `noder/` - 152 insertions, 55 deletions (transform integration)
- `ir/` - Adds OCHECK, AsCastExpr support (backward compat)
- `types2/` - Minimal changes for new node types
- Other files - Mostly toolchain integration

## Merge Conflict Reduction Strategy

### What We've Done
1. ✅ Moved `check` keyword handling from IR/noder/types2 → transformer (Dec 2025)
2. ✅ Kept `as` keyword in transformer (already was)
3. ✅ All Goo syntax → Isolated in transforms/ and syntax/
4. ✅ Core compiler sees only standard Go constructs after transformation

### What This Means for Future Merges
- **Low conflict areas**: Runtime, standard library, most of cmd/compile
- **Medium conflict areas**: noder/ (transform integration), syntax/ (our extensions)
- **High conflict areas**: transforms/ (but upstream doesn't touch this)

## Merge Recommendations

### ✅ Safe to Merge: go1.25.x Point Releases
**All point releases** (go1.25.6, go1.25.7, etc.)
- Bug fixes and security patches
- Low risk of conflicts
- **Status**: Currently up to date with go1.25.5
- **Action**: `git merge upstream/release-branch.go1.25` when new releases appear

### ⚠️ Risky: master (go1.26+/go1.27)
**Requires go1.26 runtime** - Not ready until official go1.26 release
- ~1605 commits ahead of go1.25.5
- Runtime changes, API additions
- **Action**: Wait for go1.26 official release, then merge release-branch.go1.26

### 🔍 Cherry-Pick Candidates from master
Some commits could be cherry-picked individually but have conflicts:

**Compiler Bug Fixes** (attempted - had conflicts):
- `f84f8d86be` - Fix mis-infer bounds in slice len/cap (❌ conflicts in prove.go)
- `f87aaec53d` - Fix integer overflow in prove pass (❌ conflicts in prove.go)

**Parser/Scanner Fixes**:
- `8e734ec954` - Fix BasicLit.End for raw strings with \r (⚠️ API change)
- `902dc27ae9` - Fix go/token cache race

**Recommendation**: Defer cherry-picks until we need specific fixes

## Testing After Merges

### Required Tests
1. **Build**: `cd src && ./make.bash`
2. **Goo Tests**: `./run_all_tests.sh` (should maintain ~113/125 passing)
3. **Transforms**: Test .goo files with GOO_USE_TRANSFORMERS=1
4. **Normal Go**: Ensure .go files work without transformers

### Test Locations
- `goo/test_*.goo` - Official Goo test suite (125 tests)
- `probes/test_*.goo` - Experimental/new features
- `src/cmd/compile/internal/transforms/*_test.go` - Unit tests

## Statistics

### Code Isolation (vs upstream go1.25.5)
```
Goo-Specific Code:
  transforms/           +13,414 lines (37 files) - 100% Goo-specific
  syntax/              +2,265 lines (net)        - Goo extensions
  noder/               +97 lines (net)           - Integration hook

Total Goo-specific:    ~15,776 lines of code
Total core changes:    <500 lines outside transforms/syntax
```

### Merge Conflict Risk Assessment
- **transforms/**: 🟢 Zero risk (upstream never touches)
- **syntax/**: 🟡 Low-medium risk (isolated extensions)
- **noder/**: 🟡 Low-medium risk (minimal integration code)
- **Core compiler**: 🟢 Low risk (minimal changes)

## Next Steps

### Immediate (Dec 2025)
1. ✅ Complete `check` keyword refactoring
2. ⏳ Monitor go1.25.x for new point releases
3. ⏳ Continue testing current setup

### Q1 2026
1. ⏳ Wait for official go1.26 release
2. ⏳ Merge go1.26 release branch when stable
3. ⏳ Update bootstrap toolchain if needed

### Continuous
- Monitor upstream/release-branch.go1.25 for security patches
- Keep Goo features isolated in transformers
- Document any new core compiler modifications

## Summary

**Current Merge Health**: ✅ **Excellent**
- Up to date with go1.25.5 (all commits merged)
- Goo features well-isolated (13,414 LoC in transforms/)
- Low risk for future go1.25.x merges
- Good preparation for eventual go1.26

**Conflict Risk**: 🟢 **Low**
- Syntax extensions isolated
- Transformers isolated
- Core compiler minimally modified
- No conflicts with current upstream

**Recommendation**:
- ✅ Continue monitoring go1.25.x for patches (merge immediately)
- ⏸️ Wait for official go1.26 before major version merge
- ✅ Continue refactoring to keep features isolated
- ✅ Current architecture well-positioned for easy merges
