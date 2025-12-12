# Goo Binary Release Size Optimization

## Current Size Analysis

### Original Release (125MB compressed, 344MB extracted)
- bin/: 104MB (includes 56MB of Windows .exe files!)
- src/: 199MB (entire Go source tree including cmd/)
- pkg/: 31MB (only compile tool, missing asm/link/cgo/cover)
- api/: 8.6MB
- lib/: 1.7MB
- misc/: 124KB

### Optimized Release (113MB compressed, estimated ~310MB extracted)
- Removes cmd/ from src/ (-60MB uncompressed, -12MB compressed)
- Missing: Windows .exe removal, complete toolchain

## Optimization Opportunities

### 1. Remove Platform-Specific Binaries (Easy Win: -56MB)
**Current:** Darwin release includes Windows .exe files
```bash
# These shouldn't be in darwin-arm64 release:
bin/go.exe (18MB)
bin/go-amd.exe (20MB)
bin/go-arm64.exe (18MB)
```
**Impact:** -56MB uncompressed, ~-20MB compressed

### 2. Complete Toolchain Required (+19MB)
**Issue:** Missing essential tools (asm, link, cgo, cover)
- Current: only compile (31MB)
- Need: asm (4.5MB), link (5.8MB), cgo (4MB), cover (4.7MB)
**Impact:** +19MB for complete functionality

### 3. Source Directory Optimization
**Options:**
- **A) Full source (current):** 199MB - needed for cgo, debugging, rebuilding packages
- **B) No cmd/ source (implemented):** 139MB - removes tool source code
- **C) Stdlib only (aggressive):** ~100MB - might break some edge cases
- **D) Depend on system Go:** Require users have Go installed, reference system stdlib

**Recommendation:** Keep option B (no cmd/ source)

## Comparison with Standard Go

Standard Go 1.25.5 via Homebrew: 232MB uncompressed
- bin/: 16MB (go, gofmt)
- src/: 139MB
- pkg/: 48MB (all tools)
- api/: ~29MB

Goo is comparable in size and slightly smaller when optimized.

## Can We Share Resources with System Go?

### Short Answer: No (for now)

**Why not:**
1. **Version mismatch:** Goo uses Go 1.26.1, system might have 1.25.5
2. **Modified stdlib:** Goo has custom transformers and language extensions
3. **Tool compatibility:** Compile tool must match link/asm versions exactly
4. **Modified compiler:** The entire reason Goo exists is custom compilation

### Hybrid Approach (Future Consideration)

Could potentially:
- Ship only modified components (bin/compile, transforms/)
- Reference system Go for unmodified stdlib packages
- Detect and warn about version mismatches

**Challenges:**
- Complex dependency detection
- Version compatibility matrix
- User experience (installation becomes more fragile)
- Debugging becomes harder

## Recommended Optimizations

### Phase 1: Immediate (Target: ~90-95MB compressed)
1. ✅ Remove cmd/ from source distribution (-12MB compressed)
2. ⬜ Remove Windows .exe from Darwin releases (-20MB compressed)
3. ⬜ Build complete toolchain (asm, link, cgo) (+7MB compressed)

### Phase 2: Future Consideration
4. More aggressive source pruning (test data, examples)
5. Compress with better algorithms (zstd vs gzip)
6. Optional "minimal" vs "full" release variants

## Build Script Updates Needed

```bash
# scripts/package-release.sh should:
1. Build complete toolchain for target platform
2. Exclude cmd/ source from distribution
3. Only include binaries for target platform
4. Strip debug symbols if appropriate (careful - affects debugging)
```

## Answer to Original Question

**Q:** Does our binary really need to be so big? Can we share with pre-existing Go?

**A:**
- Current size (125MB) is comparable to standard Go (232MB uncompressed)
- Can optimize to ~90-95MB by removing waste (Windows .exe, cmd/ source)
- **Cannot share with system Go** because:
  - Goo's entire value is custom compilation with language extensions
  - Modified stdlib and compiler require matched versions
  - Toolchain components must be version-compatible

The Homebrew tap repository (2GB) is large due to:
- Full git history (1.4GB) - normal for git repos
- Source code (199MB) - needed for development
- Built artifacts (bins, caches) - can be gitignored

For distribution, users download only the 125MB (soon ~95MB) tarball, not the full repo.
