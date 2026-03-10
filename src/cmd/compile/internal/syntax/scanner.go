// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements scanner, a lexical tokenizer for
// Go source. After initialization, consecutive calls of
// next advance the scanner one token at a time.
//
// This file, source.go, tokens.go, and token_string.go are self-contained
// (`go tool compile scanner.go source.go tokens.go token_string.go` compiles)
// and thus could be made into their own package.

package syntax

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The mode flags below control which comments are reported
// by calling the error handler. If no flag is set, comments
// are ignored.
const (
	comments   uint = 1 << iota // call handler for all comments
	directives                  // call handler for directives only
)

type scanner struct {
	source
	mode     uint
	nlsemi   bool   // if set '\n' and EOF translate to ';'
	filename string // filename being scanned, used for .goo file detection

	// current token, valid after calling next()
	line, col uint
	blank     bool // line is blank up to col
	tok       token
	lit       string   // valid if tok is _Name, _Literal, or _Semi ("semicolon", "newline", or "EOF"); may be malformed if bad is true
	bad       bool     // valid if tok is _Literal, true if a syntax error occurred, lit may be malformed
	kind      LitKind  // valid if tok is _Literal
	op        Operator // valid if tok is _Operator, _Star, _AssignOp, or _IncOp
	prec      int      // valid if tok is _Operator, _Star, _AssignOp, or _IncOp
	
	// implicit multiplication state
	implicitMul bool // if true, emit '*' operator next
}

func (s *scanner) init(src io.Reader, errh func(line, col uint, msg string), mode uint, filename string) {
	s.source.init(src, errh)
	s.mode = mode
	s.filename = filename
	s.nlsemi = false
}

// errorf reports an error at the most recently read character position.
func (s *scanner) errorf(format string, args ...any) {
	s.error(fmt.Sprintf(format, args...))
}

// errorAtf reports an error at a byte column offset relative to the current token start.
func (s *scanner) errorAtf(offset int, format string, args ...any) {
	s.errh(s.line, s.col+uint(offset), fmt.Sprintf(format, args...))
}

// setLit sets the scanner state for a recognized _Literal token.
func (s *scanner) setLit(kind LitKind, ok bool) {
	s.nlsemi = true
	s.tok = _Literal
	s.lit = string(s.segment())
	s.bad = !ok
	s.kind = kind
}

