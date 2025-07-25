// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements typechecking of index/slice expressions.

package types

import (
	"go/ast"
	"go/constant"
	"go/token"
	. "internal/types/errors"
)

// If e is a valid function instantiation, indexExpr returns true.
// In that case x represents the uninstantiated function value and
// it is the caller's responsibility to instantiate the function.
func (checks *Checker) indexExpr(x *operand, e *indexedExpr) (isFuncInst bool) {
	checks.exprOrType(x, e.x, true)
	// x may be generic

	switch x.mode {
	case invalid:
		checks.use(e.indices...)
		return false

	case typexpr:
		// type instantiation
		x.mode = invalid
		// TODO(gri) here we re-evaluate e.X - try to avoid this
		x.typ = checks.varType(e.orig)
		if isValid(x.typ) {
			x.mode = typexpr
		}
		return false

	case value:
		if sig, _ := under(x.typ).(*Signature); sig != nil && sig.TypeParams().Len() > 0 {
			// function instantiation
			return true
		}
	}

	// x should not be generic at this point, but be safe and check
	checks.nonGeneric(nil, x)
	if x.mode == invalid {
		return false
	}

	// ordinary index expression
	valid := false
	length := int64(-1) // valid if >= 0
	switch typ := under(x.typ).(type) {
	case *Basic:
		if isString(typ) {
			valid = true
			if x.mode == constant_ {
				length = int64(len(constant.StringVal(x.val)))
			}
			// an indexed string always yields a byte value
			// (not a constant) even if the string and the
			// index are constant
			x.mode = value
			x.typ = universeByte // use 'byte' name
		}

	case *Array:
		valid = true
		length = typ.len
		if x.mode != variable {
			x.mode = value
		}
		x.typ = typ.elem

	case *Pointer:
		if typ, _ := under(typ.base).(*Array); typ != nil {
			valid = true
			length = typ.len
			x.mode = variable
			x.typ = typ.elem
		}

	case *Slice:
		valid = true
		x.mode = variable
		x.typ = typ.elem

	case *Map:
		index := checks.singleIndex(e)
		if index == nil {
			x.mode = invalid
			return false
		}
		var key operand
		checks.expr(nil, &key, index)
		checks.assignment(&key, typ.key, "map index")
		// ok to continue even if indexing failed - map element type is known
		x.mode = mapindex
		x.typ = typ.elem
		x.expr = e.orig
		return false

	case *Interface:
		if !isTypeParam(x.typ) {
			break
		}
		// TODO(gri) report detailed failure cause for better error messages
		var key, elem Type // key != nil: we must have all maps
		mode := variable   // non-maps result mode
		// TODO(gri) factor out closure and use it for non-typeparam cases as well
		if underIs(x.typ, func(u Type) bool {
			l := int64(-1) // valid if >= 0
			var k, e Type  // k is only set for maps
			switch t := u.(type) {
			case *Basic:
				if isString(t) {
					e = universeByte
					mode = value
				}
			case *Array:
				l = t.len
				e = t.elem
				if x.mode != variable {
					mode = value
				}
			case *Pointer:
				if t, _ := under(t.base).(*Array); t != nil {
					l = t.len
					e = t.elem
				}
			case *Slice:
				e = t.elem
			case *Map:
				k = t.key
				e = t.elem
			}
			if e == nil {
				return false
			}
			if elem == nil {
				// first type
				length = l
				key, elem = k, e
				return true
			}
			// all map keys must be identical (incl. all nil)
			// (that is, we cannot mix maps with other types)
			if !Identical(key, k) {
				return false
			}
			// all element types must be identical
			if !Identical(elem, e) {
				return false
			}
			// track the minimal length for arrays, if any
			if l >= 0 && l < length {
				length = l
			}
			return true
		}) {
			// For maps, the index expression must be assignable to the map key type.
			if key != nil {
				index := checks.singleIndex(e)
				if index == nil {
					x.mode = invalid
					return false
				}
				var k operand
				checks.expr(nil, &k, index)
				checks.assignment(&k, key, "map index")
				// ok to continue even if indexing failed - map element type is known
				x.mode = mapindex
				x.typ = elem
				x.expr = e.orig
				return false
			}

			// no maps
			valid = true
			x.mode = mode
			x.typ = elem
		}
	}

	if !valid {
		// types2 uses the position of '[' for the error
		checks.errorf(x, NonIndexableOperand, "cannot index %s", x)
		checks.use(e.indices...)
		x.mode = invalid
		return false
	}

	index := checks.singleIndex(e)
	if index == nil {
		x.mode = invalid
		return false
	}

	// In pathological (invalid) cases (e.g.: type T1 [][[]T1{}[0][0]]T0)
	// the element type may be accessed before it's set. Make sure we have
	// a valid type.
	if x.typ == nil {
		x.typ = Typ[Invalid]
	}

	checks.index(index, length)
	return false
}

