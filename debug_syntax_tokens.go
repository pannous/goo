package main

import (
	"fmt"
	"unsafe"
)

// Duplicate the token definitions to debug
type token uint

const (
	_    token = iota
	_EOF

	_Name
	_Literal

	_Operator
	_AssignOp
	_IncOp
	_Assign
	_Define
	_Arrow
	_Star

	_Lparen
	_Lbrack
	_Lbrace
	_Rparen
	_Rbrack
	_Rbrace
	_Comma
	_Semi
	_Colon
	_Dot
	_DotDotDot
	_Hash

	_Break
	_Case
	_Chan
	_Check
	_Const
	_Continue
	_Default
	_Defer
	_Else
	_Fallthrough
	_For
	_Func
	_Go
	_Goto
	_If
	_Import
	_Interface
	_Map
	_Package
	_Range
	_Return
	_Select
	_Struct
	_Switch
	_Type
	_Var
	_Fn

	tokenCount
)

func main() {
	fmt.Printf("_Func = %d\n", _Func)
	fmt.Printf("_Var = %d\n", _Var) 
	fmt.Printf("_Fn = %d\n", _Fn)
	fmt.Printf("tokenCount = %d\n", tokenCount)
	fmt.Printf("Size check: 1 << (tokenCount - 1) = %d\n", 1 << (tokenCount - 1))
	fmt.Printf("Size of uint64: %d bits\n", unsafe.Sizeof(uint64(0)) * 8)
}