// next advances the scanner by reading the next token.
//
// If a read, source encoding, or lexical error occurs, next calls
// the installed error handler with the respective error position
// and message. The error message is guaranteed to be non-empty and
// never starts with a '/'. The error handler must exist.
//
// If the scanner mode includes the comments flag and a comment
// (including comments containing directives) is encountered, the
// error handler is also called with each comment position and text
// (including opening /* or // and closing */, but without a newline
// at the end of line comments). Comment text always starts with a /
// which can be used to distinguish these handler calls from errors.
//
// If the scanner mode includes the directives (but not the comments)
// flag, only comments containing a //line, /*line, or //go: directive
// are reported, in the same way as regular comments.
func (s *scanner) next() {
	nlsemi := s.nlsemi
	s.nlsemi = false

	// Handle implicit multiplication state
	if s.implicitMul {
		s.implicitMul = false
		s.op, s.prec = Mul, precMul
		s.tok = _Operator
		s.lit = "*"
		return
	}

redo:
	// skip white space
	s.stop()
	startLine, startCol := s.pos()
	for s.ch == ' ' || s.ch == '\t' || s.ch == '\n' && !nlsemi || s.ch == '\r' {
		s.nextch()
	}

	// token start
	s.line, s.col = s.pos()
	s.blank = s.line > startLine || startCol == colbase
	s.start()
	if isLetter(s.ch) || s.ch >= utf8.RuneSelf && s.ch != '…' && s.ch != '·' && s.ch != '²' && s.ch != '³' && s.ch != '≈' && s.atIdentChar(true) {
		s.nextch()
		s.ident()
		return
	}

	switch s.ch {
	case -1:
		if nlsemi {
			s.lit = "EOF"
			s.tok = _Semi
			break
		}
		s.tok = _EOF

	case '\n':
		s.nextch()
		s.lit = "newline"
		s.tok = _Semi

	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		s.number(false)

	case '"':
		s.stdString()

	case '`':
		s.rawString()

	case '\'':
		s.rune()

	case '(':
		s.nextch()
		s.tok = _Lparen

	case '[':
		s.nextch()
		s.tok = _Lbrack

	case '{':
		s.nextch()
		s.tok = _Lbrace

	case ',':
		s.nextch()
		s.tok = _Comma

	case ';':
		s.nextch()
		s.lit = "semicolon"
		s.tok = _Semi

	case ')':
		s.nextch()
		s.nlsemi = true
		s.tok = _Rparen

	case ']':
		s.nextch()
		s.nlsemi = true
		s.tok = _Rbrack

	case '}':
		s.nextch()
		s.nlsemi = true
		s.tok = _Rbrace

	case ':':
		s.nextch()
		if s.ch == '=' {
			s.nextch()
			s.tok = _Define
			break
		}
		s.tok = _Colon

	case '.':
		s.nextch()
		if s.ch == '.' {
			s.nextch()
			if s.ch == '.' {
				// ... is kept as _DotDotDot for variadic parameters and array literals
				s.nextch()
				s.tok = _DotDotDot
				break
			}
			// .. is exclusive range operator
			s.op, s.prec = RangeExclusive, precAdd
			s.tok = _Operator
			break
		}
		if isDecimal(s.ch) {
			s.number(true)
			break
		}
		s.tok = _Dot

	case '…': // ellipsis (U+2026) as inclusive range operator
		s.nextch()
		s.op, s.prec = Range, precAdd
		s.tok = _Operator

	case '·': // Middle dot (U+00B7) as multiplication operator
		s.nextch()
		s.op, s.prec = MiddleDot, precMul
		s.tok = _Operator

	case '+':
		s.nextch()
		s.op, s.prec = Add, precAdd
		if s.ch != '+' {
			goto assignop
		}
		s.nextch()
		s.nlsemi = true
		s.tok = _IncOp

	case '-':
		s.nextch()
		s.op, s.prec = Sub, precAdd
		if s.ch != '-' {
			goto assignop
		}
		s.nextch()
		s.nlsemi = true
		s.tok = _IncOp

	case '*':
		s.nextch()
		if s.ch == '*' && s.isPowerContext() {
			s.nextch()
			s.op, s.prec = Power, precPower
			s.tok = _Power
			break
		}
		s.op, s.prec = Mul, precMul
		// don't goto assignop - want _Star token
		if s.ch == '=' {
			s.nextch()
			s.tok = _AssignOp
			break
		}
		s.tok = _Star

	case '/':
		s.nextch()
		if s.ch == '/' {
			s.nextch()
			s.lineComment()
			goto redo
		}
		if s.ch == '*' {
			s.nextch()
			s.fullComment()
			if line, _ := s.pos(); line > s.line && nlsemi {
				// A multi-line comment acts like a newline;
				// it translates to a ';' if nlsemi is set.
				s.lit = "newline"
				s.tok = _Semi
				break
			}
			goto redo
		}
		s.op, s.prec = Div, precMul
		goto assignop

	case '%':
		s.nextch()
		s.op, s.prec = Rem, precMul
		goto assignop

	case '&':
		s.nextch()
		if s.ch == '&' {
			s.nextch()
			s.op, s.prec = AndAnd, precAndAnd
			s.tok = _Operator
			break
		}
		s.op, s.prec = And, precMul
		if s.ch == '^' {
			s.nextch()
			s.op = AndNot
		}
		goto assignop

	case '|':
		s.nextch()
		if s.ch == '|' {
			s.nextch()
			s.op, s.prec = OrOr, precOrOr
			s.tok = _Operator
			break
		}
		s.op, s.prec = Or, precAdd
		goto assignop

	case '^':
		s.nextch()
		s.op, s.prec = Xor, precAdd
		goto assignop

	case '<':
		s.nextch()
		if s.ch == '=' {
			s.nextch()
			s.op, s.prec = Leq, precCmp
			s.tok = _Operator
			break
		}
		if s.ch == '<' {
			s.nextch()
			s.op, s.prec = Shl, precMul
			goto assignop
		}
		if s.ch == '-' {
			s.nextch()
			s.tok = _Arrow
			break
		}
		s.op, s.prec = Lss, precCmp
		s.tok = _Operator

	case '>':
		s.nextch()
		if s.ch == '=' {
			s.nextch()
			s.op, s.prec = Geq, precCmp
			s.tok = _Operator
			break
		}
		if s.ch == '>' {
			s.nextch()
			s.op, s.prec = Shr, precMul
			goto assignop
		}
		s.op, s.prec = Gtr, precCmp
		s.tok = _Operator

	case '=':
		s.nextch()
		if s.ch == '=' {
			s.nextch()
			s.op, s.prec = Eql, precCmp
			s.tok = _Operator
			break
		}
		if s.ch == '>' {
			s.nextch()
			s.tok = _Lambda
			break
		}
		s.tok = _Assign

	case '!':
		s.nextch()
		if s.ch == '=' {
			s.nextch()
			s.op, s.prec = Neq, precCmp
			s.tok = _Operator
			break
		}
		s.op, s.prec = Not, 0
		s.tok = _Operator

	case '~':
		s.nextch()
		s.op, s.prec = Tilde, 0
		s.tok = _Operator

	case '?':
		s.nextch()
		if s.ch == '?' {
			s.nextch()
			s.op, s.prec = NullCoalesce, precOrOr
			s.tok = _Operator
			break
		}
		if s.ch == '.' {
			s.nextch()
			s.tok = _OptChain
			break
		}
		s.errorf("invalid character '?'")
		goto redo

	case '#':
		// Check if this is a 1-indexed array access operator or a comment
		// Comments start at beginning of line or after whitespace
		// 1-indexed access follows an identifier/expression
		if s.canBeIndexOperator() {
			s.tok = _Hash
			s.nextch()
		} else {
			s.nextch()
			s.hashComment()
			goto redo
		}

	case '²', '³':
		// Handle superscript operators for mathematical expressions like 3², 2³
		if s.transformsEnabled() {
			ch := s.ch // Save the current character before advancing
			s.tok = _Superscript
			s.lit = string(ch) // Save the superscript symbol
			s.nextch()
		} else {
			s.errorf("invalid character %#U", s.ch)
			s.nextch()
			goto redo
		}

	default:
		// Check for superscript characters specifically
		if s.ch == '²' || s.ch == '³' {
			if s.transformsEnabled() {
				ch := s.ch
				s.tok = _Superscript
				s.lit = string(ch)
				s.nextch()
			} else {
				s.errorf("invalid character %#U", s.ch)
				s.nextch()
				goto redo
			}
		} else if s.ch == 'π' {
			// Handle π as a mathematical constant
			s.tok = _Pi
			s.lit = "π"
			s.nlsemi = true
			s.nextch()
		} else if s.ch == 'τ' {
			// Handle τ as a mathematical constant
			s.tok = _Tau
			s.lit = "τ"
			s.nlsemi = true
			s.nextch()
		} else if s.ch == '≈' {
			// Handle ≈ as approximate equality operator
			s.nextch()
			s.tok = _Operator
			s.op = Approx
			s.prec = precCmp
		} else {
			s.errorf("invalid character %#U", s.ch)
			s.nextch()
			goto redo
		}
	}

	return

assignop:
	if s.ch == '=' {
		s.nextch()
		s.tok = _AssignOp
		return
	}
	s.tok = _Operator
}

