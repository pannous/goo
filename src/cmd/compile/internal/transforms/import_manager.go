// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transforms

import (
	"cmd/compile/internal/syntax"
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
	trace("ImportManager: Requesting import %s -> %s", packageName, importPath)
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

	changed := false
	for packageName, importPath := range im.neededImports {
		if !im.hasImport(file, importPath) {
			im.addImport(file, packageName, importPath)
			changed = true
			trace("ImportManager: Added import %s (%s)", importPath, packageName)
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
	// Normalize import path (add quotes if missing)
	if importPath[0] != '"' {
		importPath = "\"" + importPath + "\""
	}

	// Create new import declaration
	newImport := &syntax.ImportDecl{
		Path: &syntax.BasicLit{
			Kind:  syntax.StringLit,
			Value: importPath,
		},
	}

	// Set position info if available
	if file.PkgName != nil {
		ensureImportPos(file, newImport)
	}

	// If package name is different from default, add alias
	defaultPackageName := im.getDefaultPackageName(importPath)
	if packageName != defaultPackageName {
		newImport.LocalPkgName = &syntax.Name{Value: packageName}
	}

	// Find insertion point (after existing imports)
	insertPos := 0
	for i, decl := range file.DeclList {
		if _, ok := decl.(*syntax.ImportDecl); ok {
			insertPos = i + 1
		} else {
			break
		}
	}

	// Insert the new import
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

func RequestMapsImport() {
	RequestStandardImport("maps")
}

func RequestMathImport() {
	RequestStandardImport("math")
}

func RequestFmtImport() {
	RequestStandardImport("fmt")
}

func RequestSortImport() {
	RequestStandardImport("sort")
}

func RequestStrconvImport() {
	RequestStandardImport("strconv")
}
