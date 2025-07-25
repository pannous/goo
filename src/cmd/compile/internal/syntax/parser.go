// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

import (
	"fmt"
	"go/build/constraint"
	"io"
	"strconv"
	"strings"
)

const debug = false
const trace = false

type parser struct {
	file  *PosBase
	errh  ErrorHandler
	mode  Mode
	pragh PragmaHandler
	scanner

	base      *PosBase // current position base
	first     error    // first error encountered
	errcnt    int      // number of errors encountered
	pragma    Pragma   // pragmas
	goVersion string   // Go version from //go:build line

	top    bool   // in top of file (before package clause)
	fnest  int    // function nesting level (for error handling)
	xnest  int    // expression nesting level (for complit ambiguity resolution)
	indent []byte // tracing support
}

func (p *parser) init(file *PosBase, r io.Reader, errh ErrorHandler, pragh PragmaHandler, mode Mode) {
	p.top = true
	p.file = file
	p.errh = errh
	p.mode = mode
	p.pragh = pragh
	p.scanner.init(
		r,
		// Error and directive handler for scanner.
		// Because the (line, col) positions passed to the
		// handler is always at or after the current reading
		// position, it is safe to use the most recent position
		// base to compute the corresponding Pos value.
		func(line, col uint, msg string) {
			if msg[0] != '/' {
				p.errorAt(p.posAt(line, col), msg)
				return
			}

			// otherwise it must be a comment containing a line or go: directive.
			// //line directives must be at the start of the line (column colbase).
			// /*line*/ directives can be anywhere in the line.
			text := commentText(msg)
			if (col == colbase || msg[1] == '*') && strings.HasPrefix(text, "line ") {
				var pos Pos // position immediately following the comment
				if msg[1] == '/' {
					// line comment (newline is part of the comment)
					pos = MakePos(p.file, line+1, colbase)
				} else {
					// regular comment
					// (if the comment spans multiple lines it's not
					// a valid line directive and will be discarded
					// by updateBase)
					pos = MakePos(p.file, line, col+uint(len(msg)))
				}
				p.updateBase(pos, line, col+2+5, text[5:]) // +2 to skip over // or /*
				return
			}

			// go: directive (but be conservative and test)
			if strings.HasPrefix(text, "go:") {
				if p.top && strings.HasPrefix(msg, "//go:build") {
					if x, err := constraint.Parse(msg); err == nil {
						p.goVersion = constraint.GoVersion(x)
					}
				}
				if pragh != nil {
					p.pragma = pragh(p.posAt(line, col+2), p.scanner.blank, text, p.pragma) // +2 to skip over // or /*
				}
			}
		},
		directives,
	)

	p.base = file
	p.first = nil
	p.errcnt = 0
	p.pragma = nil

	p.fnest = 0
	p.xnest = 0
	p.indent = nil
}

// takePragma returns the current parsed pragmas
// and clears them from the parser state.
func (p *parser) takePragma() Pragma {
	prag := p.pragma
	p.pragma = nil
	return prag
}

// clearPragma is called at the end of a statement or
// other Go form that does NOT accept a pragma.
// It sends the pragma back to the pragma handler
// to be reported as unused.
func (p *parser) clearPragma() {
	if p.pragma != nil {
		p.pragh(p.pos(), p.scanner.blank, "", p.pragma)
		p.pragma = nil
	}
}

// updateBase sets the current position base to a new line base at pos.
// The base's filename, line, and column values are extracted from text
// which is positioned at (tline, tcol) (only needed for error messages).
func (p *parser) updateBase(pos Pos, tline, tcol uint, text string) {
	i, n, ok := trailingDigits(text)
	if i == 0 {
		return // ignore (not a line directive)
	}
	// i > 0

	if !ok {
		// text has a suffix :xxx but xxx is not a number
		p.errorAt(p.posAt(tline, tcol+i), "invalid line number: "+text[i:])
		return
	}

	var line, col uint
	i2, n2, ok2 := trailingDigits(text[:i-1])
	if ok2 {
		//line filename:line:col
		i, i2 = i2, i
		line, col = n2, n
		if col == 0 || col > PosMax {
			p.errorAt(p.posAt(tline, tcol+i2), "invalid column number: "+text[i2:])
			return
		}
		text = text[:i2-1] // lop off ":col"
	} else {
		//line filename:line
		line = n
	}

	if line == 0 || line > PosMax {
		p.errorAt(p.posAt(tline, tcol+i), "invalid line number: "+text[i:])
		return
	}

	// If we have a column (//line filename:line:col form),
	// an empty filename means to use the previous filename.
	filename := text[:i-1] // lop off ":line"
	trimmed := false
	if filename == "" && ok2 {
		filename = p.base.Filename()
		trimmed = p.base.Trimmed()
	}

	p.base = NewLineBase(pos, filename, trimmed, line, col)
}

func commentText(s string) string {
	if s[:2] == "/*" {
		return s[2 : len(s)-2] // lop off /* and */
	}

	// line comment (does not include newline)
	// (on Windows, the line comment may end in \r\n)
	i := len(s)
	if s[i-1] == '\r' {
		i--
	}
	return s[2:i] // lop off //, and \r at end, if any
}

func trailingDigits(text string) (uint, uint, bool) {
	i := strings.LastIndexByte(text, ':') // look from right (Windows filenames may contain ':')
	if i < 0 {
		return 0, 0, false // no ':'
	}
	// i >= 0
	n, err := strconv.ParseUint(text[i+1:], 10, 0)
	return uint(i + 1), uint(n), err == nil
}

func (p *parser) got(tok token) bool {
	if p.tok == tok {
		p.next()
		return true
	}
	return false
}

func (p *parser) want(tok token) {
	if !p.got(tok) {
		p.syntaxError("expected " + tokstring(tok))
		p.advance()
	}
}

// gotAssign is like got(_Assign) but it also accepts ":="
// (and reports an error) for better parser error recovery.
func (p *parser) gotAssign() bool {
	switch p.tok {
	case _Define:
		p.syntaxError("expected =")
		fallthrough
	case _Assign:
		p.next()
		return true
	default:
		return false
	}
}

// ----------------------------------------------------------------------------
// Error handling

// posAt returns the Pos value for (line, col) and the current position base.
func (p *parser) posAt(line, col uint) Pos {
	return MakePos(p.base, line, col)
}

// errorAt reports an error at the given position.
func (p *parser) errorAt(pos Pos, msg string) {
	err := Error{pos, msg}
	if p.first == nil {
		p.first = err
	}
	p.errcnt++
	if p.errh == nil {
		panic(p.first)
	}
	p.errh(err)
}

// syntaxErrorAt reports a syntax error at the given position.
func (p *parser) syntaxErrorAt(pos Pos, msg string) {
	if trace {
		p.print("syntax error: " + msg)
	}

	if p.tok == _EOF && p.first != nil {
		return // avoid meaningless follow-up errors
	}

	// add punctuation etc. as needed to msg
	switch {
	case msg == "":
		// nothing to do
	case strings.HasPrefix(msg, "in "), strings.HasPrefix(msg, "at "), strings.HasPrefix(msg, "after "):
		msg = " " + msg
	case strings.HasPrefix(msg, "expected "):
		msg = ", " + msg
	default:
		// plain error - we don't care about current token
		p.errorAt(pos, "syntax error: "+msg)
		return
	}

	// determine token string
	var tok string
	switch p.tok {
	case _Name:
		tok = "name " + p.lit
	case _Semi:
		tok = p.lit
	case _Literal:
		tok = "literal " + p.lit
	case _Operator:
		tok = p.op.String()
	case _AssignOp:
		tok = p.op.String() + "="
	case _IncOp:
		tok = p.op.String()
		tok += tok
	default:
		tok = tokstring(p.tok)
	}

	// TODO(gri) This may print "unexpected X, expected Y".
	//           Consider "got X, expected Y" in this case.
	p.errorAt(pos, "syntax error: unexpected "+tok+msg)
}

// tokstring returns the English word for selected punctuation tokens
// for more readable error messages. Use tokstring (not tok.String())
// for user-facing (error) messages; use tok.String() for debugging
// output.
func tokstring(tok token) string {
	switch tok {
	case _Comma:
		return "comma"
	case _Semi:
		return "semicolon or newline"
	default:
	}
	s := tok.String()
	if _Break <= tok && tok <= _Var {
		return "keyword " + s
	}
	return s
}

// Convenience methods using the current token position.
func (p *parser) pos() Pos               { return p.posAt(p.line, p.col) }
func (p *parser) error(msg string)       { p.errorAt(p.pos(), msg) }
func (p *parser) syntaxError(msg string) { p.syntaxErrorAt(p.pos(), msg) }

// The stopset contains keywords that start a statement.
// They are good synchronization points in case of syntax
// errors and (usually) shouldn't be skipped over.
const stopset uint64 = 1<<_Break |
	1<<_Const |
	1<<_Continue |
	1<<_Defer |
	1<<_Fallthrough |
	1<<_For |
	1<<_Go |
	1<<_Goto |
	1<<_If |
	1<<_Return |
	1<<_Select |
	1<<_Switch |
	1<<_Type |
	1<<_Var

// advance consumes tokens until it finds a token of the stopset or followlist.
// The stopset is only considered if we are inside a function (p.fnest > 0).
// The followlist is the list of valid tokens that can follow a production;
// if it is empty, exactly one (non-EOF) token is consumed to ensure progress.
func (p *parser) advance(followlist ...token) {
	if trace {
		p.print(fmt.Sprintf("advance %s", followlist))
	}

	// compute follow set
	// (not speed critical, advance is only called in error situations)
	var followset uint64 = 1 << _EOF // don't skip over EOF
	if len(followlist) > 0 {
		if p.fnest > 0 {
			followset |= stopset
		}
		for _, tok := range followlist {
			followset |= 1 << tok
		}
	}

	for !contains(followset, p.tok) {
		if trace {
			p.print("skip " + p.tok.String())
		}
		p.next()
		if len(followlist) == 0 {
			break
		}
	}

	if trace {
		p.print("next " + p.tok.String())
	}
}

// usage: defer p.trace(msg)()
func (p *parser) trace(msg string) func() {
	p.print(msg + " (")
	const tab = ". "
	p.indent = append(p.indent, tab...)
	return func() {
		p.indent = p.indent[:len(p.indent)-len(tab)]
		if x := recover(); x != nil {
			panic(x) // skip print_trace
		}
		p.print(")")
	}
}

func (p *parser) print(msg string) {
	fmt.Printf("%5d: %s%s\n", p.line, p.indent, msg)
}

// ----------------------------------------------------------------------------
// Package files
//
// Parse methods are annotated with matching Go productions as appropriate.
// The annotations are intended as guidelines only since a single Go grammar
// rule may be covered by multiple parse methods and vice versa.
//
// Excluding methods returning slices, parse methods named xOrNil may return
// nil; all others are expected to return a valid non-nil node.