func (s *scanner) ident() {
	// accelerate common case (7bit ASCII)
	for isLetter(s.ch) || isDecimal(s.ch) {
		s.nextch()
	}

	// general case
	if s.ch >= utf8.RuneSelf {
		for s.atIdentChar(false) {
			s.nextch()
		}
	}

	// allow '!' as final character in identifier
	if s.ch == '!' {
		s.nextch()
	}

	// possibly a keyword
	lit := s.segment()
	litStr := string(lit)
	
	// Check for .goo-specific keywords when transforms are enabled
	if s.transformsEnabled() {
		switch litStr {
		case "as":
			s.tok = _As
			return
		case "enum":
			s.tok = _Enum
			return
		case "and":
			s.op, s.prec = TruthyAnd, precAndAnd
			s.tok = _Operator
			return
		case "catch":
			s.tok = _Catch
			return
		case "class":
			s.tok = _Class
			return
		case "check":
			s.tok = _Check
			return
		case "def":
			s.tok = _Def
			return
		case "try":
			s.tok = _Try
			return
		case "void":
			s.tok = _Void
			return
		case "in":
			s.op, s.prec = In, precAdd
			s.tok = _Operator
			return
		case "is":
			s.op, s.prec = IS, precMul
			s.tok = _Operator
			return
		case "while":
			s.tok = _While
			return
		}
	}
	
	if len(lit) >= 2 {
		h := hash(lit)
		for keywordMap[h] != 0 {
			if tok := keywordMap[h]; tokStrFast(tok) == litStr {
				s.nlsemi = contains(1<<_Break|1<<_Continue|1<<_Fallthrough|1<<_Return, tok)
				s.tok = tok
				return
			}
			h = (h + 1) & uint(len(keywordMap)-1)
		}
	}

	// special case for 'not' operator
	if string(lit) == "not" {
		s.op, s.prec = Not, 0
		s.tok = _Operator
		return
	}

	// special case for 'ø' as nil
	if string(lit) == "ø" {
		s.nlsemi = true
		s.lit = "nil"
		s.tok = _Name
		return
	}

	// special case for '≠' as !=
	if string(lit) == "≠" {
		s.op, s.prec = Neq, precCmp
		s.tok = _Operator
		return
	}

	if s.transformsEnabled() {
		// special case for '¬' as !
		if string(lit) == "¬" {
			s.op, s.prec = Not, 0
			s.tok = _Operator
			return
		}

		// special case for 'and' as &&
		if string(lit) == "and" {
			s.op, s.prec = AndAnd, precAndAnd
			s.tok = _Operator
			return
		}

		// special case for 'or' as ||
		if string(lit) == "or" {
			s.op, s.prec = OrOr, precOrOr
			s.tok = _Operator
			return
		}
	}

	// special case for 'isa' as type assertion operator
	if string(lit) == "isa" && os.Getenv("GOO_USE_TRANSFORMERS") == "1" {
		fmt.Printf("DEBUG: Scanner recognized 'isa' as IS operator\n")
		s.op, s.prec = IS, precCmp
		s.tok = _Operator
		return
	}

	s.nlsemi = true
	s.lit = string(lit)
	s.tok = _Name
}

