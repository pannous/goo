// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

import "strconv"

type Token uint

type token = Token

const (
	_    token = iota
	_EOF       // EOF

	// names and literals
	_Name    // name
	_Literal // literal

	// operators and operations
	// _Operator is excluding '*' (_Star)
	_Operator // op
	_AssignOp // op=
	_IncOp    // opop
	_Assign   // =
	_Define   // :=
	_Arrow    // <-
	_Star     // *
	_Lambda   // =>
	_Superscript // ² ³
	_Power    // **

	// delimiters
	_Lparen    // (
	_Lbrack    // [
	_Lbrace    // {
	_Rparen    // )
	_Rbrack    // ]
	_Rbrace    // }
	_Comma     // ,
	_Semi      // ;
	_Colon     // :
	_Dot       // .
	_DotDot    // ..
	_DotDotDot // ...
	// … .. ..? ..<

	// keywords
	_keywords_start
	_Break          // break
	_Case           // case
	_Chan           // chan
	_Const          // const
	_Continue       // continue
	_Default        // default
	_Defer          // defer
	_Else           // else
	_Fallthrough    // fallthrough
	_For            // for
	_Func           // func
	_Go             // go
	_Goto           // goto (wtf)
	_If             // if
	_Import         // import
	_Interface      // interface
	_Map            // map
	_Package        // package
	_Range          // range
	_Return         // return
	_Select         // select
	_Struct         // struct
	_Switch         // switch
	_Type           // type
	_Var            // var
	_CUSTOM_TOKENS_ // the following tokens are only for .goo files:
	_And            // and (truthy and operator)
	_As             // as cast
	_Enum           // enum
	_Catch          // catch + try
	_Class          // class
	_Check          // check ≈ assert
	_Def            // def
	_Hash           // # as comment or index
	_Try            // try
	_Void           // void
	_In             // in
	_IS             // is (type checking operator)
	_While          // while

	// empty line comment to exclude it from .String
	tokenCount //
)

const (
	// for BranchStmt
	Break       = _Break
	Continue    = _Continue
	Fallthrough = _Fallthrough
	Goto        = _Goto

	// for CallStmt
	Go    = _Go
	Defer = _Defer
)

// Note: We have exceeded 64 tokens, so token sets need to use a larger bit field if needed.

// contains reports whether tok is in tokset.
func contains(tokset uint64, tok token) bool {
	return tokset&(1<<tok) != 0
}

// TokenNames provides string representations for tokens, indexed by token value.
// This replaces the fragile stringer-generated token_string.go file.
// Tokens start at 1, so index 0 is unused.
var TokenNames = [...]string{
	0:               "", // unused (token 0)
	_EOF:            "EOF",
	_Name:           "name",
	_Literal:        "literal",
	_Operator:       "op",
	_AssignOp:       "op=",
	_IncOp:          "opop",
	_Assign:         "=",
	_Define:         ":=",
	_Arrow:          "<-",
	_Lambda:         "=>",
	_Star:           "*",
	_Superscript:    "²",
	_Power:          "**",
	_Lparen:         "(",
	_Lbrack:         "[",
	_Lbrace:         "{",
	_Rparen:         ")",
	_Rbrack:         "]",
	_Rbrace:         "}",
	_Comma:          ",",
	_Semi:           ";",
	_Colon:          ":",
	_Dot:            ".",
	_DotDot:         "..",
	_DotDotDot:      "...",
	_As:             "as",
	_Break:          "break",
	_Case:           "case",
	_Chan:           "chan",
	_Check:          "check",
	_Def:            "def",
	_Const:          "const",
	_Enum:           "enum",
	_Continue:       "continue",
	_Default:        "default",
	_Defer:          "defer",
	_Else:           "else",
	_Fallthrough:    "fallthrough",
	_For:            "for",
	_Func:           "func",
	_Go:             "go",
	_Goto:           "goto",
	_If:             "if",
	_Import:         "import",
	_Interface:      "interface",
	_Map:            "map",
	_Package:        "package",
	_Range:          "range",
	_Return:         "return",
	_Select:         "select",
	_Struct:         "struct",
	_Switch:         "switch",
	_Type:           "type",
	_Var:            "var",
	_Class:          "class",
	_Try:            "try",
	_Catch:          "catch",
	_Void:           "void",
	_Hash:           "#",
	_In:             "in",
	_While:          "while",
	_CUSTOM_TOKENS_: "avoid_hash_collision",
}

// String returns the string representation of the token.

type LitKind uint8

// TODO(gri) With the 'i' (imaginary) suffix now permitted on integer
// and floating-point numbers, having a single ImagLit does
// not represent the literal kind well anymore. Remove it?
const (
	IntLit LitKind = iota
	FloatLit
	ImagLit
	RuneLit
	StringLit
	UnitLit
)

type Operator uint

const (
	_ Operator = iota

	// Def is the : in :=
	Def   // :
	Not   // !
	Recv  // <-
	Tilde // ~

	// precOrOr
	OrOr       // ||
	NullCoalesce // ??

	// precAndAnd
	TruthyAnd // and (truthy and)
	AndAnd    // &&

	// precCmp
	Eql // ==
	Neq // !=
	Lss // <
	Leq // <=
	Gtr // >
	Geq // >=

	// precAdd
	Add // +
	Sub // -
	Or  // |
	Xor // ^
	In  // in

	// precMul
	Mul       // *
	MiddleDot // ·
	Div       // /
	Rem       // %
	And       // &
	AndNot    // &^
	Shl       // <<
	Shr       // >>
	IS        // is
	Range     // …
	
	// precPower  
	Power     // **
)

// OperatorNames provides string representations for Operator constants, replacing stringer-generated operator_string.go
var OperatorNames = [...]string{
	0:      "", // unused (Operator 0)
	Def:    ":",
	Not:    "!",
	Recv:   "<-",
	Tilde:       "~",
	OrOr:        "||",
	NullCoalesce: "??",
	TruthyAnd:   "and",
	AndAnd:      "&&",
	Eql:    "==",
	Neq:    "!=",
	Lss:    "<",
	Leq:    "<=",
	Gtr:    ">",
	Geq:    ">=",
	In:     "in",
	Add:    "+",
	Sub:    "-",
	Or:     "|",
	Xor:    "^",
	Mul:       "*",
	MiddleDot: "·",
	Div:       "/",
	Rem:    "%",
	And:    "&",
	AndNot: "&^",
	Shl:    "<<",
	Shr:    ">>",
	IS:     "is",
	Range:  "…",
	Power:  "**",
}

// String returns the string representation of the Operator.
// This replaces the stringer-generated String() method.
func (op Operator) String() string {
	if int(op) < len(OperatorNames) && OperatorNames[op] != "" {
		return OperatorNames[op]
	}
	return "Operator(" + strconv.FormatInt(int64(op), 10) + ")"
}

// Operator precedences
const (
	_ = iota
	precOrOr
	precAndAnd
	precCmp
	precAdd
	precMul
	precPower
)
