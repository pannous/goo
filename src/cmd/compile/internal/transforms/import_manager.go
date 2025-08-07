// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"strings"
)

// ImportManager centralizes import handling for all transformers
// This solves the import resolution issues by collecting all needed imports
// and adding them in a single, coordinated way
type ImportManager struct {
	neededImports map[string]string // package -> import path
}

// NewImportManager creates a new import manager
func NewImportManager() *ImportManager {
	return &ImportManager{
		neededImports: make(map[string]string),
	}
}

// RequestImport registers that a package is needed
func (im *ImportManager) RequestImport(packageName, importPath string) {
	im.neededImports[packageName] = importPath
	fmt.Printf("ImportManager: Requesting import %s -> %s\n", packageName, importPath)
}

// GetRequestedImports returns all requested imports
func (im *ImportManager) GetRequestedImports() map[string]string {
	return im.neededImports
}

// ApplyImports adds all needed imports to the file
func (im *ImportManager) ApplyImports(file *syntax.File) bool {
	if len(im.neededImports) == 0 {
		return false
	}
	
	fmt.Printf("DEBUG: ImportManager.ApplyImports called, file has %d declarations\n", len(file.DeclList))
	
	changed := false
	for packageName, importPath := range im.neededImports {
		if !im.hasImport(file, importPath) {
			im.addImport(file, packageName, importPath)
			changed = true
			fmt.Printf("ImportManager: Added import %s (%s) - file now has %d declarations\n", importPath, packageName, len(file.DeclList))
		}
	}
	
	// Clear requests after applying
	im.neededImports = make(map[string]string)
	return changed
}

// hasImport checks if the file already has the given import
func (im *ImportManager) hasImport(file *syntax.File, importPath string) bool {
	// Normalize import path (add quotes if missing)
	if importPath[0] != '"' {
		importPath = "\"" + importPath + "\""
	}
	
	for _, decl := range file.DeclList {
		if importDecl, ok := decl.(*syntax.ImportDecl); ok {
			if importDecl.Path != nil && importDecl.Path.Value == importPath {
				return true
			}
		}
	}
	return false
}

// addImport adds a new import to the file
func (im *ImportManager) addImport(file *syntax.File, packageName, importPath string) {
	// Use EXACT same logic as the working old system
	if im.hasImport(file, importPath) {
		return
	}

	// Normalize import path (add quotes if missing)
	if importPath[0] != '"' {
		importPath = "\"" + importPath + "\""
	}

	// EXACT copy of working addStringsImport pattern
	newImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Value: importPath,
			Kind:  syntax.StringLit,
		},
	}
	fmt.Printf("DEBUG ImportManager: Creating import with Value='%s', Kind=%d\n", importPath, syntax.StringLit)
	newImport.SetPos(syntax.Pos{})

	// Find insertion point (after existing imports)  
	var insertPos int
	for i, decl := range file.DeclList {
		if _, ok := decl.(*syntax.ImportDecl); ok {
			insertPos = i + 1
		} else {
			break
		}
	}

	// Insert the new import using EXACT same pattern
	newDeclList := make([]syntax.Decl, 0, len(file.DeclList)+1)
	newDeclList = append(newDeclList, file.DeclList[:insertPos]...)
	newDeclList = append(newDeclList, newImport)
	newDeclList = append(newDeclList, file.DeclList[insertPos:]...)
	file.DeclList = newDeclList
}

// getDefaultPackageName extracts the default package name from import path
func (im *ImportManager) getDefaultPackageName(importPath string) string {
	// Remove quotes
	path := strings.Trim(importPath, "\"")
	
	// Extract last component
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

// Global import manager instance
var GlobalImportManager = NewImportManager()

// Helper functions for transformers to request imports
func RequestImport(packageName, importPath string) {
	GlobalImportManager.RequestImport(packageName, importPath)
}

func RequestStandardImport(packageName string) {
	// Standard library packages have simple paths
	GlobalImportManager.RequestImport(packageName, packageName)
}

// Common import requests
func RequestStringsImport() {
	RequestStandardImport("strings")
}

func RequestSlicesImport() {
	RequestStandardImport("slices")
}

func RequestFmtImport() {
	RequestStandardImport("fmt")
}

func RequestSortImport() {
	RequestStandardImport("sort")
}