// tokStrFast is a faster version of token.String, which assumes that tok
// is one of the valid tokens - and can thus skip bounds checks.
func tokStrFast(tok token) string {
	return TokenNames[tok]
}

func (s *scanner) atIdentChar(first bool) bool {
	switch {
	case unicode.IsLetter(s.ch) || s.ch == '_':
		// ok
	case unicode.IsDigit(s.ch):
		if first {
			s.errorf("identifier cannot begin with digit %#U", s.ch)
		}
	case s.ch == '≠' || s.ch == '¬':
		// allow special Unicode operators
	case s.ch >= utf8.RuneSelf:
		// Allow Unicode symbols commonly used in units when transformers are enabled
		if s.transformsEnabled() && s.isUnicodeSymbol(s.ch) {
			// Allow Unicode unit symbols like °, ², ³, μ, Ω, π, τ
			// ok
		} else {
			s.errorf("invalid character %#U in identifier", s.ch)
		}
	default:
		return false
	}
	return true
}

// hash is a perfect hash function for keywords.
// It handles strings of any length.
func hash(s []byte) uint {
	if len(s) == 0 {
		return 0
	}
	if len(s) == 1 {
		return (uint(s[0])<<4 + uint(len(s))) & uint(len(keywordMap)-1)
	}
	return (uint(s[0])<<4 ^ uint(s[1]) + uint(len(s))) & uint(len(keywordMap)-1)
}

var keywordMap [1 << 12]token // size must be power of two

func init() {
	// populate keywordMap with standard Go keywords only
	// Custom .goo tokens are handled separately in ident()
	for tok := _keywords_start + 1; tok < _CUSTOM_TOKENS_; tok++ {
		h := hash([]byte(tok.String()))
		// Handle hash collisions with linear probing
		for keywordMap[h] != 0 {
			h = (h + 1) & uint(len(keywordMap)-1)
		}
		keywordMap[h] = tok
	}
}

func useTransforms() bool {
	return os.Getenv("GOO_USE_TRANSFORMERS") == "1"
}

// transformsEnabled returns true if .goo syntax should be enabled for this scanner
func (s *scanner) transformsEnabled() bool {
	// Check environment variable first
	envSet := os.Getenv("GOO_USE_TRANSFORMERS") == "1"
	hasGooExt := strings.HasSuffix(s.filename, ".goo")
	result := envSet && (s.filename == "" || hasGooExt) || hasGooExt
	return result
}

// isPowerContext determines if ** should be treated as power operator vs pointer-to-pointer
func (s *scanner) isPowerContext() bool {
	// Only treat ** as power operator in .goo files
	if !strings.HasSuffix(s.filename, ".goo") {
		return false
	}
	
	// Look at the character before the first * to determine context
	if s.source.r >= 3 {
		prevChar := s.source.buf[s.source.r-3] // Character before first *
		
		// Power context: digit, identifier, ) or whitespace before **
		// Examples: "3 ** 2", "x**2", "(a+b)**3"  
		if (prevChar >= '0' && prevChar <= '9') || // digit
		   (prevChar >= 'a' && prevChar <= 'z') || // lowercase letter
		   (prevChar >= 'A' && prevChar <= 'Z') || // uppercase letter  
		   prevChar == '_' || prevChar == ')' ||   // identifier chars or closing paren
		   prevChar == ' ' || prevChar == '\t' {   // whitespace
			return true
		}
		
		// Type context: comma, opening paren, or other type indicators
		// Examples: "func(a **int)", "var x **Type"
		return false
	}
	
	return false
}

