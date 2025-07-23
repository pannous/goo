// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package buildtag defines an Analyzer that checks build tags.
package buildtag

import (
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/internal/analysisutil"
)

const Doc = "check //go:build and // +build directives"

var Analyzer = &analysis.Analyzer{
	Name: "buildtag",
	Doc:  Doc,
	URL:  "https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/buildtag",
	Run:  runBuildTag,
}

func runBuildTag(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		checkGoFile(pass, f)
	}
	for _, name := range pass.OtherFiles {
		if err := checkOtherFile(pass, name); err != nil {
			return nil, err
		}
	}
	for _, name := range pass.IgnoredFiles {
		if strings.HasSuffix(name, ".go") {
			f, err := parser.ParseFile(pass.Fset, name, nil, parser.ParseComments|parser.SkipObjectResolution)
			if err != nil {
				// Not valid Go source code - not our job to diagnose, so ignore.
				return nil, nil
			}
			checkGoFile(pass, f)
		} else {
			if err := checkOtherFile(pass, name); err != nil {
				return nil, err
			}
		}
	}
	return nil, nil
}

func checkGoFile(pass *analysis.Pass, f *ast.File) {
	var checks checker
	checks.init(pass)
	defer checks.finish()

	for _, group := range f.Comments {
		// A +build comment is ignored after or adjoining the package declaration.
		if group.End()+1 >= f.Package {
			checks.plusBuildOK = false
		}
		// A //go:build comment is ignored after the package declaration
		// (but adjoining it is OK, in contrast to +build comments).
		if group.Pos() >= f.Package {
			checks.goBuildOK = false
		}

		// Check each line of a //-comment.
		for _, c := range group.List {
			// "+build" is ignored within or after a /*...*/ comment.
			if !strings.HasPrefix(c.Text, "//") {
				checks.plusBuildOK = false
			}
			checks.comment(c.Slash, c.Text)
		}
	}
}

func checkOtherFile(pass *analysis.Pass, filename string) error {
	var checks checker
	checks.init(pass)
	defer checks.finish()

	// We cannot use the Go parser, since this may not be a Go source file.
	// Read the raw bytes instead.
	content, tf, err := analysisutil.ReadFile(pass, filename)
	if err != nil {
		return err
	}

	checks.file(token.Pos(tf.Base()), string(content))
	return nil
}

type checker struct {
	pass         *analysis.Pass
	plusBuildOK  bool            // "+build" lines still OK
	goBuildOK    bool            // "go:build" lines still OK
	crossCheck   bool            // cross-check go:build and +build lines when done reading file
	inStar       bool            // currently in a /* */ comment
	goBuildPos   token.Pos       // position of first go:build line found
	plusBuildPos token.Pos       // position of first "+build" line found
	goBuild      constraint.Expr // go:build constraint found
	plusBuild    constraint.Expr // AND of +build constraints found
}

func (checks *checker) init(pass *analysis.Pass) {
	checks.pass = pass
	checks.goBuildOK = true
	checks.plusBuildOK = true
	checks.crossCheck = true
}

func (checks *checker) file(pos token.Pos, text string) {
	// Determine cutpoint where +build comments are no longer valid.
	// They are valid in leading // comments in the file followed by
	// a blank line.
	//
	// This must be done as a separate pass because of the
	// requirement that the comment be followed by a blank line.
	var plusBuildCutoff int
	fullText := text
	for text != "" {
		i := strings.Index(text, "\n")
		if i < 0 {
			i = len(text)
		} else {
			i++
		}
		offset := len(fullText) - len(text)
		line := text[:i]
		text = text[i:]
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "//") && line != "" {
			break
		}
		if line == "" {
			plusBuildCutoff = offset
		}
	}

	// Process each line.
	// Must stop once we hit goBuildOK == false
	text = fullText
	checks.inStar = false
	for text != "" {
		i := strings.Index(text, "\n")
		if i < 0 {
			i = len(text)
		} else {
			i++
		}
		offset := len(fullText) - len(text)
		line := text[:i]
		text = text[i:]
		checks.plusBuildOK = offset < plusBuildCutoff

		if strings.HasPrefix(line, "//") {
			checks.comment(pos+token.Pos(offset), line)
			continue
		}

		// Keep looking for the point at which //go:build comments
		// stop being allowed. Skip over, cut out any /* */ comments.
		for {
			line = strings.TrimSpace(line)
			if checks.inStar {
				i := strings.Index(line, "*/")
				if i < 0 {
					line = ""
					break
				}
				line = line[i+len("*/"):]
				checks.inStar = false
				continue
			}
			if strings.HasPrefix(line, "/*") {
				checks.inStar = true
				line = line[len("/*"):]
				continue
			}
			break
		}
		if line != "" {
			// Found non-comment non-blank line.
			// Ends space for valid //go:build comments,
			// but also ends the fraction of the file we can
			// reliably parse. From this point on we might
			// incorrectly flag "comments" inside multiline
			// string constants or anything else (this might
			// not even be a Go program). So stop.
			break
		}
	}
}

func (checks *checker) comment(pos token.Pos, text string) {
	if strings.HasPrefix(text, "//") {
		if strings.Contains(text, "+build") {
			checks.plusBuildLine(pos, text)
		}
		if strings.Contains(text, "//go:build") {
			checks.goBuildLine(pos, text)
		}
	}
	if strings.HasPrefix(text, "/*") {
		if i := strings.Index(text, "\n"); i >= 0 {
			// multiline /* */ comment - process interior lines
			checks.inStar = true
			i++
			pos += token.Pos(i)
			text = text[i:]
			for text != "" {
				i := strings.Index(text, "\n")
				if i < 0 {
					i = len(text)
				} else {
					i++
				}
				line := text[:i]
				if strings.HasPrefix(line, "//") {
					checks.comment(pos, line)
				}
				pos += token.Pos(i)
				text = text[i:]
			}
			checks.inStar = false
		}
	}
}