func (checks *Checker) sliceExpr(x *operand, e *ast.SliceExpr) {
	checks.expr(nil, x, e.X)
	if x.mode == invalid {
		checks.use(e.Low, e.High, e.Max)
		return
	}

	// determine common underlying type cu
	var ct, cu Type // type and respective common underlying type
	var hasString bool
	typeset(x.typ, func(t, u Type) bool {
		if u == nil {
			checks.errorf(x, NonSliceableOperand, "cannot slice %s: no specific type in %s", x, x.typ)
			cu = nil
			return false
		}

		// Treat strings like byte slices but remember that we saw a string.
		if isString(u) {
			u = NewSlice(universeByte)
			hasString = true
		}

		// If this is the first type we're seeing, we're done.
		if cu == nil {
			ct, cu = t, u
			return true
		}

		// Otherwise, the current type must have the same underlying type as all previous types.
		if !Identical(cu, u) {
			checks.errorf(x, NonSliceableOperand, "cannot slice %s: %s and %s have different underlying types", x, ct, t)
			cu = nil
			return false
		}

		return true
	})
	if hasString {
		// If we saw a string, proceed with string type,
		// but don't go from untyped string to string.
		cu = Typ[String]
		if !isTypeParam(x.typ) {
			cu = under(x.typ) // untyped string remains untyped
		}
	}

	valid := false
	length := int64(-1) // valid if >= 0
	switch u := cu.(type) {
	case nil:
		// error reported above
		x.mode = invalid
		return

	case *Basic:
		if isString(u) {
			if e.Slice3 {
				at := e.Max
				if at == nil {
					at = e // e.Index[2] should be present but be careful
				}
				checks.error(at, InvalidSliceExpr, invalidOp+"3-index slice of string")
				x.mode = invalid
				return
			}
			valid = true
			if x.mode == constant_ {
				length = int64(len(constant.StringVal(x.val)))
			}
			// spec: "For untyped string operands the result
			// is a non-constant value of type string."
			if isUntyped(x.typ) {
				x.typ = Typ[String]
			}
		}

	case *Array:
		valid = true
		length = u.len
		if x.mode != variable {
			checks.errorf(x, NonSliceableOperand, "cannot slice unaddressable value %s", x)
			x.mode = invalid
			return
		}
		x.typ = &Slice{elem: u.elem}

	case *Pointer:
		if u, _ := under(u.base).(*Array); u != nil {
			valid = true
			length = u.len
			x.typ = &Slice{elem: u.elem}
		}

	case *Slice:
		valid = true
		// x.typ doesn't change
	}

	if !valid {
		checks.errorf(x, NonSliceableOperand, "cannot slice %s", x)
		x.mode = invalid
		return
	}

	x.mode = value

	// spec: "Only the first index may be omitted; it defaults to 0."
	if e.Slice3 && (e.High == nil || e.Max == nil) {
		checks.error(inNode(e, e.Rbrack), InvalidSyntaxTree, "2nd and 3rd index required in 3-index slice")
		x.mode = invalid
		return
	}

	// check indices
	var ind [3]int64
	for i, expr := range []ast.Expr{e.Low, e.High, e.Max} {
		x := int64(-1)
		switch {
		case expr != nil:
			// The "capacity" is only known statically for strings, arrays,
			// and pointers to arrays, and it is the same as the length for
			// those types.
			max := int64(-1)
			if length >= 0 {
				max = length + 1
			}
			if _, v := checks.index(expr, max); v >= 0 {
				x = v
			}
		case i == 0:
			// default is 0 for the first index
			x = 0
		case length >= 0:
			// default is length (== capacity) otherwise
			x = length
		}
		ind[i] = x
	}

	// constant indices must be in range
	// (check.index already checks that existing indices >= 0)
L:
	for i, x := range ind[:len(ind)-1] {
		if x > 0 {
			for j, y := range ind[i+1:] {
				if y >= 0 && y < x {
					// The value y corresponds to the expression e.Index[i+1+j].
					// Because y >= 0, it must have been set from the expression
					// when checking indices and thus e.Index[i+1+j] is not nil.
					at := []ast.Expr{e.Low, e.High, e.Max}[i+1+j]
					checks.errorf(at, SwappedSliceIndices, "invalid slice indices: %d < %d", y, x)
					break L // only report one error, ok to continue
				}
			}
		}
	}
}