func lower(ch rune) rune     { return ('a' - 'A') | ch } // returns lower-case ch iff ch is ASCII letter
func isLetter(ch rune) bool  { return 'a' <= lower(ch) && lower(ch) <= 'z' || ch == '_' }
func isDecimal(ch rune) bool { return '0' <= ch && ch <= '9' }
func isHex(ch rune) bool     { return '0' <= ch && ch <= '9' || 'a' <= lower(ch) && lower(ch) <= 'f' }

// digits accepts the sequence { digit | '_' }.
// If base <= 10, digits accepts any decimal digit but records
// the index (relative to the literal start) of a digit >= base
// in *invalid, if *invalid < 0.
// digits returns a bitset describing whether the sequence contained
// digits (bit 0 is set), or separators '_' (bit 1 is set).
func (s *scanner) digits(base int, invalid *int) (digsep int) {
	if base <= 10 {
		maxi := rune('0' + base)
		for isDecimal(s.ch) || s.ch == '_' {
			ds := 1
			if s.ch == '_' {
				ds = 2
			} else if s.ch >= maxi && *invalid < 0 {
				_, col := s.pos()
				*invalid = int(col - s.col) // record invalid rune index
			}
			digsep |= ds
			s.nextch()
		}
	} else {
		for isHex(s.ch) || s.ch == '_' {
			ds := 1
			if s.ch == '_' {
				ds = 2
			}
			digsep |= ds
			s.nextch()
		}
	}
	return
}

func (s *scanner) number(seenPoint bool) {
	ok := true
	kind := IntLit
	base := 10        // number base
	prefix := rune(0) // one of 0 (decimal), '0' (0-octal), 'x', 'o', or 'b'
	digsep := 0       // bit 0: digit present, bit 1: '_' present
	invalid := -1     // index of invalid digit in literal, or < 0

	// integer part
	if !seenPoint {
		if s.ch == '0' {
			s.nextch()
			switch lower(s.ch) {
			case 'x':
				s.nextch()
				base, prefix = 16, 'x'
			case 'o':
				s.nextch()
				base, prefix = 8, 'o'
			case 'b':
				s.nextch()
				base, prefix = 2, 'b'
			default:
				base, prefix = 8, '0'
				digsep = 1 // leading 0
			}
		}
		digsep |= s.digits(base, &invalid)
		if s.ch == '.' {
			// Check for .. range operator - don't consume the . if it's followed by another .
			if s.peek() == '.' {
				// This is the start of .. range operator, don't treat as decimal point
				goto done
			}
			if prefix == 'o' || prefix == 'b' {
				s.errorf("invalid radix point in %s literal", baseName(base))
				ok = false
			}
			s.nextch()
			seenPoint = true
		}
	}

done:

	// fractional part
	if seenPoint {
		kind = FloatLit
		digsep |= s.digits(base, &invalid)
	}

	if digsep&1 == 0 && ok {
		s.errorf("%s literal has no digits", baseName(base))
		ok = false
	}

	// exponent
	if e := lower(s.ch); e == 'e' || (e == 'p' && s.isValidExponentContext()) {
		if ok {
			switch {
			case e == 'e' && prefix != 0 && prefix != '0':
				s.errorf("%q exponent requires decimal mantissa", s.ch)
				ok = false
			case e == 'p' && prefix != 'x':
				s.errorf("%q exponent requires hexadecimal mantissa", s.ch)
				ok = false
			}
		}
		s.nextch()
		kind = FloatLit
		if s.ch == '+' || s.ch == '-' {
			s.nextch()
		}
		digsep = s.digits(10, nil) | digsep&2 // don't lose sep bit
		if digsep&1 == 0 && ok {
			s.errorf("exponent has no digits")
			ok = false
		}
	} else if prefix == 'x' && kind == FloatLit && ok {
		s.errorf("hexadecimal mantissa requires a 'p' exponent")
		ok = false
	}

	// Check for postfix operators first, then units, then imaginary numbers
	if s.transformsEnabled() && (s.ch == '²' || s.ch == '³') {
		// Standalone ² or ³ after a number should be treated as postfix operators, not units
		// Don't consume the character - let the main scanner loop handle it as _Superscript
		// This prevents the unit scanner from treating 3² as "3 * ²"
		s.implicitMul = false // Make sure we don't trigger implicit multiplication
	} else if s.transformsEnabled() && (isLetter(s.ch) || s.isUnicodeSymbol(s.ch)) {
		// Check if this might be a unit literal before checking for imaginary numbers
		if unitSuffix := s.scanPotentialUnit(); unitSuffix != "" {
			// Create unit literal: 500ms, 2km, 12inch, etc.
			kind = UnitLit
			s.lit = s.lit + unitSuffix
		} else if s.ch == 'i' {
			// Only treat as imaginary if it's a standalone 'i' (not part of a unit like 'inch')
			kind = ImagLit
			s.nextch()
		} else {
			// Implicit multiplication: 3x -> 3 * x
			// Don't consume the letter, just set implicit multiplication flag
			s.implicitMul = true
		}
	} else if s.ch == 'i' {
		// suffix 'i' for imaginary numbers (when transforms disabled)
		kind = ImagLit
		s.nextch()
	}

	s.setLit(kind, ok) // do this now so we can use s.lit below

	if kind == IntLit && invalid >= 0 && ok {
		s.errorAtf(invalid, "invalid digit %q in %s literal", s.lit[invalid], baseName(base))
		ok = false
	}

	if digsep&2 != 0 && ok {
		if i := invalidSep(s.lit); i >= 0 {
			s.errorAtf(i, "'_' must separate successive digits")
			ok = false
		}
	}

	s.bad = !ok // correct s.bad
}