// SourceFile = PackageClause ";" { ImportDecl ";" } { TopLevelDecl ";" } .
func (p *parser) fileOrNil() *File {
	if trace {
		defer p.trace("file")()
	}

	f := new(File)
	f.pos = p.pos()

	// PackageClause
	f.GoVersion = p.goVersion
	p.top = false
	if p.tok != _Package {
		// Auto-inject "package main" if no package declaration found
		f.Pragma = nil
		f.PkgName = NewName(p.pos(), "main")
	} else {
		p.next() // consume the package token
		f.Pragma = p.takePragma()
		f.PkgName = p.name()
		p.want(_Semi)
	}

	// don't bother continuing if package clause has errors
	if p.first /*error*/ != nil {
		return nil
	}

	// Accept import declarations anywhere for error tolerance, but complain.
	// { ( ImportDecl | TopLevelDecl ) ";" }

	prev := _Import
	for p.tok != _EOF {
		if p.tok == _Import && prev != _Import {
			p.syntaxError("imports must appear before other declarations")
		}
		prev = p.tok

		switch p.tok {
		case _Import:
			p.next()
			f.DeclList = p.appendGroup(f.DeclList, p.importDecl)

		case _Const:
			p.next()
			f.DeclList = p.appendGroup(f.DeclList, p.constDecl)

		case _Type:
			p.next()
			f.DeclList = p.appendGroup(f.DeclList, p.typeDecl)

		case _Enum:
			p.next()
			f.DeclList = p.enumDeclGroup(f.DeclList)

		case _Var:
			p.next()
			f.DeclList = p.appendGroup(f.DeclList, p.varDecl)

		case _Func:
			p.next()
			if d := p.funcDeclOrNil(); d != nil {
				f.DeclList = append(f.DeclList, d)
			}

		case _Name:
			if p.lit == "def" {
				p.next()
				if d := p.funcDeclOrNil(); d != nil {
					f.DeclList = append(f.DeclList, d)
				}
				break
			}
			fallthrough

		default:
			if p.tok == _Lbrace && len(f.DeclList) > 0 && isEmptyFuncDecl(f.DeclList[len(f.DeclList)-1]) {
				// opening { of function declaration on next line
				p.syntaxError("unexpected semicolon or newline before {")
				p.advance(_Import, _Const, _Type, _Var, _Func)
				continue
			} else {
				// Parse as a top-level statement for implicit main
				if stmt := p.stmtOrNil(); stmt != nil {
					f.TopLevelStmts = append(f.TopLevelStmts, stmt)
				}
			}
		}

		// Reset p.pragma BEFORE advancing to the next token (consuming ';')
		// since comments before may set pragmas for the next function decl.
		p.clearPragma()

		if p.tok != _EOF && !p.got(_Semi) {
			p.syntaxError("after top level declaration")
			p.advance(_Import, _Const, _Type, _Var, _Func)
		}
	}
	// p.tok == _EOF

	p.clearPragma()
	f.EOF = p.pos()

	// Generate implicit main function if needed
	if f.PkgName != nil && f.PkgName.Value == "main" && len(f.TopLevelStmts) > 0 {
		// Check if main function already exists
		hasMain := false
		for _, decl := range f.DeclList {
			if fd, ok := decl.(*FuncDecl); ok && fd.Name.Value == "main" && fd.Recv == nil {
				hasMain = true
				break
			}
		}

		if !hasMain {
			// Create synthetic main function
			mainFunc := &FuncDecl{
				Name: NewName(f.pos, "main"),
				Type: &FuncType{},
				Body: &BlockStmt{
					List: f.TopLevelStmts,
				},
			}
			mainFunc.SetPos(f.pos)
			f.DeclList = append(f.DeclList, mainFunc)
			// Clear TopLevelStmts since they're now in main
			f.TopLevelStmts = nil
		}
	}

	return f
}

func isEmptyFuncDecl(dcl Decl) bool {
	f, ok := dcl.(*FuncDecl)
	return ok && f.Body == nil
}

// ----------------------------------------------------------------------------
// Declarations

// list parses a possibly empty, sep-separated list of elements, optionally
// followed by sep, and closed by close (or EOF). sep must be one of _Comma
// or _Semi, and close must be one of _Rparen, _Rbrace, or _Rbrack.
//
// For each list element, f is called. Specifically, unless we're at close
// (or EOF), f is called at least once. After f returns true, no more list
// elements are accepted. list returns the position of the closing token.
//
// list = [ f { sep f } [sep] ] close .
func (p *parser) list(context string, sep, close token, f func() bool) Pos {
	if debug && (sep != _Comma && sep != _Semi || close != _Rparen && close != _Rbrace && close != _Rbrack) {
		panic("invalid sep or close argument for list")
	}

	done := false
	for p.tok != _EOF && p.tok != close && !done {
		done = f()
		// sep is optional before close
		if !p.got(sep) && p.tok != close {
			p.syntaxError(fmt.Sprintf("in %s; possibly missing %s or %s", context, tokstring(sep), tokstring(close)))
			p.advance(_Rparen, _Rbrack, _Rbrace)
			if p.tok != close {
				// position could be better but we had an error so we don't care
				return p.pos()
			}
		}
	}

	pos := p.pos()
	p.want(close)
	return pos
}

// appendGroup(f) = f | "(" { f ";" } ")" . // ";" is optional before ")"
func (p *parser) appendGroup(list []Decl, f func(*Group) Decl) []Decl {
	if p.tok == _Lparen {
		g := new(Group)
		p.clearPragma()
		p.next() // must consume "(" after calling clearPragma!
		p.list("grouped declaration", _Semi, _Rparen, func() bool {
			if x := f(g); x != nil {
				list = append(list, x)
			}
			return false
		})
	} else {
		if x := f(nil); x != nil {
			list = append(list, x)
		}
	}
	return list
}

// ImportSpec = [ "." | PackageName ] ImportPath .
// ImportPath = string_lit .
func (p *parser) importDecl(group *Group) Decl {
	if trace {
		defer p.trace("importDecl")()
	}

	d := new(ImportDecl)
	d.pos = p.pos()
	d.Group = group
	d.Pragma = p.takePragma()

	switch p.tok {
	case _Name:
		d.LocalPkgName = p.name()
	case _Dot:
		d.LocalPkgName = NewName(p.pos(), ".")
		p.next()
	}
	d.Path = p.oliteral()
	if d.Path == nil {
		p.syntaxError("missing import path")
		p.advance(_Semi, _Rparen)
		return d
	}
	if !d.Path.Bad && d.Path.Kind != StringLit {
		p.syntaxErrorAt(d.Path.Pos(), "import path must be a string")
		d.Path.Bad = true
	}
	// d.Path.Bad || d.Path.Kind == StringLit

	return d
}

// ConstSpec = IdentifierList [ [ Type ] "=" ExpressionList ] .
func (p *parser) constDecl(group *Group) Decl {
	if trace {
		defer p.trace("constDecl")()
	}

	d := new(ConstDecl)
	d.pos = p.pos()
	d.Group = group
	d.Pragma = p.takePragma()

	d.NameList = p.nameList(p.name())
	if p.tok != _EOF && p.tok != _Semi && p.tok != _Rparen {
		d.Type = p.typeOrNil()
		if p.gotAssign() {
			d.Values = p.exprList()
		}
	}

	return d
}

// TypeSpec = identifier [ TypeParams ] [ "=" ] Type .
func (p *parser) typeDecl(group *Group) Decl {
	if trace {
		defer p.trace("typeDecl")()
	}

	d := new(TypeDecl)
	d.pos = p.pos()
	d.Group = group
	d.Pragma = p.takePragma()

	d.Name = p.name()
	if p.tok == _Lbrack {
		// d.Name "[" ...
		// array/slice type or type parameter list
		pos := p.pos()
		p.next()
		switch p.tok {
		case _Name:
			// We may have an array type or a type parameter list.
			// In either case we expect an expression x (which may
			// just be a name, or a more complex expression) which
			// we can analyze further.
			//
			// A type parameter list may have a type bound starting
			// with a "[" as in: P []E. In that case, simply parsing
			// an expression would lead to an error: P[] is invalid.
			// But since index or slice expressions are never constant
			// and thus invalid array length expressions, if the name
			// is followed by "[" it must be the start of an array or
			// slice constraint. Only if we don't see a "[" do we
			// need to parse a full expression. Notably, name <- x
			// is not a concern because name <- x is a statement and
			// not an expression.
			var x Expr = p.name()
			if p.tok != _Lbrack {
				// To parse the expression starting with name, expand
				// the call sequence we would get by passing in name
				// to parser.expr, and pass in name to parser.pexpr.
				p.xnest++
				x = p.binaryExpr(p.pexpr(x, false), 0)
				p.xnest--
			}
			// Analyze expression x. If we can split x into a type parameter
			// name, possibly followed by a type parameter type, we consider
			// this the start of a type parameter list, with some caveats:
			// a single name followed by "]" tilts the decision towards an
			// array declaration; a type parameter type that could also be
			// an ordinary expression but which is followed by a comma tilts
			// the decision towards a type parameter list.
			if pname, ptype := extractName(x, p.tok == _Comma); pname != nil && (ptype != nil || p.tok != _Rbrack) {
				// d.Name "[" pname ...
				// d.Name "[" pname ptype ...
				// d.Name "[" pname ptype "," ...
				d.TParamList = p.paramList(pname, ptype, _Rbrack, true, false) // ptype may be nil
				d.Alias = p.gotAssign()
				d.Type = p.typeOrNil()
			} else {
				// d.Name "[" pname "]" ...
				// d.Name "[" x ...
				d.Type = p.arrayType(pos, x)
			}
		case _Rbrack:
			// d.Name "[" "]" ...
			p.next()
			d.Type = p.sliceType(pos)
		default:
			// d.Name "[" ...
			d.Type = p.arrayType(pos, nil)
		}
	} else {
		d.Alias = p.gotAssign()
		d.Type = p.typeOrNil()
	}

	if d.Type == nil {
		d.Type = p.badExpr()
		p.syntaxError("in type declaration")
		p.advance(_Semi, _Rparen)
	}

	return d
}

// enumDecl parses "enum Name { NameList }" and returns equivalent type and const declarations.
// enum Status { OK, BAD }
// as example becomes emitted as AST Nodes:
// type Status int
// const ( OK Status = 0; BAD Status = 1 )
// func (s Status) String() string { return [...]string{"OK", "BAD"}[s] }
func (p *parser) enumDecl() []Decl {
	if trace {
		defer p.trace("enumDecl")()
	}

	pos := p.pos()
	enumName := p.name()

	p.want(_Lbrace)

	if p.tok != _Name {
		p.syntaxError("expected enum value name")
		return nil
	}

	enumValues := p.nameList(p.name())

	p.want(_Rbrace)

	if len(enumValues) == 0 {
		p.syntaxErrorAt(pos, "enum must have at least one value")
		return nil
	}

	// Create type declaration: type EnumName int
	typeDecl := &TypeDecl{
		Name: enumName,
		Type: &Name{Value: "int"},
	}
	typeDecl.SetPos(pos)

	// Note: EnumTypes registration happens later in typecheck phase

	// Do iota counting ourselves! Generate: OK=0, BAD=1, etc.
	var constDecls []Decl
	for i, enumValue := range enumValues {
		constDecl := &ConstDecl{
			NameList: []*Name{enumValue},
			Type:     enumName,
			Values:   &BasicLit{Value: fmt.Sprintf("%d", i), Kind: IntLit},
		}
		constDecl.SetPos(enumValue.Pos())
		constDecls = append(constDecls, constDecl)
	}

	// Generate name map: var EnumName_names = map[EnumName]string{0:"OK", 1:"BAD"}
	// nameMapDecl := p.createEnumNameMap(pos, enumName, enumValues)

	// Generate String() method: func (s EnumName) String() string { return [...]string{"OK", "BAD"}[s] }
	stringMethodDecl := p.createEnumStringMethod(pos, enumName, enumValues)

	result := []Decl{typeDecl}
	result = append(result, constDecls...)
	// result = append(result, nameMapDecl)
	result = append(result, stringMethodDecl)
	return result
}

