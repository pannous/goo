// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements recording of type information
// in the types2.Info maps.

package types2

import (
	"cmd/compile/internal/syntax"
	"go/constant"
)

func (checks *Checker) record(x *operand) {
	// convert x into a user-friendly set of values
	// TODO(gri) this code can be simplified
	var typ Type
	var val constant.Value
	switch x.mode {
	case invalid:
		typ = Typ[Invalid]
	case novalue:
		typ = (*Tuple)(nil)
	case constant_:
		typ = x.typ
		val = x.val
	default:
		typ = x.typ
	}
	assert(x.expr != nil && typ != nil)

	if isUntyped(typ) {
		// delay type and value recording until we know the type
		// or until the end of type checking
		checks.rememberUntyped(x.expr, false, x.mode, typ.(*Basic), val)
	} else {
		checks.recordTypeAndValue(x.expr, x.mode, typ, val)
	}
}

func (checks *Checker) recordUntyped() {
	if !debug && !checks.recordTypes() {
		return // nothing to do
	}

	for x, info := range checks.untyped {
		if debug && isTyped(info.typ) {
			checks.dump("%v: %s (type %s) is typed", atPos(x), x, info.typ)
			panic("unreachable")
		}
		checks.recordTypeAndValue(x, info.mode, info.typ, info.val)
	}
}

func (checks *Checker) recordTypeAndValue(x syntax.Expr, mode operandMode, typ Type, val constant.Value) {
	assert(x != nil)
	assert(typ != nil)
	if mode == invalid {
		return // omit
	}
	if mode == constant_ {
		assert(val != nil)
		// We check allBasic(typ, IsConstType) here as constant expressions may be
		// recorded as type parameters.
		assert(!isValid(typ) || allBasic(typ, IsConstType))
	}
	if m := checks.Types; m != nil {
		m[x] = TypeAndValue{mode, typ, val}
	}
	checks.recordTypeAndValueInSyntax(x, mode, typ, val)
}

func (checks *Checker) recordBuiltinType(f syntax.Expr, sig *Signature) {
	// f must be a (possibly parenthesized, possibly qualified)
	// identifier denoting a built-in (including unsafe's non-constant
	// functions Add and Slice): record the signature for f and possible
	// children.
	for {
		checks.recordTypeAndValue(f, builtin, sig, nil)
		switch p := f.(type) {
		case *syntax.Name, *syntax.SelectorExpr:
			return // we're done
		case *syntax.ParenExpr:
			f = p.X
		default:
			panic("unreachable")
		}
	}
}

// recordCommaOkTypes updates recorded types to reflect that x is used in a commaOk context
// (and therefore has tuple type).
func (checks *Checker) recordCommaOkTypes(x syntax.Expr, a []*operand) {
	assert(x != nil)
	assert(len(a) == 2)
	if a[0].mode == invalid {
		return
	}
	t0, t1 := a[0].typ, a[1].typ
	assert(isTyped(t0) && isTyped(t1) && (allBoolean(t1) || t1 == universeError))
	if m := checks.Types; m != nil {
		for {
			tv := m[x]
			assert(tv.Type != nil) // should have been recorded already
			pos := x.Pos()
			tv.Type = NewTuple(
				newVar(LocalVar, pos, checks.pkg, "", t0),
				newVar(LocalVar, pos, checks.pkg, "", t1),
			)
			m[x] = tv
			// if x is a parenthesized expression (p.X), update p.X
			p, _ := x.(*syntax.ParenExpr)
			if p == nil {
				break
			}
			x = p.X
		}
	}
	checks.recordCommaOkTypesInSyntax(x, t0, t1)
}

// recordInstance records instantiation information into check.Info, if the
// Instances map is non-nil. The given expr must be an ident, selector, or
// index (list) expr with ident or selector operand.
//
// TODO(rfindley): the expr parameter is fragile. See if we can access the
// instantiated identifier in some other way.
func (checks *Checker) recordInstance(expr syntax.Expr, targs []Type, typ Type) {
	ident := instantiatedIdent(expr)
	assert(ident != nil)
	assert(typ != nil)
	if m := checks.Instances; m != nil {
		m[ident] = Instance{newTypeList(targs), typ}
	}
}

func (checks *Checker) recordDef(id *syntax.Name, obj Object) {
	assert(id != nil)
	if m := checks.Defs; m != nil {
		m[id] = obj
	}
}

func (checks *Checker) recordUse(id *syntax.Name, obj Object) {
	assert(id != nil)
	assert(obj != nil)
	if m := checks.Uses; m != nil {
		m[id] = obj
	}
}

func (checks *Checker) recordImplicit(node syntax.Node, obj Object) {
	assert(node != nil)
	assert(obj != nil)
	if m := checks.Implicits; m != nil {
		m[node] = obj
	}
}

func (checks *Checker) recordSelection(x *syntax.SelectorExpr, kind SelectionKind, recv Type, obj Object, index []int, indirect bool) {
	assert(obj != nil && (recv == nil || len(index) > 0))
	checks.recordUse(x.Sel, obj)
	if m := checks.Selections; m != nil {
		m[x] = &Selection{kind, recv, obj, index, indirect}
	}
}

func (checks *Checker) recordScope(node syntax.Node, scope *Scope) {
	assert(node != nil)
	assert(scope != nil)
	if m := checks.Scopes; m != nil {
		m[node] = scope
	}
}