func (s *scanner) scanPotentialUnit() string {
	// Check if the current position starts a valid unit suffix
	// Order by length (longest first) to match correctly  
	validUnits := []string{
		// Multi-character units (longest first)
		"BTU/h", "fl oz", "kcal", "keV", "MeV", "kWh", "mmHg", "torr", "grad", "turn",
		"MHz", "GHz", "THz", "kHz", "min", "rpm", "kPa", "MPa", "bar", "atm", "psi",
		"inch", "km", "cm", "mm", "ms", "μm", "nm", "pm", "fm", "in", "ft", "yd", "mi", "nmi",
		"AU", "ly", "pc", "μs", "ns", "ps", "wk", "mo", "yr", "kg", "mg", "μg", "lb", 
		"oz", "st", "°C", "°F", "°R", "kJ", "MJ", "cal", "BTU", "eV", "kW", "MW", "GW",
		"hp", "Pa", "Wb", "ha", "ac", "m²", "km²", "cm²", "mm²", "ft²", "in²", "m³",
		"mL", "gal", "qt", "pt", "cup", "bbl", "m/s", "km/h", "mph", "kn", "ft/s",
		"m/s²", "mps2", "gf", "rad", "deg", "sqm", "cbm",
		// Single character units  
		"Hz", "m", "s", "h", "d", "g", "t", "u", "K", "J", "W", "A", "V", "F", "H", "T", "L", "°",
	}
	
	// Check each unit pattern
	for _, unit := range validUnits {
		if s.matchesUnit(unit) {
			return unit
		}
	}
	
	return ""
}

// Helper function to check if current position matches a unit and consume it
func (s *scanner) matchesUnit(unit string) bool {
	// Save position
	savedR := s.r  
	savedCh := s.ch
	
	// Check if each character of the unit matches
	for _, r := range unit {
		if s.ch != r {
			// Restore position
			s.r = savedR
			s.ch = savedCh
			return false
		}
		s.nextch()
	}
	
	// For units with Unicode symbols (like °, ²), we need more permissive boundary detection
	// Check that this is the end of the identifier (no more ASCII letters after unit)
	// Allow Unicode symbols as terminal characters
	if isLetter(s.ch) && !s.isUnicodeSymbol(s.ch) {
		// Restore position
		s.r = savedR
		s.ch = savedCh
		return false
	}
	
	// Match found and characters already consumed
	return true
}

// Check if character is a Unicode symbol commonly used in units
func (s *scanner) isUnicodeSymbol(ch rune) bool {
	switch ch {
	case '²', '³', '°', 'μ', 'Ω', 'π', 'τ': // Common unit symbols
		return true
	default:
		return false
	}
}

// Check if 'p'/'P' should be treated as an exponent vs part of a unit
func (s *scanner) isValidExponentContext() bool {
	// Look ahead to see what follows 'P'
	r := s.r
	ch := s.ch
	s.nextch() // Move past 'P'
	
	// P should be treated as exponent if followed by:
	// 1. Digits (P10, P5)
	// 2. + or - followed by digits (P+10, P-5)
	isExponent := false
	if s.ch == '+' || s.ch == '-' {
		s.nextch()
	}
	if isDecimal(s.ch) {
		isExponent = true
	}
	
	// Restore position
	s.r = r
	s.ch = ch
	
	return isExponent
}