// enumDeclGroup handles enum declarations in file context using appendGroup pattern
func (p *parser) enumDeclGroup(list []Decl) []Decl {
	if trace {
		defer p.trace("enumDeclGroup")()
	}

	pos := p.pos()
	enumName := p.name()

	p.want(_Lbrace)

	if p.tok != _Name {
		p.syntaxError("expected enum value name")
		return list
	}

	enumValues := p.nameList(p.name())
	p.want(_Rbrace)

	if len(enumValues) == 0 {
		p.syntaxErrorAt(pos, "enum must have at least one value")
		return list
	}

	// Create type declaration: type EnumName int
	typeDecl := &TypeDecl{
		Name: enumName,
		Type: &Name{Value: "int"},
	}
	typeDecl.SetPos(pos)
	list = append(list, typeDecl)

	// Note: EnumTypes registration happens later in typecheck phase

	// Do iota counting ourselves! Generate: OK=0, BAD=1, etc.
	// This avoids the complexity of Go's const inheritance
	for i, enumValue := range enumValues {
		constDecl := &ConstDecl{
			NameList: []*Name{enumValue},
			Type:     enumName,
			Values:   &BasicLit{Value: fmt.Sprintf("%d", i), Kind: IntLit},
		}
		constDecl.SetPos(enumValue.Pos())
		list = append(list, constDecl)
	}

	// Generate name map: var EnumName_names = map[EnumName]string{0:"OK", 1:"BAD"}
	// nameMapDecl := p.createEnumNameMap(pos, enumName, enumValues)
	// list = append(list, nameMapDecl)

	// Generate String() method: func (s EnumName) String() string { return [...]string{"OK", "BAD"}[s] }
	stringMethodDecl := p.createEnumStringMethod(pos, enumName, enumValues)
	list = append(list, stringMethodDecl)

	return list
}

// enumStmt handles enum declarations in statement context
func (p *parser) enumStmt() *DeclStmt {
	if trace {
		defer p.trace("enumStmt")()
	}

	s := new(DeclStmt)
	s.pos = p.pos()

	p.next() // consume _Enum
	s.DeclList = p.enumDeclGroup(nil)

	return s
}

// createEnumNameMap creates var EnumName_names = map[EnumName]string{0:"OK", 1:"BAD"}
func (p *parser) createEnumNameMap(pos Pos, enumName *Name, enumValues []*Name) *VarDecl {
	// Create map variable name: EnumName_names
	mapVarName := &Name{Value: enumName.Value + "_names"}
	mapVarName.SetPos(pos)

	// Create map type: map[EnumName]string
	mapType := &MapType{
		Key:   enumName,               // key type
		Value: &Name{Value: "string"}, // value type
	}
	mapType.SetPos(pos)

	// Create map literal elements: {0: "OK", 1: "BAD"}
	var elements []Expr
	for i, enumValue := range enumValues {
		keyLit := &BasicLit{Value: fmt.Sprintf("%d", i), Kind: IntLit}
		keyLit.SetPos(enumValue.Pos())

		valueLit := &BasicLit{Value: fmt.Sprintf(`"%s"`, enumValue.Value), Kind: StringLit}
		valueLit.SetPos(enumValue.Pos())

		keyValue := &KeyValueExpr{
			Key:   keyLit,
			Value: valueLit,
		}
		keyValue.SetPos(enumValue.Pos())
		elements = append(elements, keyValue)
	}

	// Create composite literal: map[EnumName]string{...}
	mapLit := &CompositeLit{
		Type:     mapType,
		ElemList: elements,
	}
	mapLit.SetPos(pos)

	// Create var declaration
	varDecl := &VarDecl{
		NameList: []*Name{mapVarName},
		Type:     mapType,
		Values:   mapLit,
	}
	varDecl.SetPos(pos)

	return varDecl
}

// createEnumStringMethod creates func (s EnumName) String() string { return [...]string{"OK", "BAD"}[s] }
func (p *parser) createEnumStringMethod(pos Pos, enumName *Name, enumValues []*Name) *FuncDecl {
	receiverName := &Name{Value: "s"}
	receiverName.SetPos(pos)
	receiverField := &Field{Name: receiverName, Type: enumName}
	receiverField.SetPos(pos)

	methodName := &Name{Value: "String"}
	methodName.SetPos(pos)

	returnField := &Field{Type: &Name{Value: "string"}}
	returnField.Type.SetPos(pos)
	returnField.SetPos(pos)

	funcType := &FuncType{ResultList: []*Field{returnField}}
	funcType.SetPos(pos)

	var arrayElements []Expr
	for _, enumValue := range enumValues {
		stringLit := &BasicLit{
			Value: fmt.Sprintf(`"%s"`, enumValue.Value),
			Kind:  StringLit,
		}
		stringLit.SetPos(enumValue.Pos())
		arrayElements = append(arrayElements, stringLit)
	}

	arrayType := &ArrayType{
		Len:  nil, // nil means [...] in Go syntax
		Elem: &Name{Value: "string"},
	}
	arrayType.Elem.SetPos(pos)
	arrayType.SetPos(pos)

	arrayLit := &CompositeLit{
		Type:     arrayType,
		ElemList: arrayElements,
	}
	arrayLit.SetPos(pos)

	indexExpr := &IndexExpr{
		X:     arrayLit,
		Index: &Name{Value: "s"},
	}
	indexExpr.Index.SetPos(pos)
	indexExpr.SetPos(pos)

	returnStmt := &ReturnStmt{Results: indexExpr}
	returnStmt.SetPos(pos)

	body := &BlockStmt{List: []Stmt{returnStmt}}
	body.SetPos(pos)

	funcDecl := &FuncDecl{
		Recv: receiverField,
		Name: methodName,
		Type: funcType,
		Body: body,
	}
	funcDecl.SetPos(pos)
	return funcDecl
}

// extractName splits the expression x into (name, expr) if syntactically
// x can be written as name expr. The split only happens if expr is a type
// element (per the isTypeElem predicate) or if force is set.
// If x is just a name, the result is (name, nil). If the split succeeds,
// the result is (name, expr). Otherwise the result is (nil, x).
// Examples:
//
//	x           force    name    expr
//	------------------------------------
//	P*[]int     T/F      P       *[]int
//	P*E         T        P       *E
//	P*E         F        nil     P*E
//	P([]int)    T/F      P       []int
//	P(E)        T        P       E
//	P(E)        F        nil     P(E)
//	P*E|F|~G    T/F      P       *E|F|~G
//	P*E|F|G     T        P       *E|F|G
//	P*E|F|G     F        nil     P*E|F|G
func extractName(x Expr, force bool) (*Name, Expr) {
	switch x := x.(type) {
	case *Name:
		return x, nil
	case *Operation:
		if x.Y == nil {
			break // unary expr
		}
		switch x.Op {
		case Mul:
			if name, _ := x.X.(*Name); name != nil && (force || isTypeElem(x.Y)) {
				// x = name *x.Y
				op := *x
				op.X, op.Y = op.Y, nil // change op into unary *op.Y
				return name, &op
			}
		case Or:
			if name, lhs := extractName(x.X, force || isTypeElem(x.Y)); name != nil && lhs != nil {
				// x = name lhs|x.Y
				op := *x
				op.X = lhs
				return name, &op
			}
		}
	case *CallExpr:
		if name, _ := x.Fun.(*Name); name != nil {
			if len(x.ArgList) == 1 && !x.HasDots && (force || isTypeElem(x.ArgList[0])) {
				// The parser doesn't keep unnecessary parentheses.
				// Set the flag below to keep them, for testing
				// (see go.dev/issues/69206).
				const keep_parens = false
				if keep_parens {
					// x = name (x.ArgList[0])
					px := new(ParenExpr)
					px.pos = x.pos // position of "(" in call
					px.X = x.ArgList[0]
					return name, px
				} else {
					// x = name x.ArgList[0]
					return name, Unparen(x.ArgList[0])
				}
			}
		}
	}
	return nil, x
}

// isTypeElem reports whether x is a (possibly parenthesized) type element expression.
// The result is false if x could be a type element OR an ordinary (value) expression.
func isTypeElem(x Expr) bool {
	switch x := x.(type) {
	case *ArrayType, *StructType, *FuncType, *InterfaceType, *SliceType, *MapType, *ChanType:
		return true
	case *Operation:
		return isTypeElem(x.X) || (x.Y != nil && isTypeElem(x.Y)) || x.Op == Tilde
	case *ParenExpr:
		return isTypeElem(x.X)
	}
	return false
}

// VarSpec = IdentifierList ( Type [ "=" ExpressionList ] | "=" ExpressionList ) .
func (p *parser) varDecl(group *Group) Decl {
	if trace {
		defer p.trace("varDecl")()
	}

	d := new(VarDecl)
	d.pos = p.pos()
	d.Group = group
	d.Pragma = p.takePragma()

	d.NameList = p.nameList(p.name())
	if p.gotAssign() {
		d.Values = p.exprList()
	} else {
		d.Type = p.type_()
		if p.gotAssign() {
			d.Values = p.exprList()
		}
	}

	return d
}

// FunctionDecl = "func" FunctionName [ TypeParams ] ( Function | Signature ) .
// FunctionName = identifier .
// Function     = Signature FunctionBody .
// MethodDecl   = "func" Receiver MethodName ( Function | Signature ) .
// Receiver     = Parameters .
func (p *parser) funcDeclOrNil() *FuncDecl {
	if trace {
		defer p.trace("funcDecl")()
	}

	f := new(FuncDecl)
	f.pos = p.pos()
	f.Pragma = p.takePragma()

	var context string
	if p.got(_Lparen) {
		context = "method"
		rcvr := p.paramList(nil, nil, _Rparen, false, false)
		switch len(rcvr) {
		case 0:
			p.error("method has no receiver")
		default:
			p.error("method has multiple receivers")
			fallthrough
		case 1:
			f.Recv = rcvr[0]
		}
	}

	if p.tok == _Name {
		f.Name = p.name()
		f.TParamList, f.Type = p.funcType(context)
	} else {
		f.Name = NewName(p.pos(), "_")
		f.Type = new(FuncType)
		f.Type.pos = p.pos()
		msg := "expected name or ("
		if context != "" {
			msg = "expected name"
		}
		p.syntaxError(msg)
		p.advance(_Lbrace, _Semi)
	}

	if p.tok == _Lbrace {
		f.Body = p.funcBody()
	}

	return f
}

func (p *parser) funcBody() *BlockStmt {
	p.fnest++
	errcnt := p.errcnt
	body := p.blockStmt("")
	p.fnest--

	// Don't check branches if there were syntax errors in the function
	// as it may lead to spurious errors (e.g., see test/switch2.go) or
	// possibly crashes due to incomplete syntax trees.
	if p.mode&CheckBranches != 0 && errcnt == p.errcnt {
		checkBranches(body, p.errh)
	}

	return body
}

// ----------------------------------------------------------------------------
// Expressions

func (p *parser) expr() Expr {
	if trace {
		defer p.trace("expr")()
	}

	return p.binaryExpr(nil, 0)
}

// Expression = UnaryExpr | Expression binary_op Expression .
func (p *parser) binaryExpr(x Expr, prec int) Expr {
	// don't trace binaryExpr - only leads to overly nested trace output

	if x == nil {
		x = p.unaryExpr()
	}
	for (p.tok == _Operator || p.tok == _Star) && p.prec > prec {
		t := new(Operation)
		t.pos = p.pos()
		t.Op = p.op
		tprec := p.prec
		p.next()
		t.X = x
		t.Y = p.binaryExpr(nil, tprec)
		x = t
	}
	return x
}

