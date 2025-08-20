// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
	"os"
	"path/filepath"
	"strings"
)

// GooImportTransform handles local .goo file imports
// Transforms import "helper.goo" into import "./helper" and ensures proper package structure
type GooImportTransform struct{}

func (t *GooImportTransform) Name() string {
	return "goo_import_transform"
}

func (t *GooImportTransform) Priority() int {
	return 100 // Default priority - between list methods (50) and lambda (200)
}

func (t *GooImportTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	// Temporarily disabled to isolate PosBase panic
	return false
	
	// Get source file directory for relative path resolution
	var sourceDir string
	if file.Path != nil {
		sourceDir = filepath.Dir(file.Path.Filename())
	} else {
		// Fallback to current working directory
		sourceDir = "."
	}
	
	changed := false

	// Transform import declarations
	for i, decl := range file.DeclList {
		if importDecl, ok := decl.(*syntax.ImportDecl); ok {
			if newImportDecl := t.transformImportDeclWithContext(importDecl, sourceDir); newImportDecl != importDecl {
				file.DeclList[i] = newImportDecl
				changed = true
			}
		}
	}

	return changed
}

func (t *GooImportTransform) transformImportDeclWithContext(importDecl *syntax.ImportDecl, sourceDir string) *syntax.ImportDecl {
	if importDecl.Path == nil || importDecl.Path.Kind != syntax.StringLit {
		return importDecl
	}

	importPath := strings.Trim(importDecl.Path.Value, "\"")

	// Debug: print what we're processing
	println("GooImportTransform processing import:", importPath)

	// Check if this is a .goo file import
	if strings.HasSuffix(importPath, ".goo") {
		println("Transforming .goo import:", importPath)

		// Extract base name for package directory
		baseName := strings.TrimSuffix(filepath.Base(importPath), ".goo")

		// Create new relative import path
		newImportPath := "./" + baseName

		println("Transformed .goo import to:", newImportPath)

		// Create new import declaration
		newImportDecl := *importDecl
		newPath := *importDecl.Path
		newPath.Value = "\"" + newImportPath + "\""
		// Preserve position information
		if importDecl.Path.Pos().IsKnown() {
			newPath.SetPos(importDecl.Path.Pos())
		}
		newImportDecl.Path = &newPath

		return &newImportDecl
	}

	// Check if this is a local directory import that should be converted to relative
	if t.shouldConvertToLocalImport(importPath, sourceDir) {
		// Convert bare directory name to relative import
		newImportPath := "./" + importPath

		// Create new import declaration
		newImportDecl := *importDecl
		newPath := *importDecl.Path
		newPath.Value = "\"" + newImportPath + "\""
		// Preserve position information
		if importDecl.Path.Pos().IsKnown() {
			newPath.SetPos(importDecl.Path.Pos())
		}
		newImportDecl.Path = &newPath

		return &newImportDecl
	}

	return importDecl
}

// shouldConvertToLocalImport checks if a bare import should be treated as local directory
func (t *GooImportTransform) shouldConvertToLocalImport(importPath string, sourceDir string) bool {
	// Skip if already relative or absolute
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") || filepath.IsAbs(importPath) {
		return false
	}
	
	// Skip standard library packages (simple heuristic: no dots or well-known names)  
	if t.isStandardLibrary(importPath) {
		return false
	}
	
	// Skip if it looks like a module path (contains dots/slashes)
	if strings.Contains(importPath, ".") || strings.Contains(importPath, "/") {
		return false
	}
	
	// Check if local directory exists relative to the source file directory
	localDir := filepath.Join(sourceDir, importPath)
	if _, err := os.Stat(localDir); err == nil {
		return true
	}
	
	return false
}

// isStandardLibrary checks if import is likely a standard library package
func (t *GooImportTransform) isStandardLibrary(importPath string) bool {
	stdLibPackages := []string{
		"fmt", "os", "io", "net", "http", "time", "strings", "strconv", "bytes",
		"bufio", "context", "errors", "log", "math", "path", "regexp", "sort",
		"sync", "testing", "unicode", "encoding", "crypto", "database", "debug",
		"go", "hash", "html", "image", "index", "mime", "reflect", "runtime",
		"text", "unsafe", "archive", "compress", "container", "embed", "slices",
		"maps", "cmp", "iter", "units", "builtin",
	}

	for _, pkg := range stdLibPackages {
		if importPath == pkg {
			return true
		}
	}

	return false
}

func init() {
	RegisterTransformer(&GooImportTransform{})
}
