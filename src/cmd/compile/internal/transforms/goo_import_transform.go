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

func (t *GooImportTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	println("GooImportTransform.Transform called")
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
		println("TRANSFORMING .goo import:", importPath)

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
		println("TRANSFORMING local directory import:", importPath)
		
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
	
	// Check if local directory exists
	if _, err := os.Stat(importPath); err == nil {
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