// UnaryExpr = PrimaryExpr | unary_op UnaryExpr .
func (p *parser) unaryExpr() Expr {
	if trace {
		defer p.trace("unaryExpr")()
	}

	switch p.tok {
	case _Operator, _Star:
		switch p.op {
		case Mul, Add, Sub, Not, Xor, Tilde:
			x := new(Operation)
			x.pos = p.pos()
			x.Op = p.op
			p.next()
			x.X = p.unaryExpr()
			return x

		case And:
			x := new(Operation)
			x.pos = p.pos()
			x.Op = And
			p.next()
			// unaryExpr may have returned a parenthesized composite literal
			// (see comment in operand) - remove parentheses if any
			x.X = Unparen(p.unaryExpr())
			return x
		}

	case _Arrow:
		// receive op (<-x) or receive-only channel (<-chan E)
		pos := p.pos()
		p.next()

		// If the next token is _Chan we still don't know if it is
		// a channel (<-chan int) or a receive op (<-chan int(ch)).
		// We only know once we have found the end of the unaryExpr.

		x := p.unaryExpr()

		// There are two cases:
		//
		//   <-chan...  => <-x is a channel type
		//   <-x        => <-x is a receive operation
		//
		// In the first case, <- must be re-associated with
		// the channel type parsed already:
		//
		//   <-(chan E)   =>  (<-chan E)
		//   <-(chan<-E)  =>  (<-chan (<-E))

		if _, ok := x.(*ChanType); ok {
			// x is a channel type => re-associate <-
			dir := SendOnly
			t := x
			for dir == SendOnly {
				c, ok := t.(*ChanType)
				if !ok {
					break
				}
				dir = c.Dir
				if dir == RecvOnly {
					// t is type <-chan E but <-<-chan E is not permitted
					// (report same error as for "type _ <-<-chan E")
					p.syntaxError("unexpected <-, expected chan")
					// already progressed, no need to advance
				}
				c.Dir = RecvOnly
				t = c.Elem
			}
			if dir == SendOnly {
				// channel dir is <- but channel element E is not a channel
				// (report same error as for "type _ <-chan<-E")
				p.syntaxError(fmt.Sprintf("unexpected %s, expected chan", String(t)))
				// already progressed, no need to advance
			}
			return x
		}

		// x is not a channel type => we have a receive op
		o := new(Operation)
		o.pos = pos
		o.Op = Recv
		o.X = x
		return o
	}

	// TODO(mdempsky): We need parens here so we can report an
	// error for "(x) := true". It should be possible to detect
	// and reject that more efficiently though.
	return p.pexpr(nil, true)
}

// callStmt parses call-like statements that can be preceded by 'defer' and 'go'.
func (p *parser) callStmt() *CallStmt {
	if trace {
		defer p.trace("callStmt")()
	}

	s := new(CallStmt)
	s.pos = p.pos()
	s.Tok = p.tok // _Defer or _Go
	p.next()

	x := p.pexpr(nil, p.tok == _Lparen) // keep_parens so we can report error below
	if t := Unparen(x); t != x {
		p.errorAt(x.Pos(), fmt.Sprintf("expression in %s must not be parenthesized", s.Tok))
		// already progressed, no need to advance
		x = t
	}

	s.Call = x
	return s
}

// Operand     = Literal | OperandName | MethodExpr | "(" Expression ")" .
// Literal     = BasicLit | CompositeLit | FunctionLit .
// BasicLit    = int_lit | float_lit | imaginary_lit | rune_lit | string_lit .
// OperandName = identifier | QualifiedIdent.
func (p *parser) operand(keep_parens bool) Expr {
	if trace {
		defer p.trace("operand " + p.tok.String())()
	}

	switch p.tok {
	case _Name:
		return p.name()

	case _Literal:
		return p.oliteral()

	case _Lparen:
		pos := p.pos()
		p.next()
		p.xnest++
		x := p.expr()
		p.xnest--
		p.want(_Rparen)

		// Optimization: Record presence of ()'s only where needed
		// for error reporting. Don't bother in other cases; it is
		// just a waste of memory and time.
		//
		// Parentheses are not permitted around T in a composite
		// literal T{}. If the next token is a {, assume x is a
		// composite literal type T (it may not be, { could be
		// the opening brace of a block, but we don't know yet).
		if p.tok == _Lbrace {
			keep_parens = true
		}

		// Parentheses are also not permitted around the expression
		// in a go/defer statement. In that case, operand is called
		// with keep_parens set.
		if keep_parens {
			px := new(ParenExpr)
			px.pos = pos
			px.X = x
			x = px
		}
		return x

	case _Func:
		pos := p.pos()
		p.next()
		_, ftyp := p.funcType("function type")
		if p.tok == _Lbrace {
			p.xnest++

			f := new(FuncLit)
			f.pos = pos
			f.Type = ftyp
			f.Body = p.funcBody()

			p.xnest--
			return f
		}
		return ftyp

	case _Map:
		// Parse map type - could be map[K]V or map{...}
		return p.mapTypeOrLiteral()

	case _Lbrack:
		// Handle both [1,2,3] slice literals and [1]Type array types
		pos := p.pos()
		p.want(_Lbrack)

		// Empty brackets [] - this should be followed by type for slice type
		if p.tok == _Rbrack {
			p.next()
			t := new(SliceType)
			t.pos = pos
			t.Elem = p.type_()
			return t
		}

		// Check for ellipsis [...] arrays first
		if p.got(_DotDotDot) {
			p.want(_Rbrack)
			t := new(ArrayType)
			t.pos = pos
			t.Len = nil // nil means ellipsis
			t.Elem = p.type_()
			return t
		}

		// Parse first expression
		first := p.expr()

		// Disambiguate: if comma follows, it's slice literal [1,2,3]
		if p.tok == _Comma {
			return p.sliceLiteral(pos, first)
		}

		// Otherwise it's array type [expr]Type
		p.want(_Rbrack)
		t := new(ArrayType)
		t.pos = pos
		t.Len = first
		t.Elem = p.type_()
		return t

	case _Lbrace:
		// Handle {a: 1, b: 2} map literals with symbol keys
		return p.mapLiteralFromBrace()

	case _Chan, _Struct, _Interface:
		return p.type_() // othertype

	default:
		x := p.badExpr()
		p.syntaxError("expected expression")
		p.advance(_Rparen, _Rbrack, _Rbrace)
		return x
	}

	// Syntactically, composite literals are operands. Because a complit
	// type may be a qualified identifier which is handled by pexpr
	// (together with selector expressions), complits are parsed there
	// as well (operand is only called from pexpr).
}

// pexpr parses a PrimaryExpr.
//
//	PrimaryExpr =
//		Operand |
//		Conversion |
//		PrimaryExpr Selector |
//		PrimaryExpr Index |
//		PrimaryExpr Slice |
//		PrimaryExpr TypeAssertion |
//		PrimaryExpr Arguments .
//
//	Selector       = "." identifier .
//	Index          = "[" Expression "]" .
//	Slice          = "[" ( [ Expression ] ":" [ Expression ] ) |
//	                     ( [ Expression ] ":" Expression ":" Expression )
//	                 "]" .
//	TypeAssertion  = "." "(" Type ")" .
//	Arguments      = "(" [ ( ExpressionList | Type [ "," ExpressionList ] ) [ "..." ] [ "," ] ] ")" .
func (p *parser) pexpr(x Expr, keep_parens bool) Expr {
	if trace {
		defer p.trace("pexpr")()
	}

	if x == nil {
		x = p.operand(keep_parens)
	}

loop:
	for {
		pos := p.pos()
		switch p.tok {
		case _Dot:
			p.next()
			switch p.tok {
			case _Name:
				// pexpr '.' sym
				t := new(SelectorExpr)
				t.pos = pos
				t.X = x
				t.Sel = p.name()
				x = t

			case _Lparen:
				p.next()
				if p.got(_Type) {
					t := new(TypeSwitchGuard)
					// t.Lhs is filled in by parser.simpleStmt
					t.pos = pos
					t.X = x
					x = t
				} else {
					t := new(AssertExpr)
					t.pos = pos
					t.X = x
					t.Type = p.type_()
					x = t
				}
				p.want(_Rparen)

			default:
				p.syntaxError("expected name or (")
				p.advance(_Semi, _Rparen)
			}

		case _Lbrack:
			p.next()

			var i Expr
			if p.tok != _Colon {
				var comma bool
				if p.tok == _Rbrack {
					// invalid empty instance, slice or index expression; accept but complain
					p.syntaxError("expected operand")
					i = p.badExpr()
				} else {
					i, comma = p.typeList(false)
				}
				if comma || p.tok == _Rbrack {
					p.want(_Rbrack)
					// x[], x[i,] or x[i, j, ...]
					t := new(IndexExpr)
					t.pos = pos
					t.X = x
					t.Index = i
					x = t
					break
				}
			}

			// x[i:...
			// For better error message, don't simply use p.want(_Colon) here (go.dev/issue/47704).
			if !p.got(_Colon) {
				p.syntaxError("expected comma, : or ]")
				p.advance(_Comma, _Colon, _Rbrack)
			}
			p.xnest++
			t := new(SliceExpr)
			t.pos = pos
			t.X = x
			t.Index[0] = i
			if p.tok != _Colon && p.tok != _Rbrack {
				// x[i:j...
				t.Index[1] = p.expr()
			}
			if p.tok == _Colon {
				t.Full = true
				// x[i:j:...]
				if t.Index[1] == nil {
					p.error("middle index required in 3-index slice")
					t.Index[1] = p.badExpr()
				}
				p.next()
				if p.tok != _Rbrack {
					// x[i:j:k...
					t.Index[2] = p.expr()
				} else {
					p.error("final index required in 3-index slice")
					t.Index[2] = p.badExpr()
				}
			}
			p.xnest--
			p.want(_Rbrack)
			x = t

		case _Hash:
			// 1-indexed array access: x#i becomes x[i-1]
			p.next()
			i := p.unaryExpr()
			// Create IndexExpr with subtraction to convert 1-indexed to 0-indexed
			t := new(IndexExpr)
			t.pos = pos
			t.X = x
			// Create literal 1
			one := new(BasicLit)
			one.pos = pos
			one.Value = "1"
			one.Kind = IntLit
			// Create subtraction: i - 1
			sub := new(Operation)
			sub.pos = pos
			sub.Op = Sub
			sub.X = i
			sub.Y = one
			t.Index = sub
			x = t

		case _Lparen:
			t := new(CallExpr)
			t.pos = pos
			p.next()
			t.Fun = x
			t.ArgList, t.HasDots = p.argList()
			x = t

		case _Lbrace:
			// operand may have returned a parenthesized complit
			// type; accept it but complain if we have a complit
			t := Unparen(x)
			// determine if '{' belongs to a composite literal or a block statement
			complit_ok := false
			switch t.(type) {
			case *Name, *SelectorExpr:
				if p.xnest >= 0 {
					// x is possibly a composite literal type
					complit_ok = true
				}
			case *IndexExpr:
				if p.xnest >= 0 && !isValue(t) {
					// x is possibly a composite literal type
					complit_ok = true
				}
			case *ArrayType, *SliceType, *StructType, *MapType:
				// x is a comptype
				complit_ok = true
			}
			if !complit_ok {
				break loop
			}
			if t != x {
				p.syntaxError("cannot parenthesize type in composite literal")
				// already progressed, no need to advance
			}
			n := p.complitexpr()
			n.Type = x
			x = n

		default:
			break loop
		}
	}

	return x
}

// isValue reports whether x syntactically must be a value (and not a type) expression.
func isValue(x Expr) bool {
	switch x := x.(type) {
	case *BasicLit, *CompositeLit, *FuncLit, *SliceExpr, *AssertExpr, *TypeSwitchGuard, *CallExpr:
		return true
	case *Operation:
		return x.Op != Mul || x.Y != nil // *T may be a type
	case *ParenExpr:
		return isValue(x.X)
	case *IndexExpr:
		return isValue(x.X) || isValue(x.Index)
	}
	return false
}

// Element = Expression | LiteralValue .
func (p *parser) bare_complitexpr() Expr {
	if trace {
		defer p.trace("bare_complitexpr")()
	}

	if p.tok == _Lbrace {
		// '{' start_complit braced_keyval_list '}'
		return p.complitexpr()
	}

	return p.expr()
}

// LiteralValue = "{" [ ElementList [ "," ] ] "}" .
func (p *parser) complitexpr() *CompositeLit {
	if trace {
		defer p.trace("complitexpr")()
	}

	x := new(CompositeLit)
	x.pos = p.pos()

	p.xnest++
	p.want(_Lbrace)
	x.Rbrace = p.list("composite literal", _Comma, _Rbrace, func() bool {
		// value
		e := p.bare_complitexpr()
		if p.tok == _Colon {
			// key ':' value
			l := new(KeyValueExpr)
			l.pos = p.pos()
			p.next()
			l.Key = e
			l.Value = p.bare_complitexpr()
			e = l
			x.NKeys++
		}
		x.ElemList = append(x.ElemList, e)
		return false
	})
	p.xnest--

	return x
}

