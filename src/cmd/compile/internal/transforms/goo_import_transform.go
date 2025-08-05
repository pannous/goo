// Copyright 2025 The Goo Authors. All rights reserved.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
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
	changed := false

	// Transform import declarations
	for i, decl := range file.DeclList {
		if importDecl, ok := decl.(*syntax.ImportDecl); ok {
			if newImportDecl := t.transformImportDecl(importDecl); newImportDecl != importDecl {
				file.DeclList[i] = newImportDecl
				changed = true
			}
		}
	}

	return changed
}

func (t *GooImportTransform) transformImportDecl(importDecl *syntax.ImportDecl) *syntax.ImportDecl {
	if importDecl.Path == nil || importDecl.Path.Kind != syntax.StringLit {
		return importDecl
	}

	importPath := strings.Trim(importDecl.Path.Value, "\"")

	// Check if this is a .goo file import
	if strings.HasSuffix(importPath, ".goo") {

		// Extract base name for package directory
		baseName := strings.TrimSuffix(filepath.Base(importPath), ".goo")

		// Create new relative import path
		newImportPath := "./" + baseName

		// Create new import declaration
		newImportDecl := *importDecl
		newPath := *importDecl.Path
		newPath.Value = "\"" + newImportPath + "\""
		newImportDecl.Path = &newPath

		return &newImportDecl
	}

	// Check if this is a local directory import that should be converted to relative
	if t.shouldConvertToLocalImport(importPath) {

		// Convert bare directory name to relative import
		newImportPath := "./" + importPath

		// Create new import declaration
		newImportDecl := *importDecl
		newPath := *importDecl.Path
		newPath.Value = "\"" + newImportPath + "\""
		newImportDecl.Path = &newPath

		return &newImportDecl
	}

	return importDecl
}

// shouldConvertToLocalImport checks if a bare import should be treated as local directory
func (t *GooImportTransform) shouldConvertToLocalImport(importPath string) bool {
	// DISABLED: This was interfering with auto-import system
	// The auto-import system needs to see bare package names like "strings"
	// to know when to auto-inject imports. Converting them to "./strings"
	// breaks this detection.
	return false
}

// isStandardLibrary checks if import is likely a standard library package
func (t *GooImportTransform) isStandardLibrary(importPath string) bool {
	stdLibPackages := []string{
		"fmt", "os", "io", "net", "http", "time", "strings", "strconv", "bytes",
		"bufio", "context", "errors", "log", "math", "path", "regexp", "sort",
		"sync", "testing", "unicode", "encoding", "crypto", "database", "debug",
		"go", "hash", "html", "image", "index", "mime", "reflect", "runtime",
		"text", "unsafe", "archive", "compress", "container", "embed",
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