func (checks *checker) goBuildLine(pos token.Pos, line string) {
	if !constraint.IsGoBuild(line) {
		if !strings.HasPrefix(line, "//go:build") && constraint.IsGoBuild("//"+strings.TrimSpace(line[len("//"):])) {
			checks.pass.Reportf(pos, "malformed //go:build line (space between // and go:build)")
		}
		return
	}
	if !checks.goBuildOK || checks.inStar {
		checks.pass.Reportf(pos, "misplaced //go:build comment")
		checks.crossCheck = false
		return
	}

	if checks.goBuildPos == token.NoPos {
		checks.goBuildPos = pos
	} else {
		checks.pass.Reportf(pos, "unexpected extra //go:build line")
		checks.crossCheck = false
	}

	// testing hack: stop at // ERROR
	if i := strings.Index(line, " // ERROR "); i >= 0 {
		line = line[:i]
	}

	x, err := constraint.Parse(line)
	if err != nil {
		checks.pass.Reportf(pos, "%v", err)
		checks.crossCheck = false
		return
	}

	checks.tags(pos, x)

	if checks.goBuild == nil {
		checks.goBuild = x
	}
}

func (checks *checker) plusBuildLine(pos token.Pos, line string) {
	line = strings.TrimSpace(line)
	if !constraint.IsPlusBuild(line) {
		// Comment with +build but not at beginning.
		// Only report early in file.
		if checks.plusBuildOK && !strings.HasPrefix(line, "// want") {
			checks.pass.Reportf(pos, "possible malformed +build comment")
		}
		return
	}
	if !checks.plusBuildOK { // inStar implies !plusBuildOK
		checks.pass.Reportf(pos, "misplaced +build comment")
		checks.crossCheck = false
	}

	if checks.plusBuildPos == token.NoPos {
		checks.plusBuildPos = pos
	}

	// testing hack: stop at // ERROR
	if i := strings.Index(line, " // ERROR "); i >= 0 {
		line = line[:i]
	}

	fields := strings.Fields(line[len("//"):])
	// IsPlusBuildConstraint check above implies fields[0] == "+build"
	for _, arg := range fields[1:] {
		for _, elem := range strings.Split(arg, ",") {
			if strings.HasPrefix(elem, "!!") {
				checks.pass.Reportf(pos, "invalid double negative in build constraint: %s", arg)
				checks.crossCheck = false
				continue
			}
			elem = strings.TrimPrefix(elem, "!")
			for _, c := range elem {
				if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' && c != '.' {
					checks.pass.Reportf(pos, "invalid non-alphanumeric build constraint: %s", arg)
					checks.crossCheck = false
					break
				}
			}
		}
	}

	if checks.crossCheck {
		y, err := constraint.Parse(line)
		if err != nil {
			// Should never happen - constraint.Parse never rejects a // +build line.
			// Also, we just checked the syntax above.
			// Even so, report.
			checks.pass.Reportf(pos, "%v", err)
			checks.crossCheck = false
			return
		}
		checks.tags(pos, y)

		if checks.plusBuild == nil {
			checks.plusBuild = y
		} else {
			checks.plusBuild = &constraint.AndExpr{X: checks.plusBuild, Y: y}
		}
	}
}

func (checks *checker) finish() {
	if !checks.crossCheck || checks.plusBuildPos == token.NoPos || checks.goBuildPos == token.NoPos {
		return
	}

	// Have both //go:build and // +build,
	// with no errors found (crossCheck still true).
	// Check they match.
	var want constraint.Expr
	lines, err := constraint.PlusBuildLines(checks.goBuild)
	if err != nil {
		checks.pass.Reportf(checks.goBuildPos, "%v", err)
		return
	}
	for _, line := range lines {
		y, err := constraint.Parse(line)
		if err != nil {
			// Definitely should not happen, but not the user's fault.
			// Do not report.
			return
		}
		if want == nil {
			want = y
		} else {
			want = &constraint.AndExpr{X: want, Y: y}
		}
	}
	if want.String() != checks.plusBuild.String() {
		checks.pass.Reportf(checks.plusBuildPos, "+build lines do not match //go:build condition")
		return
	}
}

// tags reports issues in go versions in tags within the expression e.
func (checks *checker) tags(pos token.Pos, e constraint.Expr) {
	// Use Eval to visit each tag.
	_ = e.Eval(func(tag string) bool {
		if malformedGoTag(tag) {
			checks.pass.Reportf(pos, "invalid go version %q in build constraint", tag)
		}
		return false // result is immaterial as Eval does not short-circuit
	})
}

// malformedGoTag returns true if a tag is likely to be a malformed
// go version constraint.
func malformedGoTag(tag string) bool {
	// Not a go version?
	if !strings.HasPrefix(tag, "go1") {
		// Check for close misspellings of the "go1." prefix.
		for _, pre := range []string{"go.", "g1.", "go"} {
			suffix := strings.TrimPrefix(tag, pre)
			if suffix != tag && validGoVersion("go1."+suffix) {
				return true
			}
		}
		return false
	}

	// The tag starts with "go1" so it is almost certainly a GoVersion.
	// Report it if it is not a valid build constraint.
	return !validGoVersion(tag)
}

// validGoVersion reports when a tag is a valid go version.
func validGoVersion(tag string) bool {
	return constraint.GoVersion(&constraint.TagExpr{Tag: tag}) != ""
}