// ----------------------------------------------------------------------------
// Types

func (p *parser) type_() Expr {
	if trace {
		defer p.trace("type_")()
	}

	typ := p.typeOrNil()
	if typ == nil {
		typ = p.badExpr()
		p.syntaxError("expected type")
		p.advance(_Comma, _Colon, _Semi, _Rparen, _Rbrack, _Rbrace)
	}

	return typ
}

func newIndirect(pos Pos, typ Expr) Expr {
	o := new(Operation)
	o.pos = pos
	o.Op = Mul
	o.X = typ
	return o
}

// typeOrNil is like type_ but it returns nil if there was no type
// instead of reporting an error.
//
//	Type     = TypeName | TypeLit | "(" Type ")" .
//	TypeName = identifier | QualifiedIdent .
//	TypeLit  = ArrayType | StructType | PointerType | FunctionType | InterfaceType |
//		      SliceType | MapType | Channel_Type .
func (p *parser) typeOrNil() Expr {
	if trace {
		defer p.trace("typeOrNil")()
	}

	pos := p.pos()
	switch p.tok {
	case _Star:
		// ptrtype
		p.next()
		return newIndirect(pos, p.type_())

	case _Arrow:
		// recvchantype
		p.next()
		p.want(_Chan)
		t := new(ChanType)
		t.pos = pos
		t.Dir = RecvOnly
		t.Elem = p.chanElem()
		return t

	case _Func:
		// fntype
		p.next()
		_, t := p.funcType("function type")
		return t

	case _Lbrack:
		// '[' oexpr ']' ntype
		// '[' _DotDotDot ']' ntype
		p.next()
		if p.got(_Rbrack) {
			return p.sliceType(pos)
		}
		return p.arrayType(pos, nil)

	case _Chan:
		// _Chan non_recvchantype
		// _Chan _Comm ntype
		p.next()
		t := new(ChanType)
		t.pos = pos
		if p.got(_Arrow) {
			t.Dir = SendOnly
		}
		t.Elem = p.chanElem()
		return t

	case _Map:
		// _Map '[' ntype ']' ntype
		p.next()
		p.want(_Lbrack)
		t := new(MapType)
		t.pos = pos
		t.Key = p.type_()
		p.want(_Rbrack)
		t.Value = p.type_()
		return t

	case _Struct:
		return p.structType()

	case _Interface:
		return p.interfaceType()

	case _Name:
		return p.qualifiedName(nil)

	case _Lparen:
		p.next()
		t := p.type_()
		p.want(_Rparen)
		// The parser doesn't keep unnecessary parentheses.
		// Set the flag below to keep them, for testing
		// (see e.g. tests for go.dev/issue/68639).
		const keep_parens = false
		if keep_parens {
			px := new(ParenExpr)
			px.pos = pos
			px.X = t
			t = px
		}
		return t
	}

	return nil
}

// mapTypeOrLiteral parses either map[K]V, map{...}, or map[key:value] (auto-detect map literal)
func (p *parser) mapTypeOrLiteral() Expr {
	if trace {
		defer p.trace("mapTypeOrLiteral")()
	}

	pos := p.pos()
	p.next() // consume 'map'

	if p.tok == _Lbrack {
		p.want(_Lbrack)

		// Check for empty map[] case
		if p.tok == _Rbrack {
			// Empty map literal map[]
			return p.mapLiteralFromBracket(pos, nil)
		}

		// Look ahead to determine if this is map[key:value] or map[K]V
		firstExpr := p.expr()

		if p.tok == _Colon {
			// This is map[key:value key:value...] literal syntax
			return p.mapLiteralFromBracket(pos, firstExpr)
		} else {
			// Regular map[K]V syntax - firstExpr is the key type
			t := new(MapType)
			t.pos = pos
			t.Key = firstExpr
			p.want(_Rbrack)
			t.Value = p.type_()
			return t
		}
	} else if p.tok == _Lbrace {
		// map{...} syntax - create MapType with any key/value types (map[any]any)
		t := new(MapType)
		t.pos = pos
		// Use 'any' as placeholder for both key and value types
		anyName := new(Name)
		anyName.pos = pos
		anyName.Value = "any"
		t.Key = anyName
		t.Value = anyName
		return t
	} else {
		p.syntaxError("expected '[' or '{'")
		return p.badExpr()
	}
}

// sliceLiteral parses [1,2,3] style slice literals
func (p *parser) sliceLiteral(pos Pos, first Expr) Expr {
	if trace {
		defer p.trace("sliceLiteral")()
	}

	// Create slice type with inferred element type ([]any)
	sliceType := new(SliceType)
	sliceType.pos = pos
	anyName := new(Name)
	anyName.pos = pos
	anyName.Value = "any"
	sliceType.Elem = anyName

	// Create composite literal
	lit := new(CompositeLit)
	lit.pos = pos
	lit.Type = sliceType

	// Add first element - for slice literals, elements are just expressions, not key-value pairs
	lit.ElemList = append(lit.ElemList, first)

	// Parse remaining elements
	for p.tok == _Comma {
		p.next()
		if p.tok == _Rbrack {
			break // trailing comma
		}

		expr := p.expr()
		lit.ElemList = append(lit.ElemList, expr)
	}

	lit.Rbrace = p.pos()
	p.want(_Rbrack)
	return lit
}

// convertSymbolKeyToString converts unquoted identifiers to string literals for map keys
func (p *parser) convertSymbolKeyToString(keyExpr Expr) Expr {
	if nameExpr, ok := keyExpr.(*Name); ok {
		// Convert identifier 'a' to string literal "a"
		stringLit := new(BasicLit)
		stringLit.pos = nameExpr.pos
		stringLit.Value = `"` + nameExpr.Value + `"`
		stringLit.Kind = StringLit
		return stringLit
	}
	return keyExpr
}

// mapLiteralFromBracket parses map[key:value key:value...] style map literals
func (p *parser) mapLiteralFromBracket(pos Pos, firstKey Expr) Expr {
	if trace {
		defer p.trace("mapLiteralFromBracket")()
	}

	// Create map type with any key/value types (map[any]any)
	mapType := new(MapType)
	mapType.pos = pos
	anyName := new(Name)
	anyName.pos = pos
	anyName.Value = "any"
	mapType.Key = anyName
	mapType.Value = anyName

	// Create composite literal
	lit := new(CompositeLit)
	lit.pos = pos
	lit.Type = mapType

	// Process first key:value pair if not empty
	if firstKey != nil {
		keyExpr := p.convertSymbolKeyToString(firstKey)
		p.want(_Colon)
		valueExpr := p.expr()

		kvExpr := new(KeyValueExpr)
		kvExpr.pos = keyExpr.Pos()
		kvExpr.Key = keyExpr
		kvExpr.Value = valueExpr
		lit.ElemList = append(lit.ElemList, kvExpr)
		lit.NKeys++
	}

	// Parse remaining elements
	for p.tok != _Rbrack && p.tok != _EOF {
		// For non-empty maps, optionally consume comma
		if firstKey != nil || len(lit.ElemList) > 0 {
			p.got(_Comma) // Optional comma - don't break if missing
		}
		if p.tok == _Rbrack {
			break // closing bracket or empty map
		}

		keyExpr := p.expr()
		keyExpr = p.convertSymbolKeyToString(keyExpr)

		p.want(_Colon)
		valueExpr := p.expr()

		kvExpr := new(KeyValueExpr)
		kvExpr.pos = keyExpr.Pos()
		kvExpr.Key = keyExpr
		kvExpr.Value = valueExpr
		lit.ElemList = append(lit.ElemList, kvExpr)
		lit.NKeys++
	}

	lit.Rbrace = p.pos()
	p.want(_Rbrack)
	return lit
}

// mapLiteralFromBrace parses {a: 1, b: 2} style map literals with symbol key conversion
func (p *parser) mapLiteralFromBrace() Expr {
	if trace {
		defer p.trace("mapLiteralFromBrace")()
	}

	pos := p.pos()
	p.want(_Lbrace)

	// Create map type with any key/value types (map[any]any)
	mapType := new(MapType)
	mapType.pos = pos
	anyName := new(Name)
	anyName.pos = pos
	anyName.Value = "any"
	mapType.Key = anyName
	mapType.Value = anyName

	// Create composite literal
	lit := new(CompositeLit)
	lit.pos = pos
	lit.Type = mapType

	// Parse elements
	for p.tok != _Rbrace && p.tok != _EOF {
		keyExpr := p.expr()
		keyExpr = p.convertSymbolKeyToString(keyExpr)

		p.want(_Colon)
		valueExpr := p.expr()

		// Create key-value pair
		kvExpr := new(KeyValueExpr)
		kvExpr.pos = keyExpr.Pos()
		kvExpr.Key = keyExpr
		kvExpr.Value = valueExpr
		lit.ElemList = append(lit.ElemList, kvExpr)
		lit.NKeys++

		if !p.got(_Comma) {
			break
		}
	}

	lit.Rbrace = p.pos()
	p.want(_Rbrace)
	return lit
}

func (p *parser) typeInstance(typ Expr) Expr {
	if trace {
		defer p.trace("typeInstance")()
	}

	pos := p.pos()
	p.want(_Lbrack)
	x := new(IndexExpr)
	x.pos = pos
	x.X = typ
	if p.tok == _Rbrack {
		p.syntaxError("expected type argument list")
		x.Index = p.badExpr()
	} else {
		x.Index, _ = p.typeList(true)
	}
	p.want(_Rbrack)
	return x
}

// If context != "", type parameters are not permitted.
func (p *parser) funcType(context string) ([]*Field, *FuncType) {
	if trace {
		defer p.trace("funcType")()
	}

	typ := new(FuncType)
	typ.pos = p.pos()

	var tparamList []*Field
	if p.got(_Lbrack) {
		if context != "" {
			// accept but complain
			p.syntaxErrorAt(typ.pos, context+" must have no type parameters")
		}
		if p.tok == _Rbrack {
			p.syntaxError("empty type parameter list")
			p.next()
		} else {
			tparamList = p.paramList(nil, nil, _Rbrack, true, false)
		}
	}

	p.want(_Lparen)
	typ.ParamList = p.paramList(nil, nil, _Rparen, false, true)
	typ.ResultList = p.funcResult()

	return tparamList, typ
}

// "[" has already been consumed, and pos is its position.
// If len != nil it is the already consumed array length.
func (p *parser) arrayType(pos Pos, len Expr) Expr {
	if trace {
		defer p.trace("arrayType")()
	}

	if len == nil && !p.got(_DotDotDot) {
		p.xnest++
		len = p.expr()
		p.xnest--
	}
	if p.tok == _Comma {
		// Trailing commas are accepted in type parameter
		// lists but not in array type declarations.
		// Accept for better error handling but complain.
		p.syntaxError("unexpected comma; expected ]")
		p.next()
	}
	p.want(_Rbrack)
	t := new(ArrayType)
	t.pos = pos
	t.Len = len
	t.Elem = p.type_()
	return t
}

// "[" and "]" have already been consumed, and pos is the position of "[".
func (p *parser) sliceType(pos Pos) Expr {
	t := new(SliceType)
	t.pos = pos
	t.Elem = p.type_()
	return t
}

func (p *parser) chanElem() Expr {
	if trace {
		defer p.trace("chanElem")()
	}

	typ := p.typeOrNil()
	if typ == nil {
		typ = p.badExpr()
		p.syntaxError("missing channel element type")
		// assume element type is simply absent - don't advance
	}

	return typ
}