func baseName(base int) string {
	switch base {
	case 2:
		return "binary"
	case 8:
		return "octal"
	case 10:
		return "decimal"
	case 16:
		return "hexadecimal"
	}
	panic("invalid base")
}

// invalidSep returns the index of the first invalid separator in x, or -1.
func invalidSep(x string) int {
	x1 := ' ' // prefix char, we only care if it's 'x'
	d := '.'  // digit, one of '_', '0' (a digit), or '.' (anything else)
	i := 0

	// a prefix counts as a digit
	if len(x) >= 2 && x[0] == '0' {
		x1 = lower(rune(x[1]))
		if x1 == 'x' || x1 == 'o' || x1 == 'b' {
			d = '0'
			i = 2
		}
	}

	// mantissa and exponent
	for ; i < len(x); i++ {
		p := d // previous digit
		d = rune(x[i])
		switch {
		case d == '_':
			if p != '0' {
				return i
			}
		case isDecimal(d) || x1 == 'x' && isHex(d):
			d = '0'
		default:
			if p == '_' {
				return i - 1
			}
			d = '.'
		}
	}
	if d == '_' {
		return len(x) - 1
	}

	return -1
}

func (s *scanner) rune() {
	ok := true
	s.nextch()

	n := 0
	for ; ; n++ {
		if s.ch == '\'' {
			if ok {
				if n == 0 {
					s.errorf("empty rune literal or unescaped '")
					ok = false
				} else if n != 1 {
					s.errorAtf(0, "more than one character in rune literal")
					ok = false
				}
			}
			s.nextch()
			break
		}
		if s.ch == '\\' {
			s.nextch()
			if !s.escape('\'') {
				ok = false
			}
			continue
		}
		if s.ch == '\n' {
			if ok {
				s.errorf("newline in rune literal")
				ok = false
			}
			break
		}
		if s.ch < 0 {
			if ok {
				s.errorAtf(0, "rune literal not terminated")
				ok = false
			}
			break
		}
		s.nextch()
	}

	s.setLit(RuneLit, ok)
}

func (s *scanner) stdString() {
	ok := true
	s.nextch()

	for {
		if s.ch == '"' {
			s.nextch()
			break
		}
		if s.ch == '\\' {
			s.nextch()
			if !s.escape('"') {
				ok = false
			}
			continue
		}
		if s.ch == '\n' {
			s.errorf("newline in string")
			ok = false
			break
		}
		if s.ch < 0 {
			s.errorAtf(0, "string not terminated")
			ok = false
			break
		}
		s.nextch()
	}

	s.setLit(StringLit, ok)
}

func (s *scanner) rawString() {
	ok := true
	s.nextch()

	for {
		if s.ch == '`' {
			s.nextch()
			break
		}
		if s.ch < 0 {
			s.errorAtf(0, "string not terminated")
			ok = false
			break
		}
		s.nextch()
	}
	// We leave CRs in the string since they are part of the
	// literal (even though they are not part of the literal
	// value).

	s.setLit(StringLit, ok)
}

func (s *scanner) comment(text string) {
	s.errorAtf(0, "%s", text)
}

func (s *scanner) skipLine() {
	// don't consume '\n' - needed for nlsemi logic
	for s.ch >= 0 && s.ch != '\n' {
		s.nextch()
	}
}

func (s *scanner) lineComment() {
	// opening has already been consumed

	if s.mode&comments != 0 {
		s.skipLine()
		s.comment(string(s.segment()))
		return
	}

	// are we saving directives? or is this definitely not a directive?
	if s.mode&directives == 0 || (s.ch != 'g' && s.ch != 'l') {
		s.stop()
		s.skipLine()
		return
	}

	// recognize go: or line directives
	prefix := "go:"
	if s.ch == 'l' {
		prefix = "line "
	}
	for _, m := range prefix {
		if s.ch != m {
			s.stop()
			s.skipLine()
			return
		}
		s.nextch()
	}

	// directive text
	s.skipLine()
	s.comment(string(s.segment()))
}

func (s *scanner) skipComment() bool {
	for s.ch >= 0 {
		for s.ch == '*' {
			s.nextch()
			if s.ch == '/' {
				s.nextch()
				return true
			}
		}
		s.nextch()
	}
	s.errorAtf(0, "comment not terminated")
	return false
}

func (s *scanner) fullComment() {
	/* opening has already been consumed */

	if s.mode&comments != 0 {
		if s.skipComment() {
			s.comment(string(s.segment()))
		}
		return
	}

	if s.mode&directives == 0 || s.ch != 'l' {
		s.stop()
		s.skipComment()
		return
	}

	// recognize line directive
	const prefix = "line "
	for _, m := range prefix {
		if s.ch != m {
			s.stop()
			s.skipComment()
			return
		}
		s.nextch()
	}

	// directive text
	if s.skipComment() {
		s.comment(string(s.segment()))
	}
}

