// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements (error and trace) message formatting support.

package types

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

func sprintf(fset *token.FileSet, qf Qualifier, tpSubscripts bool, format string, args ...any) string {
	for i, arg := range args {
		switch a := arg.(type) {
		case nil:
			arg = "<nil>"
		case operand:
			panic("got operand instead of *operand")
		case *operand:
			arg = operandString(a, qf)
		case token.Pos:
			if fset != nil {
				arg = fset.Position(a).String()
			}
		case ast.Expr:
			arg = ExprString(a)
		case []ast.Expr:
			var buf bytes.Buffer
			buf.WriteByte('[')
			writeExprList(&buf, a)
			buf.WriteByte(']')
			arg = buf.String()
		case Object:
			arg = ObjectString(a, qf)
		case Type:
			var buf bytes.Buffer
			w := newTypeWriter(&buf, qf)
			w.tpSubscripts = tpSubscripts
			w.typ(a)
			arg = buf.String()
		case []Type:
			var buf bytes.Buffer
			w := newTypeWriter(&buf, qf)
			w.tpSubscripts = tpSubscripts
			buf.WriteByte('[')
			for i, x := range a {
				if i > 0 {
					buf.WriteString(", ")
				}
				w.typ(x)
			}
			buf.WriteByte(']')
			arg = buf.String()
		case []*TypeParam:
			var buf bytes.Buffer
			w := newTypeWriter(&buf, qf)
			w.tpSubscripts = tpSubscripts
			buf.WriteByte('[')
			for i, x := range a {
				if i > 0 {
					buf.WriteString(", ")
				}
				w.typ(x)
			}
			buf.WriteByte(']')
			arg = buf.String()
		}
		args[i] = arg
	}
	return fmt.Sprintf(format, args...)
}

// check may be nil.
func (checks *Checker) sprintf(format string, args ...any) string {
	var fset *token.FileSet
	var qf Qualifier
	if checks != nil {
		fset = checks.fset
		qf = checks.qualifier
	}
	return sprintf(fset, qf, false, format, args...)
}

func (checks *Checker) trace(pos token.Pos, format string, args ...any) {
	pos1 := checks.fset.Position(pos)
	// Use the width of line and pos values to align the ":" by adding padding before it.
	// Cap padding at 5: 3 digits for the line, 2 digits for the column number, which is
	// ok for most cases.
	w := ndigits(pos1.Line) + ndigits(pos1.Column)
	pad := "     "[:max(5-w, 0)]
	fmt.Printf("%s%s:  %s%s\n",
		pos1,
		pad,
		strings.Repeat(".  ", checks.indent),
		sprintf(checks.fset, checks.qualifier, true, format, args...),
	)
}

// ndigits returns the number of decimal digits in x.
// For x < 10, the result is always 1.
// For x > 100, the result is always 3.
func ndigits(x int) int {
	switch {
	case x < 10:
		return 1
	case x < 100:
		return 2
	default:
		return 3
	}
}

// dump is only needed for debugging
func (checks *Checker) dump(format string, args ...any) {
	fmt.Println(sprintf(checks.fset, checks.qualifier, true, format, args...))
}

func (checks *Checker) qualifier(pkg *Package) string {
	// Qualify the package unless it's the package being type-checked.
	if pkg != checks.pkg {
		if checks.pkgPathMap == nil {
			checks.pkgPathMap = make(map[string]map[string]bool)
			checks.seenPkgMap = make(map[*Package]bool)
			checks.markImports(checks.pkg)
		}
		// If the same package name was used by multiple packages, display the full path.
		if len(checks.pkgPathMap[pkg.name]) > 1 {
			return strconv.Quote(pkg.path)
		}
		return pkg.name
	}
	return ""
}

// markImports recursively walks pkg and its imports, to record unique import
// paths in pkgPathMap.
func (checks *Checker) markImports(pkg *Package) {
	if checks.seenPkgMap[pkg] {
		return
	}
	checks.seenPkgMap[pkg] = true

	forName, ok := checks.pkgPathMap[pkg.name]
	if !ok {
		forName = make(map[string]bool)
		checks.pkgPathMap[pkg.name] = forName
	}
	forName[pkg.path] = true

	for _, imp := range pkg.imports {
		checks.markImports(imp)
	}
}

// stripAnnotations removes internal (type) annotations from s.
func stripAnnotations(s string) string {
	var buf strings.Builder
	for _, r := range s {
		// strip #'s and subscript digits
		if r < '₀' || '₀'+10 <= r { // '₀' == U+2080
			buf.WriteRune(r)
		}
	}
	if buf.Len() < len(s) {
		return buf.String()
	}
	return s
}