// StructType = "struct" "{" { FieldDecl ";" } "}" .
func (p *parser) structType() *StructType {
	if trace {
		defer p.trace("structType")()
	}

	typ := new(StructType)
	typ.pos = p.pos()

	p.want(_Struct)
	p.want(_Lbrace)
	p.list("struct type", _Semi, _Rbrace, func() bool {
		p.fieldDecl(typ)
		return false
	})

	return typ
}

// InterfaceType = "interface" "{" { ( MethodDecl | EmbeddedElem ) ";" } "}" .
func (p *parser) interfaceType() *InterfaceType {
	if trace {
		defer p.trace("interfaceType")()
	}

	typ := new(InterfaceType)
	typ.pos = p.pos()

	p.want(_Interface)
	p.want(_Lbrace)
	p.list("interface type", _Semi, _Rbrace, func() bool {
		var f *Field
		if p.tok == _Name {
			f = p.methodDecl()
		}
		if f == nil || f.Name == nil {
			f = p.embeddedElem(f)
		}
		typ.MethodList = append(typ.MethodList, f)
		return false
	})

	return typ
}

// Result = Parameters | Type .
func (p *parser) funcResult() []*Field {
	if trace {
		defer p.trace("funcResult")()
	}

	if p.got(_Lparen) {
		return p.paramList(nil, nil, _Rparen, false, false)
	}

	pos := p.pos()
	if typ := p.typeOrNil(); typ != nil {
		f := new(Field)
		f.pos = pos
		f.Type = typ
		return []*Field{f}
	}

	return nil
}

func (p *parser) addField(styp *StructType, pos Pos, name *Name, typ Expr, tag *BasicLit) {
	if tag != nil {
		for i := len(styp.FieldList) - len(styp.TagList); i > 0; i-- {
			styp.TagList = append(styp.TagList, nil)
		}
		styp.TagList = append(styp.TagList, tag)
	}

	f := new(Field)
	f.pos = pos
	f.Name = name
	f.Type = typ
	styp.FieldList = append(styp.FieldList, f)

	if debug && tag != nil && len(styp.FieldList) != len(styp.TagList) {
		panic("inconsistent struct field list")
	}
}

// FieldDecl      = (IdentifierList Type | AnonymousField) [ Tag ] .
// AnonymousField = [ "*" ] TypeName .
// Tag            = string_lit .
func (p *parser) fieldDecl(styp *StructType) {
	if trace {
		defer p.trace("fieldDecl")()
	}

	pos := p.pos()
	switch p.tok {
	case _Name:
		name := p.name()
		if p.tok == _Dot || p.tok == _Literal || p.tok == _Semi || p.tok == _Rbrace {
			// embedded type
			typ := p.qualifiedName(name)
			tag := p.oliteral()
			p.addField(styp, pos, nil, typ, tag)
			break
		}

		// name1, name2, ... Type [ tag ]
		names := p.nameList(name)
		var typ Expr

		// Careful dance: We don't know if we have an embedded instantiated
		// type T[P1, P2, ...] or a field T of array/slice type [P]E or []E.
		if len(names) == 1 && p.tok == _Lbrack {
			typ = p.arrayOrTArgs()
			if typ, ok := typ.(*IndexExpr); ok {
				// embedded type T[P1, P2, ...]
				typ.X = name // name == names[0]
				tag := p.oliteral()
				p.addField(styp, pos, nil, typ, tag)
				break
			}
		} else {
			// T P
			typ = p.type_()
		}

		tag := p.oliteral()

		for _, name := range names {
			p.addField(styp, name.Pos(), name, typ, tag)
		}

	case _Star:
		p.next()
		var typ Expr
		if p.tok == _Lparen {
			// *(T)
			p.syntaxError("cannot parenthesize embedded type")
			p.next()
			typ = p.qualifiedName(nil)
			p.got(_Rparen) // no need to complain if missing
		} else {
			// *T
			typ = p.qualifiedName(nil)
		}
		tag := p.oliteral()
		p.addField(styp, pos, nil, newIndirect(pos, typ), tag)

	case _Lparen:
		p.syntaxError("cannot parenthesize embedded type")
		p.next()
		var typ Expr
		if p.tok == _Star {
			// (*T)
			pos := p.pos()
			p.next()
			typ = newIndirect(pos, p.qualifiedName(nil))
		} else {
			// (T)
			typ = p.qualifiedName(nil)
		}
		p.got(_Rparen) // no need to complain if missing
		tag := p.oliteral()
		p.addField(styp, pos, nil, typ, tag)

	default:
		p.syntaxError("expected field name or embedded type")
		p.advance(_Semi, _Rbrace)
	}
}

func (p *parser) arrayOrTArgs() Expr {
	if trace {
		defer p.trace("arrayOrTArgs")()
	}

	pos := p.pos()
	p.want(_Lbrack)
	if p.got(_Rbrack) {
		return p.sliceType(pos)
	}

	// x [n]E or x[n,], x[n1, n2], ...
	n, comma := p.typeList(false)
	p.want(_Rbrack)
	if !comma {
		if elem := p.typeOrNil(); elem != nil {
			// x [n]E
			t := new(ArrayType)
			t.pos = pos
			t.Len = n
			t.Elem = elem
			return t
		}
	}

	// x[n,], x[n1, n2], ...
	t := new(IndexExpr)
	t.pos = pos
	// t.X will be filled in by caller
	t.Index = n
	return t
}

func (p *parser) oliteral() *BasicLit {
	if p.tok == _Literal {
		b := new(BasicLit)
		b.pos = p.pos()
		b.Value = p.lit
		b.Kind = p.kind
		b.Bad = p.bad
		p.next()
		return b
	}
	return nil
}

// MethodSpec        = MethodName Signature | InterfaceTypeName .
// MethodName        = identifier .
// InterfaceTypeName = TypeName .
func (p *parser) methodDecl() *Field {
	if trace {
		defer p.trace("methodDecl")()
	}

	f := new(Field)
	f.pos = p.pos()
	name := p.name()

	const context = "interface method"

	switch p.tok {
	case _Lparen:
		// method
		f.Name = name
		_, f.Type = p.funcType(context)

	case _Lbrack:
		// Careful dance: We don't know if we have a generic method m[T C](x T)
		// or an embedded instantiated type T[P1, P2] (we accept generic methods
		// for generality and robustness of parsing but complain with an error).
		pos := p.pos()
		p.next()

		// Empty type parameter or argument lists are not permitted.
		// Treat as if [] were absent.
		if p.tok == _Rbrack {
			// name[]
			pos := p.pos()
			p.next()
			if p.tok == _Lparen {
				// name[](
				p.errorAt(pos, "empty type parameter list")
				f.Name = name
				_, f.Type = p.funcType(context)
			} else {
				p.errorAt(pos, "empty type argument list")
				f.Type = name
			}
			break
		}

		// A type argument list looks like a parameter list with only
		// types. Parse a parameter list and decide afterwards.
		list := p.paramList(nil, nil, _Rbrack, false, false)
		if len(list) == 0 {
			// The type parameter list is not [] but we got nothing
			// due to other errors (reported by paramList). Treat
			// as if [] were absent.
			if p.tok == _Lparen {
				f.Name = name
				_, f.Type = p.funcType(context)
			} else {
				f.Type = name
			}
			break
		}

		// len(list) > 0
		if list[0].Name != nil {
			// generic method
			f.Name = name
			_, f.Type = p.funcType(context)
			p.errorAt(pos, "interface method must have no type parameters")
			break
		}

		// embedded instantiated type
		t := new(IndexExpr)
		t.pos = pos
		t.X = name
		if len(list) == 1 {
			t.Index = list[0].Type
		} else {
			// len(list) > 1
			l := new(ListExpr)
			l.pos = list[0].Pos()
			l.ElemList = make([]Expr, len(list))
			for i := range list {
				l.ElemList[i] = list[i].Type
			}
			t.Index = l
		}
		f.Type = t

	default:
		// embedded type
		f.Type = p.qualifiedName(name)
	}

	return f
}

// EmbeddedElem = MethodSpec | EmbeddedTerm { "|" EmbeddedTerm } .
func (p *parser) embeddedElem(f *Field) *Field {
	if trace {
		defer p.trace("embeddedElem")()
	}

	if f == nil {
		f = new(Field)
		f.pos = p.pos()
		f.Type = p.embeddedTerm()
	}

	for p.tok == _Operator && p.op == Or {
		t := new(Operation)
		t.pos = p.pos()
		t.Op = Or
		p.next()
		t.X = f.Type
		t.Y = p.embeddedTerm()
		f.Type = t
	}

	return f
}

// EmbeddedTerm = [ "~" ] Type .
func (p *parser) embeddedTerm() Expr {
	if trace {
		defer p.trace("embeddedTerm")()
	}

	if p.tok == _Operator && p.op == Tilde {
		t := new(Operation)
		t.pos = p.pos()
		t.Op = Tilde
		p.next()
		t.X = p.type_()
		return t
	}

	t := p.typeOrNil()
	if t == nil {
		t = p.badExpr()
		p.syntaxError("expected ~ term or type")
		p.advance(_Operator, _Semi, _Rparen, _Rbrack, _Rbrace)
	}

	return t
}

// ParameterDecl = [ IdentifierList ] [ "..." ] Type .
func (p *parser) paramDeclOrNil(name *Name, follow token) *Field {
	if trace {
		defer p.trace("paramDeclOrNil")()
	}

	// type set notation is ok in type parameter lists
	typeSetsOk := follow == _Rbrack

	pos := p.pos()
	if name != nil {
		pos = name.pos
	} else if typeSetsOk && p.tok == _Operator && p.op == Tilde {
		// "~" ...
		return p.embeddedElem(nil)
	}

	f := new(Field)
	f.pos = pos

	if p.tok == _Name || name != nil {
		// name
		if name == nil {
			name = p.name()
		}

		if p.tok == _Lbrack {
			// name "[" ...
			f.Type = p.arrayOrTArgs()
			if typ, ok := f.Type.(*IndexExpr); ok {
				// name "[" ... "]"
				typ.X = name
			} else {
				// name "[" n "]" E
				f.Name = name
			}
			if typeSetsOk && p.tok == _Operator && p.op == Or {
				// name "[" ... "]" "|" ...
				// name "[" n "]" E "|" ...
				f = p.embeddedElem(f)
			}
			return f
		}

		if p.tok == _Dot {
			// name "." ...
			f.Type = p.qualifiedName(name)
			if typeSetsOk && p.tok == _Operator && p.op == Or {
				// name "." name "|" ...
				f = p.embeddedElem(f)
			}
			return f
		}

		if typeSetsOk && p.tok == _Operator && p.op == Or {
			// name "|" ...
			f.Type = name
			return p.embeddedElem(f)
		}

		f.Name = name
	}

	if p.tok == _DotDotDot {
		// [name] "..." ...
		t := new(DotsType)
		t.pos = p.pos()
		p.next()
		t.Elem = p.typeOrNil()
		if t.Elem == nil {
			f.Type = p.badExpr()
			p.syntaxError("... is missing type")
		} else {
			f.Type = t
		}
		return f
	}

	if typeSetsOk && p.tok == _Operator && p.op == Tilde {
		// [name] "~" ...
		f.Type = p.embeddedElem(nil).Type
		return f
	}

	f.Type = p.typeOrNil()
	if typeSetsOk && p.tok == _Operator && p.op == Or && f.Type != nil {
		// [name] type "|"
		f = p.embeddedElem(f)
	}
	if f.Name != nil || f.Type != nil {
		return f
	}

	p.syntaxError("expected " + tokstring(follow))
	p.advance(_Comma, follow)
	return nil
}

