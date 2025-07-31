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

	// Check if this is a .goo file import
	importPath := strings.Trim(importDecl.Path.Value, "\"")
	if !strings.HasSuffix(importPath, ".goo") {
		return importDecl
	}

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

func init() {
	RegisterTransformer(&GooImportTransform{})
}