// singleIndex returns the (single) index from the index expression e.
// If the index is missing, or if there are multiple indices, an error
// is reported and the result is nil.
func (checks *Checker) singleIndex(expr *indexedExpr) ast.Expr {
	if len(expr.indices) == 0 {
		checks.errorf(expr.orig, InvalidSyntaxTree, "index expression %v with 0 indices", expr)
		return nil
	}
	if len(expr.indices) > 1 {
		// TODO(rFindley) should this get a distinct error code?
		checks.error(expr.indices[1], InvalidIndex, invalidOp+"more than one index")
	}
	return expr.indices[0]
}

// index checks an index expression for validity.
// If max >= 0, it is the upper bound for index.
// If the result typ is != Typ[Invalid], index is valid and typ is its (possibly named) integer type.
// If the result val >= 0, index is valid and val is its constant int value.
func (checks *Checker) index(index ast.Expr, max int64) (typ Type, val int64) {
	typ = Typ[Invalid]
	val = -1

	var x operand
	checks.expr(nil, &x, index)
	if !checks.isValidIndex(&x, InvalidIndex, "index", false) {
		return
	}

	if x.mode != constant_ {
		return x.typ, -1
	}

	if x.val.Kind() == constant.Unknown {
		return
	}

	v, ok := constant.Int64Val(x.val)
	assert(ok)
	if max >= 0 && v >= max {
		checks.errorf(&x, InvalidIndex, invalidArg+"index %s out of bounds [0:%d]", x.val.String(), max)
		return
	}

	// 0 <= v [ && v < max ]
	return x.typ, v
}

func (checks *Checker) isValidIndex(x *operand, code Code, what string, allowNegative bool) bool {
	if x.mode == invalid {
		return false
	}

	// spec: "a constant index that is untyped is given type int"
	checks.convertUntyped(x, Typ[Int])
	if x.mode == invalid {
		return false
	}

	// spec: "the index x must be of integer type or an untyped constant"
	if !allInteger(x.typ) {
		checks.errorf(x, code, invalidArg+"%s %s must be integer", what, x)
		return false
	}

	if x.mode == constant_ {
		// spec: "a constant index must be non-negative ..."
		if !allowNegative && constant.Sign(x.val) < 0 {
			checks.errorf(x, code, invalidArg+"%s %s must not be negative", what, x)
			return false
		}

		// spec: "... and representable by a value of type int"
		if !representableConst(x.val, checks, Typ[Int], &x.val) {
			checks.errorf(x, code, invalidArg+"%s %s overflows int", what, x)
			return false
		}
	}

	return true
}

// indexedExpr wraps an ast.IndexExpr or ast.IndexListExpr.
//
// Orig holds the original ast.Expr from which this indexedExpr was derived.
//
// Note: indexedExpr (intentionally) does not wrap ast.Expr, as that leads to
// accidental misuse such as encountered in golang/go#63933.
//
// TODO(rfindley): remove this helper, in favor of just having a helper
// function that returns indices.
type indexedExpr struct {
	orig    ast.Expr   // the wrapped expr, which may be distinct from the IndexListExpr below.
	x       ast.Expr   // expression
	lbrack  token.Pos  // position of "["
	indices []ast.Expr // index expressions
	rbrack  token.Pos  // position of "]"
}

func (x *indexedExpr) Pos() token.Pos {
	return x.orig.Pos()
}

func unpackIndexedExpr(n ast.Node) *indexedExpr {
	switch e := n.(type) {
	case *ast.IndexExpr:
		return &indexedExpr{
			orig:    e,
			x:       e.X,
			lbrack:  e.Lbrack,
			indices: []ast.Expr{e.Index},
			rbrack:  e.Rbrack,
		}
	case *ast.IndexListExpr:
		return &indexedExpr{
			orig:    e,
			x:       e.X,
			lbrack:  e.Lbrack,
			indices: e.Indices,
			rbrack:  e.Rbrack,
		}
	}
	return nil
}