// Parameters    = "(" [ ParameterList [ "," ] ] ")" .
// ParameterList = ParameterDecl { "," ParameterDecl } .
// "(" or "[" has already been consumed.
// If name != nil, it is the first name after "(" or "[".
// If typ != nil, name must be != nil, and (name, typ) is the first field in the list.
// In the result list, either all fields have a name, or no field has a name.
func (p *parser) paramList(name *Name, typ Expr, close token, requireNames, dddok bool) (list []*Field) {
	if trace {
		defer p.trace("paramList")()
	}

	// p.list won't invoke its function argument if we're at the end of the
	// parameter list. If we have a complete field, handle this case here.
	if name != nil && typ != nil && p.tok == close {
		p.next()
		par := new(Field)
		par.pos = name.pos
		par.Name = name
		par.Type = typ
		return []*Field{par}
	}

	var named int // number of parameters that have an explicit name and type
	var typed int // number of parameters that have an explicit type
	end := p.list("parameter list", _Comma, close, func() bool {
		var par *Field
		if typ != nil {
			if debug && name == nil {
				panic("initial type provided without name")
			}
			par = new(Field)
			par.pos = name.pos
			par.Name = name
			par.Type = typ
		} else {
			par = p.paramDeclOrNil(name, close)
		}
		name = nil // 1st name was consumed if present
		typ = nil  // 1st type was consumed if present
		if par != nil {
			if debug && par.Name == nil && par.Type == nil {
				panic("parameter without name or type")
			}
			if par.Name != nil && par.Type != nil {
				named++
			}
			if par.Type != nil {
				typed++
			}
			list = append(list, par)
		}
		return false
	})

	if len(list) == 0 {
		return
	}

	// distribute parameter types (len(list) > 0)
	if named == 0 && !requireNames {
		// all unnamed and we're not in a type parameter list => found names are named types
		for _, par := range list {
			if typ := par.Name; typ != nil {
				par.Type = typ
				par.Name = nil
			}
		}
	} else if named != len(list) {
		// some named or we're in a type parameter list => all must be named
		var errPos Pos // left-most error position (or unknown)
		var typ Expr   // current type (from right to left)
		for i := len(list) - 1; i >= 0; i-- {
			par := list[i]
			if par.Type != nil {
				typ = par.Type
				if par.Name == nil {
					errPos = StartPos(typ)
					par.Name = NewName(errPos, "_")
				}
			} else if typ != nil {
				par.Type = typ
			} else {
				// par.Type == nil && typ == nil => we only have a par.Name
				errPos = par.Name.Pos()
				t := p.badExpr()
				t.pos = errPos // correct position
				par.Type = t
			}
		}
		if errPos.IsKnown() {
			// Not all parameters are named because named != len(list).
			// If named == typed, there must be parameters that have no types.
			// They must be at the end of the parameter list, otherwise types
			// would have been filled in by the right-to-left sweep above and
			// there would be no error.
			// If requireNames is set, the parameter list is a type parameter
			// list.
			var msg string
			if named == typed {
				errPos = end // position error at closing token ) or ]
				if requireNames {
					msg = "missing type constraint"
				} else {
					msg = "missing parameter type"
				}
			} else {
				if requireNames {
					msg = "missing type parameter name"
					// go.dev/issue/60812
					if len(list) == 1 {
						msg += " or invalid array length"
					}
				} else {
					msg = "missing parameter name"
				}
			}
			p.syntaxErrorAt(errPos, msg)
		}
	}

	// check use of ...
	first := true // only report first occurrence
	for i, f := range list {
		if t, _ := f.Type.(*DotsType); t != nil && (!dddok || i+1 < len(list)) {
			if first {
				first = false
				if dddok {
					p.errorAt(t.pos, "can only use ... with final parameter")
				} else {
					p.errorAt(t.pos, "invalid use of ...")
				}
			}
			// use T instead of invalid ...T
			f.Type = t.Elem
		}
	}

	return
}

func (p *parser) badExpr() *BadExpr {
	b := new(BadExpr)
	b.pos = p.pos()
	return b
}

// ----------------------------------------------------------------------------
// Statements

// SimpleStmt = EmptyStmt | ExpressionStmt | SendStmt | IncDecStmt | Assignment | ShortVarDecl .
func (p *parser) simpleStmt(lhs Expr, keyword token) SimpleStmt {
	if trace {
		defer p.trace("simpleStmt")()
	}

	if keyword == _For && p.tok == _Range {
		// _Range expr
		if debug && lhs != nil {
			panic("invalid call of simpleStmt")
		}
		return p.newRangeClause(nil, false)
	}

	if lhs == nil {
		lhs = p.exprList()
	}

	if _, ok := lhs.(*ListExpr); !ok && p.tok != _Assign && p.tok != _Define {
		// expr
		pos := p.pos()
		switch p.tok {
		case _AssignOp:
			// lhs op= rhs
			op := p.op
			p.next()
			return p.newAssignStmt(pos, op, lhs, p.expr())

		case _IncOp:
			// lhs++ or lhs--
			op := p.op
			p.next()
			return p.newAssignStmt(pos, op, lhs, nil)

		case _Arrow:
			// lhs <- rhs
			s := new(SendStmt)
			s.pos = pos
			p.next()
			s.Chan = lhs
			s.Value = p.expr()
			return s

		default:
			// expr
			s := new(ExprStmt)
			s.pos = lhs.Pos()
			s.X = lhs
			return s
		}
	}

	// expr_list
	switch p.tok {
	case _Assign, _Define:
		pos := p.pos()
		var op Operator
		if p.tok == _Define {
			op = Def
		}
		p.next()

		if keyword == _For && p.tok == _Range {
			// expr_list op= _Range expr
			return p.newRangeClause(lhs, op == Def)
		}

		// expr_list op= expr_list
		rhs := p.exprList()

		if x, ok := rhs.(*TypeSwitchGuard); ok && keyword == _Switch && op == Def {
			if lhs, ok := lhs.(*Name); ok {
				// switch … lhs := rhs.(type)
				x.Lhs = lhs
				s := new(ExprStmt)
				s.pos = x.Pos()
				s.X = x
				return s
			}
		}

		return p.newAssignStmt(pos, op, lhs, rhs)

	default:
		p.syntaxError("expected := or = or comma")
		p.advance(_Semi, _Rbrace)
		// make the best of what we have
		if x, ok := lhs.(*ListExpr); ok {
			lhs = x.ElemList[0]
		}
		s := new(ExprStmt)
		s.pos = lhs.Pos()
		s.X = lhs
		return s
	}
}

func (p *parser) newRangeClause(lhs Expr, defi bool) *RangeClause {
	r := new(RangeClause)
	r.pos = p.pos()
	p.next() // consume _Range
	r.Lhs = lhs
	r.Def = defi
	r.X = p.expr()
	return r
}

func (p *parser) newAssignStmt(pos Pos, op Operator, lhs, rhs Expr) *AssignStmt {
	a := new(AssignStmt)
	a.pos = pos
	a.Op = op
	a.Lhs = lhs
	a.Rhs = rhs
	return a
}

func (p *parser) labeledStmtOrNil(label *Name) Stmt {
	if trace {
		defer p.trace("labeledStmt")()
	}

	s := new(LabeledStmt)
	s.pos = p.pos()
	s.Label = label

	p.want(_Colon)

	if p.tok == _Rbrace {
		// We expect a statement (incl. an empty statement), which must be
		// terminated by a semicolon. Because semicolons may be omitted before
		// an _Rbrace, seeing an _Rbrace implies an empty statement.
		e := new(EmptyStmt)
		e.pos = p.pos()
		s.Stmt = e
		return s
	}

	s.Stmt = p.stmtOrNil()
	if s.Stmt != nil {
		return s
	}

	// report error at line of ':' token
	p.syntaxErrorAt(s.pos, "missing statement after label")
	// we are already at the end of the labeled statement - no need to advance
	return nil // avoids follow-on errors (see e.g., fixedbugs/bug274.go)
}

// context must be a non-empty string unless we know that p.tok == _Lbrace.
func (p *parser) blockStmt(context string) *BlockStmt {
	if trace {
		defer p.trace("blockStmt")()
	}

	s := new(BlockStmt)
	s.pos = p.pos()

	// people coming from C may forget that braces are mandatory in Go
	if !p.got(_Lbrace) {
		p.syntaxError("expected { after " + context)
		p.advance(_Name, _Rbrace)
		s.Rbrace = p.pos() // in case we found "}"
		if p.got(_Rbrace) {
			return s
		}
	}

	s.List = p.stmtList()
	s.Rbrace = p.pos()
	p.want(_Rbrace)

	return s
}

func (p *parser) declStmt(f func(*Group) Decl) *DeclStmt {
	if trace {
		defer p.trace("declStmt")()
	}

	s := new(DeclStmt)
	s.pos = p.pos()

	p.next() // _Const, _Type, or _Var
	s.DeclList = p.appendGroup(nil, f)

	return s
}

func (p *parser) forStmt() Stmt {
	if trace {
		defer p.trace("forStmt")()
	}

	s := new(ForStmt)
	s.pos = p.pos()

	s.Init, s.Cond, s.Post = p.header(_For)
	s.Body = p.blockStmt("for clause")

	return s
}

func (p *parser) header(keyword token) (init SimpleStmt, cond Expr, post SimpleStmt) {
	p.want(keyword)

	if p.tok == _Lbrace {
		if keyword == _If {
			p.syntaxError("missing condition in if statement")
			cond = p.badExpr()
		}
		return
	}
	// p.tok != _Lbrace

	outer := p.xnest
	p.xnest = -1

	if p.tok != _Semi {
		// accept potential varDecl but complain
		if p.got(_Var) {
			p.syntaxError(fmt.Sprintf("var declaration not allowed in %s initializer", keyword.String()))
		}
		init = p.simpleStmt(nil, keyword)
		// If we have a range clause, we are done (can only happen for keyword == _For).
		if _, ok := init.(*RangeClause); ok {
			p.xnest = outer
			return
		}
	}

	var condStmt SimpleStmt
	var semi struct {
		pos Pos
		lit string // valid if pos.IsKnown()
	}
	if p.tok != _Lbrace {
		if p.tok == _Semi {
			semi.pos = p.pos()
			semi.lit = p.lit
			p.next()
		} else {
			// asking for a '{' rather than a ';' here leads to a better error message
			p.want(_Lbrace)
			if p.tok != _Lbrace {
				p.advance(_Lbrace, _Rbrace) // for better synchronization (e.g., go.dev/issue/22581)
			}
		}
		if keyword == _For {
			if p.tok != _Semi {
				if p.tok == _Lbrace {
					p.syntaxError("expected for loop condition")
					goto done
				}
				condStmt = p.simpleStmt(nil, 0 /* range not permitted */)
			}
			p.want(_Semi)
			if p.tok != _Lbrace {
				post = p.simpleStmt(nil, 0 /* range not permitted */)
				if a, _ := post.(*AssignStmt); a != nil && a.Op == Def {
					p.syntaxErrorAt(a.Pos(), "cannot declare in post statement of for loop")
				}
			}
		} else if p.tok != _Lbrace {
			condStmt = p.simpleStmt(nil, keyword)
		}
	} else {
		condStmt = init
		init = nil
	}

done:
	// unpack condStmt
	switch s := condStmt.(type) {
	case nil:
		if keyword == _If && semi.pos.IsKnown() {
			if semi.lit != "semicolon" {
				p.syntaxErrorAt(semi.pos, fmt.Sprintf("unexpected %s, expected { after if clause", semi.lit))
			} else {
				p.syntaxErrorAt(semi.pos, "missing condition in if statement")
			}
			b := new(BadExpr)
			b.pos = semi.pos
			cond = b
		}
	case *ExprStmt:
		cond = s.X
	default:
		// A common syntax error is to write '=' instead of '==',
		// which turns an expression into an assignment. Provide
		// a more explicit error message in that case to prevent
		// further confusion.
		var str string
		if as, ok := s.(*AssignStmt); ok && as.Op == 0 {
			// Emphasize complex Lhs and Rhs of assignment with parentheses to highlight '='.
			str = "assignment " + emphasize(as.Lhs) + " = " + emphasize(as.Rhs)
		} else {
			str = String(s)
		}
		p.syntaxErrorAt(s.Pos(), fmt.Sprintf("cannot use %s as value", str))
	}

	p.xnest = outer
	return
}