func (s *scanner) hashComment() {
	// # opening has already been consumed
	
	// Check for conditional compilation directives and handle them as special comments
	if s.checkHashDirective() {
		return
	}
	
	if s.mode&comments != 0 {
		s.skipLine()
		s.comment(string(s.segment()))
		return
	}

	// are we saving directives? Now hash comments can support conditional directives
	if s.mode&directives != 0 {
		s.skipLine()
		s.comment(string(s.segment()))
		return
	}
	
	s.stop()
	s.skipLine()
}

// checkHashDirective checks for conditional compilation directives like #if and #end
// Instead of creating new tokens, we handle these as special directive comments
func (s *scanner) checkHashDirective() bool {
	// Look ahead to see if this is "if" or "end" 
	if s.ch == 'i' && s.peekAhead("if") {
		s.skipLine()
		s.comment(string(s.segment()))
		return true
	} else if s.ch == 'e' && s.peekAhead("end") {
		s.skipLine()
		s.comment(string(s.segment()))
		return true
	}
	
	return false
}

// peek returns the next character without consuming it
func (s *scanner) peek() rune {
	savedR := s.r
	savedCh := s.ch
	s.nextch()
	nextCh := s.ch
	s.r = savedR
	s.ch = savedCh
	return nextCh
}

// peekAhead checks if the next characters match the given string
func (s *scanner) peekAhead(target string) bool {
	// Save current position
	savedR := s.r
	savedCh := s.ch

	// Try to match each character
	for _, expectedCh := range target {
		if s.ch != expectedCh {
			// Restore position and return false
			s.r = savedR
			s.ch = savedCh
			return false
		}
		s.nextch()
	}

	// Check that it's followed by whitespace or end of line
	if s.ch == ' ' || s.ch == '\t' || s.ch == '\n' || s.ch < 0 {
		// Restore position to start of matched text
		s.r = savedR
		s.ch = savedCh
		return true
	}

	// Restore position and return false
	s.r = savedR
	s.ch = savedCh
	return false
}

func (s *scanner) escape(quote rune) bool {
	var n int
	var base, maxi uint32

	switch s.ch {
	case quote, 'a', 'b', 'f', 'n', 'r', 't', 'v', '\\':
		s.nextch()
		return true
	case '0', '1', '2', '3', '4', '5', '6', '7':
		n, base, maxi = 3, 8, 255
	case 'x':
		s.nextch()
		n, base, maxi = 2, 16, 255
	case 'u':
		s.nextch()
		n, base, maxi = 4, 16, unicode.MaxRune
	case 'U':
		s.nextch()
		n, base, maxi = 8, 16, unicode.MaxRune
	default:
		if s.ch < 0 {
			return true // complain in caller about EOF
		}
		s.errorf("unknown escape")
		return false
	}

	var x uint32
	for i := n; i > 0; i-- {
		if s.ch < 0 {
			return true // complain in caller about EOF
		}
		d := base
		if isDecimal(s.ch) {
			d = uint32(s.ch) - '0'
		} else if 'a' <= lower(s.ch) && lower(s.ch) <= 'f' {
			d = uint32(lower(s.ch)) - 'a' + 10
		}
		if d >= base {
			s.errorf("invalid character %q in %s escape", s.ch, baseName(int(base)))
			return false
		}
		// d < base
		x = x*base + d
		s.nextch()
	}

	if x > maxi && base == 8 {
		s.errorf("octal escape value %d > 255", x)
		return false
	}

	if x > maxi || 0xD800 <= x && x < 0xE000 /* surrogate range */ {
		s.errorf("escape is invalid Unicode code point %#U", x)
		return false
	}

	return true
}

func (s *scanner) canBeIndexOperator() bool {
	// Check if # should be treated as a comment or index operator
	// Logic: if immediately preceded by whitespace -> comment
	//        if immediately preceded by non-whitespace -> index operator
	
	// If line is blank up to this point (only whitespace), definitely a comment
	if s.blank {
		return false
	}
	
	// Look back one character in the buffer to see what precedes the #
	// We need to go back from current position (which is at #)
	if s.r > 1 {
		prevChar := s.buf[s.r-2] // Go back 2 positions: current '#' and one before
		// If previous character is space or tab, treat as comment
		if prevChar == ' ' || prevChar == '\t' {
			return false
		}
	}
	
	return true // Default to index operator
}