// emphasize returns a string representation of x, with (top-level)
// binary expressions emphasized by enclosing them in parentheses.
func emphasize(x Expr) string {
	s := String(x)
	if op, _ := x.(*Operation); op != nil && op.Y != nil {
		// binary expression
		return "(" + s + ")"
	}
	return s
}

// needsFmtImport checks if the file uses printf and doesn't already import fmt
func (p *parser) needsFmtImport(f *File) bool {
	// Check if fmt is already imported
	for _, decl := range f.DeclList {
		if imp, ok := decl.(*ImportDecl); ok && imp.Path != nil {
			if imp.Path.Value == `"fmt"` {
				return false
			}
		}
	}

	text := string(p.scanner.source.buf)
	lines := strings.Split(text, "\n")

	// Check if fmt is already imported in the source text
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip comment lines starting with #
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Look for fmt import statements
		if strings.Contains(line, `import "fmt"`) || strings.Contains(line, `import("fmt")`) {
			return false
		}
	}

	// Check if printf is used by scanning the source text directly
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip comment lines starting with #
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Look for printf function calls
		if strings.Contains(line, "printf(") || strings.Contains(line, "put(") {
			return true
		}
	}
	return false
}

func (p *parser) ifStmt() *IfStmt {
	if trace {
		defer p.trace("ifStmt")()
	}

	s := new(IfStmt)
	s.pos = p.pos()

	s.Init, s.Cond, _ = p.header(_If)
	s.Then = p.blockStmt("if clause")

	if p.got(_Else) {
		switch p.tok {
		case _If:
			s.Else = p.ifStmt()
		case _Lbrace:
			s.Else = p.blockStmt("")
		default:
			p.syntaxError("else must be followed by if or statement block")
			p.advance(_Name, _Rbrace)
		}
	}

	return s
}

func (p *parser) switchStmt() *SwitchStmt {
	if trace {
		defer p.trace("switchStmt")()
	}

	s := new(SwitchStmt)
	s.pos = p.pos()

	s.Init, s.Tag, _ = p.header(_Switch)

	if !p.got(_Lbrace) {
		p.syntaxError("missing { after switch clause")
		p.advance(_Case, _Default, _Rbrace)
	}
	for p.tok != _EOF && p.tok != _Rbrace {
		s.Body = append(s.Body, p.caseClause())
	}
	s.Rbrace = p.pos()
	p.want(_Rbrace)

	return s
}

func (p *parser) selectStmt() *SelectStmt {
	if trace {
		defer p.trace("selectStmt")()
	}

	s := new(SelectStmt)
	s.pos = p.pos()

	p.want(_Select)
	if !p.got(_Lbrace) {
		p.syntaxError("missing { after select clause")
		p.advance(_Case, _Default, _Rbrace)
	}
	for p.tok != _EOF && p.tok != _Rbrace {
		s.Body = append(s.Body, p.commClause())
	}
	s.Rbrace = p.pos()
	p.want(_Rbrace)

	return s
}

func (p *parser) caseClause() *CaseClause {
	if trace {
		defer p.trace("caseClause")()
	}

	c := new(CaseClause)
	c.pos = p.pos()

	switch p.tok {
	case _Case:
		p.next()
		c.Cases = p.exprList()

	case _Default:
		p.next()

	default:
		p.syntaxError("expected case or default or }")
		p.advance(_Colon, _Case, _Default, _Rbrace)
	}

	c.Colon = p.pos()
	p.want(_Colon)
	c.Body = p.stmtList()

	return c
}

func (p *parser) commClause() *CommClause {
	if trace {
		defer p.trace("commClause")()
	}

	c := new(CommClause)
	c.pos = p.pos()

	switch p.tok {
	case _Case:
		p.next()
		c.Comm = p.simpleStmt(nil, 0)

		// The syntax restricts the possible simple statements here to:
		//
		//     lhs <- x (send statement)
		//     <-x
		//     lhs = <-x
		//     lhs := <-x
		//
		// All these (and more) are recognized by simpleStmt and invalid
		// syntax trees are flagged later, during type checking.

	case _Default:
		p.next()

	default:
		p.syntaxError("expected case or default or }")
		p.advance(_Colon, _Case, _Default, _Rbrace)
	}

	c.Colon = p.pos()
	p.want(_Colon)
	c.Body = p.stmtList()

	return c
}

// stmtOrNil parses a statement if one is present, or else returns nil.
//
//	Statement =
//		Declaration | LabeledStmt | SimpleStmt |
//		GoStmt | ReturnStmt | BreakStmt | ContinueStmt | GotoStmt |
//		FallthroughStmt | Block | IfStmt | SwitchStmt | SelectStmt | ForStmt |
//		DeferStmt .
func (p *parser) stmtOrNil() Stmt {
	if trace {
		defer p.trace("stmt " + p.tok.String())()
	}

	// Most statements (assignments) start with an identifier;
	// look for it first before doing anything more expensive.
	if p.tok == _Name {
		p.clearPragma()
		lhs := p.exprList()
		if label, ok := lhs.(*Name); ok && p.tok == _Colon {
			return p.labeledStmtOrNil(label)
		}
		return p.simpleStmt(lhs, 0)
	}

	switch p.tok {
	case _Var:
		return p.declStmt(p.varDecl)

	case _Const:
		return p.declStmt(p.constDecl)

	case _Type:
		return p.declStmt(p.typeDecl)

	case _Enum:
		return p.enumStmt()
	}

	p.clearPragma()

	switch p.tok {
	case _Lbrace:
		return p.blockStmt("")

	case _Operator, _Star:
		switch p.op {
		case Add, Sub, Mul, And, Xor, Not:
			return p.simpleStmt(nil, 0) // unary operators
		}

	case _Literal, _Func, _Lparen, // operands
		_Lbrack, _Struct, _Map, _Chan, _Interface, // composite types
		_Arrow: // receive operator
		return p.simpleStmt(nil, 0)

	case _For:
		return p.forStmt()

	case _Switch:
		return p.switchStmt()

	case _Select:
		return p.selectStmt()

	case _If:
		return p.ifStmt()

	case _Fallthrough:
		s := new(BranchStmt)
		s.pos = p.pos()
		p.next()
		s.Tok = _Fallthrough
		return s

	case _Break, _Continue:
		s := new(BranchStmt)
		s.pos = p.pos()
		s.Tok = p.tok
		p.next()
		if p.tok == _Name {
			s.Label = p.name()
		}
		return s

	case _Check:
		s := new(CheckStmt)
		s.pos = p.pos()
		p.next()
		s.Cond = p.expr()
		return s

	case _Go, _Defer:
		return p.callStmt()

	case _Goto:
		s := new(BranchStmt)
		s.pos = p.pos()
		s.Tok = _Goto
		p.next()
		s.Label = p.name()
		return s

	case _Return:
		s := new(ReturnStmt)
		s.pos = p.pos()
		p.next()
		if p.tok != _Semi && p.tok != _Rbrace {
			s.Results = p.exprList()
		}
		return s

	case _Semi:
		s := new(EmptyStmt)
		s.pos = p.pos()
		return s
	}

	return nil
}

// StatementList = { Statement ";" } .
func (p *parser) stmtList() (l []Stmt) {
	if trace {
		defer p.trace("stmtList")()
	}

	for p.tok != _EOF && p.tok != _Rbrace && p.tok != _Case && p.tok != _Default {
		s := p.stmtOrNil()
		p.clearPragma()
		if s == nil {
			break
		}
		l = append(l, s)
		// ";" is optional before "}"
		if !p.got(_Semi) && p.tok != _Rbrace {
			p.syntaxError("at end of statement")
			p.advance(_Semi, _Rbrace, _Case, _Default)
			p.got(_Semi) // avoid spurious empty statement
		}
	}
	return
}

// argList parses a possibly empty, comma-separated list of arguments,
// optionally followed by a comma (if not empty), and closed by ")".
// The last argument may be followed by "...".
//
// argList = [ arg { "," arg } [ "..." ] [ "," ] ] ")" .
func (p *parser) argList() (list []Expr, hasDots bool) {
	if trace {
		defer p.trace("argList")()
	}

	p.xnest++
	p.list("argument list", _Comma, _Rparen, func() bool {
		list = append(list, p.expr())
		hasDots = p.got(_DotDotDot)
		return hasDots
	})
	p.xnest--

	return
}

// ----------------------------------------------------------------------------
// Common productions

func (p *parser) name() *Name {
	// no tracing to avoid overly verbose output

	if p.tok == _Name {
		n := NewName(p.pos(), p.lit)
		p.next()
		return n
	}

	n := NewName(p.pos(), "_")
	p.syntaxError("expected name")
	p.advance()
	return n
}

// IdentifierList = identifier { "," identifier } .
// The first name must be provided.
func (p *parser) nameList(first *Name) []*Name {
	if trace {
		defer p.trace("nameList")()
	}

	if debug && first == nil {
		panic("first name not provided")
	}

	l := []*Name{first}
	for p.got(_Comma) {
		l = append(l, p.name())
	}

	return l
}

// The first name may be provided, or nil.
func (p *parser) qualifiedName(name *Name) Expr {
	if trace {
		defer p.trace("qualifiedName")()
	}

	var x Expr
	switch {
	case name != nil:
		x = name
	case p.tok == _Name:
		x = p.name()
	default:
		x = NewName(p.pos(), "_")
		p.syntaxError("expected name")
		p.advance(_Dot, _Semi, _Rbrace)
	}

	if p.tok == _Dot {
		s := new(SelectorExpr)
		s.pos = p.pos()
		p.next()
		s.X = x
		s.Sel = p.name()
		x = s
	}

	if p.tok == _Lbrack {
		x = p.typeInstance(x)
	}

	return x
}

// ExpressionList = Expression { "," Expression } .
func (p *parser) exprList() Expr {
	if trace {
		defer p.trace("exprList")()
	}

	x := p.expr()
	if p.got(_Comma) {
		list := []Expr{x, p.expr()}
		for p.got(_Comma) {
			list = append(list, p.expr())
		}
		t := new(ListExpr)
		t.pos = x.Pos()
		t.ElemList = list
		x = t
	}
	return x
}

// typeList parses a non-empty, comma-separated list of types,
// optionally followed by a comma. If strict is set to false,
// the first element may also be a (non-type) expression.
// If there is more than one argument, the result is a *ListExpr.
// The comma result indicates whether there was a (separating or
// trailing) comma.
//
// typeList = arg { "," arg } [ "," ] .
func (p *parser) typeList(strict bool) (x Expr, comma bool) {
	if trace {
		defer p.trace("typeList")()
	}

	p.xnest++
	if strict {
		x = p.type_()
	} else {
		x = p.expr()
	}
	if p.got(_Comma) {
		comma = true
		if t := p.typeOrNil(); t != nil {
			list := []Expr{x, t}
			for p.got(_Comma) {
				if t = p.typeOrNil(); t == nil {
					break
				}
				list = append(list, t)
			}
			l := new(ListExpr)
			l.pos = x.Pos() // == list[0].Pos()
			l.ElemList = list
			x = l
		}
	}
	p.xnest--
	return
}

// Unparen returns e with any enclosing parentheses stripped.
func Unparen(x Expr) Expr {
	for {
		p, ok := x.(*ParenExpr)
		if !ok {
			break
		}
		x = p.X
	}
	return x
}

// UnpackListExpr unpacks a *ListExpr into a []Expr.
func UnpackListExpr(x Expr) []Expr {
	switch x := x.(type) {
	case nil:
		return nil
	case *ListExpr:
		return x.ElemList
	default:
		return []Expr{x}
	}
}
