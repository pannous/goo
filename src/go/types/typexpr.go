// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements type-checking of identifiers and type expressions.

package types

import (
	"fmt"
	"go/ast"
	"go/constant"
	. "internal/types/errors"
	"strings"
)

// ident type-checks identifier e and initializes x with the value or type of e.
// If an error occurred, x.mode is set to invalid.
// For the meaning of def, see Checker.definedType, below.
// If wantType is set, the identifier e is expected to denote a type.
func (checks *Checker) ident(x *operand, e *ast.Ident, defi *TypeName, wantType bool) {
	x.mode = invalid
	x.expr = e

	scope, obj := checks.lookupScope(e.Name)
	switch obj {
	case nil:
		if e.Name == "_" {
			checks.error(e, InvalidBlank, "cannot use _ as value or type")
		} else if isValidName(e.Name) {
			checks.errorf(e, UndeclaredName, "undefined: %s", e.Name)
		}
		return
	case universeComparable:
		if !checks.verifyVersionf(e, go1_18, "predeclared %s", e.Name) {
			return // avoid follow-on errors
		}
	}
	// Because the representation of any depends on gotypesalias, we don't check
	// pointer identity here.
	if obj.Name() == "any" && obj.Parent() == Universe {
		if !checks.verifyVersionf(e, go1_18, "predeclared %s", e.Name) {
			return // avoid follow-on errors
		}
	}
	checks.recordUse(e, obj)

	// If we want a type but don't have one, stop right here and avoid potential problems
	// with missing underlying types. This also gives better error messages in some cases
	// (see go.dev/issue/65344).
	_, gotType := obj.(*TypeName)
	if !gotType && wantType {
		checks.errorf(e, NotAType, "%s is not a type", obj.Name())
		// avoid "declared but not used" errors
		// (don't use Checker.use - we don't want to evaluate too much)
		if v, _ := obj.(*Var); v != nil && v.pkg == checks.pkg /* see Checker.use1 */ {
			checks.usedVars[v] = true
		}
		return
	}

	// Type-check the object.
	// Only call Checker.objDecl if the object doesn't have a type yet
	// (in which case we must actually determine it) or the object is a
	// TypeName from the current package and we also want a type (in which case
	// we might detect a cycle which needs to be reported). Otherwise we can skip
	// the call and avoid a possible cycle error in favor of the more informative
	// "not a type/value" error that this function's caller will issue (see
	// go.dev/issue/25790).
	//
	// Note that it is important to avoid calling objDecl on objects from other
	// packages, to avoid races: see issue #69912.
	typ := obj.Type()
	if typ == nil || (gotType && wantType && obj.Pkg() == checks.pkg) {
		checks.objDecl(obj, defi)
		typ = obj.Type() // type must have been assigned by Checker.objDecl
	}
	assert(typ != nil)

	// The object may have been dot-imported.
	// If so, mark the respective package as used.
	// (This code is only needed for dot-imports. Without them,
	// we only have to mark variables, see *Var case below).
	if pkgName := checks.dotImportMap[dotImportKey{scope, obj.Name()}]; pkgName != nil {
		checks.usedPkgNames[pkgName] = true
	}

	switch obj := obj.(type) {
	case *PkgName:
		checks.errorf(e, InvalidPkgUse, "use of package %s not in selector", obj.name)
		return

	case *Const:
		checks.addDeclDep(obj)
		if !isValid(typ) {
			return
		}
		if obj == universeIota {
			if checks.iota == nil {
				checks.error(e, InvalidIota, "cannot use iota outside constant declaration")
				return
			}
			x.val = checks.iota
		} else {
			x.val = obj.val
		}
		assert(x.val != nil)
		x.mode = constant_

	case *TypeName:
		if !checks.conf._EnableAlias && checks.isBrokenAlias(obj) {
			checks.errorf(e, InvalidDeclCycle, "invalid use of type alias %s in recursive type (see go.dev/issue/50729)", obj.name)
			return
		}
		x.mode = typexpr

	case *Var:
		// It's ok to mark non-local variables, but ignore variables
		// from other packages to avoid potential race conditions with
		// dot-imported variables.
		if obj.pkg == checks.pkg {
			checks.usedVars[obj] = true
		}
		checks.addDeclDep(obj)
		if !isValid(typ) {
			return
		}
		x.mode = variable

	case *Func:
		checks.addDeclDep(obj)
		x.mode = value

	case *Builtin:
		x.id = obj.id
		x.mode = builtin

	case *Nil:
		x.mode = value

	default:
		panic("unreachable")
	}

	x.typ = typ
}

// typ type-checks the type expression e and returns its type, or Typ[Invalid].
// The type must not be an (uninstantiated) generic type.
func (checks *Checker) typ(e ast.Expr) Type {
	return checks.definedType(e, nil)
}

// varType type-checks the type expression e and returns its type, or Typ[Invalid].
// The type must not be an (uninstantiated) generic type and it must not be a
// constraint interface.
func (checks *Checker) varType(e ast.Expr) Type {
	typ := checks.definedType(e, nil)
	checks.validVarType(e, typ)
	return typ
}

// validVarType reports an error if typ is a constraint interface.
// The expression e is used for error reporting, if any.
func (checks *Checker) validVarType(e ast.Expr, typ Type) {
	// If we have a type parameter there's nothing to do.
	if isTypeParam(typ) {
		return
	}

	// We don't want to call under() or complete interfaces while we are in
	// the middle of type-checking parameter declarations that might belong
	// to interface methods. Delay this check to the end of type-checking.
	checks.later(func() {
		if t, _ := under(typ).(*Interface); t != nil {
			tset := computeInterfaceTypeSet(checks, e.Pos(), t) // TODO(gri) is this the correct position?
			if !tset.IsMethodSet() {
				if tset.comparable {
					checks.softErrorf(e, MisplacedConstraintIface, "cannot use type %s outside a type constraint: interface is (or embeds) comparable", typ)
				} else {
					checks.softErrorf(e, MisplacedConstraintIface, "cannot use type %s outside a type constraint: interface contains type constraints", typ)
				}
			}
		}
	}).describef(e, "check var type %s", typ)
}

// definedType is like typ but also accepts a type name def.
// If def != nil, e is the type specification for the type named def, declared
// in a type declaration, and def.typ.underlying will be set to the type of e
// before any components of e are type-checked.
func (checks *Checker) definedType(e ast.Expr, defi *TypeName) Type {
	typ := checks.typInternal(e, defi)
	assert(isTyped(typ))
	if isGeneric(typ) {
		checks.errorf(e, WrongTypeArgCount, "cannot use generic type %s without instantiation", typ)
		typ = Typ[Invalid]
	}
	checks.recordTypeAndValue(e, typexpr, typ, nil)
	return typ
}

// genericType is like typ but the type must be an (uninstantiated) generic
// type. If cause is non-nil and the type expression was a valid type but not
// generic, cause will be populated with a message describing the error.
//
// Note: If the type expression was invalid and an error was reported before,
// cause will not be populated; thus cause alone cannot be used to determine
// if an error occurred.
func (checks *Checker) genericType(e ast.Expr, cause *string) Type {
	typ := checks.typInternal(e, nil)
	assert(isTyped(typ))
	if isValid(typ) && !isGeneric(typ) {
		if cause != nil {
			*cause = checks.sprintf("%s is not a generic type", typ)
		}
		typ = Typ[Invalid]
	}
	// TODO(gri) what is the correct call below?
	checks.recordTypeAndValue(e, typexpr, typ, nil)
	return typ
}

// goTypeName returns the Go type name for typ and
// removes any occurrences of "types." from that name.
func goTypeName(typ Type) string {
	return strings.ReplaceAll(fmt.Sprintf("%T", typ), "types.", "")
}

// typInternal drives type checking of types.
// Must only be called by definedType or genericType.
func (checks *Checker) typInternal(e0 ast.Expr, defi *TypeName) (T Type) {
	if checks.conf._Trace {
		checks.trace(e0.Pos(), "-- type %s", e0)
		checks.indent++
		defer func() {
			checks.indent--
			var under Type
			if T != nil {
				// Calling under() here may lead to endless instantiations.
				// Test case: type T[P any] *T[P]
				under = safeUnderlying(T)
			}
			if T == under {
				checks.trace(e0.Pos(), "=> %s // %s", T, goTypeName(T))
			} else {
				checks.trace(e0.Pos(), "=> %s (under = %s) // %s", T, under, goTypeName(T))
			}
		}()
	}

	switch e := e0.(type) {
	case *ast.BadExpr:
		// ignore - error reported before

	case *ast.Ident:
		var x operand
		checks.ident(&x, e, defi, true)

		switch x.mode {
		case typexpr:
			typ := x.typ
			setDefType(defi, typ)
			return typ
		case invalid:
			// ignore - error reported before
		case novalue:
			checks.errorf(&x, NotAType, "%s used as type", &x)
		default:
			checks.errorf(&x, NotAType, "%s is not a type", &x)
		}

	case *ast.SelectorExpr:
		var x operand
		checks.selector(&x, e, defi, true)

		switch x.mode {
		case typexpr:
			typ := x.typ
			setDefType(defi, typ)
			return typ
		case invalid:
			// ignore - error reported before
		case novalue:
			checks.errorf(&x, NotAType, "%s used as type", &x)
		default:
			checks.errorf(&x, NotAType, "%s is not a type", &x)
		}

	case *ast.IndexExpr, *ast.IndexListExpr:
		ix := unpackIndexedExpr(e)
		checks.verifyVersionf(inNode(e, ix.lbrack), go1_18, "type instantiation")
		return checks.instantiatedType(ix, defi)

	case *ast.ParenExpr:
		// Generic types must be instantiated before they can be used in any form.
		// Consequently, generic types cannot be parenthesized.
		return checks.definedType(e.X, defi)

	case *ast.ArrayType:
		if e.Len == nil {
			typ := new(Slice)
			setDefType(defi, typ)
			typ.elem = checks.varType(e.Elt)
			return typ
		}

		typ := new(Array)
		setDefType(defi, typ)
		// Provide a more specific error when encountering a [...] array
		// rather than leaving it to the handling of the ... expression.
		if _, ok := e.Len.(*ast.Ellipsis); ok {
			checks.error(e.Len, BadDotDotDotSyntax, "invalid use of [...] array (outside a composite literal)")
			typ.len = -1
		} else {
			typ.len = checks.arrayLength(e.Len)
		}
		typ.elem = checks.varType(e.Elt)
		if typ.len >= 0 {
			return typ
		}
		// report error if we encountered [...]

	case *ast.Ellipsis:
		// dots are handled explicitly where they are valid
		checks.error(e, InvalidSyntaxTree, "invalid use of ...")

	case *ast.StructType:
		typ := new(Struct)
		setDefType(defi, typ)
		checks.structType(typ, e)
		return typ

	case *ast.StarExpr:
		typ := new(Pointer)
		typ.base = Typ[Invalid] // avoid nil base in invalid recursive type declaration
		setDefType(defi, typ)
		typ.base = checks.varType(e.X)
		// If typ.base is invalid, it's unlikely that *base is particularly
		// useful - even a valid dereferenciation will lead to an invalid
		// type again, and in some cases we get unexpected follow-on errors
		// (e.g., go.dev/issue/49005). Return an invalid type instead.
		if !isValid(typ.base) {
			return Typ[Invalid]
		}
		return typ

	case *ast.FuncType:
		typ := new(Signature)
		setDefType(defi, typ)
		checks.funcType(typ, nil, e)
		return typ

	case *ast.InterfaceType:
		typ := checks.newInterface()
		setDefType(defi, typ)
		checks.interfaceType(typ, e, defi)
		return typ

	case *ast.MapType:
		typ := new(Map)
		setDefType(defi, typ)

		typ.key = checks.varType(e.Key)
		typ.elem = checks.varType(e.Value)

		// spec: "The comparison operators == and != must be fully defined
		// for operands of the key type; thus the key type must not be a
		// function, map, or slice."
		//
		// Delay this check because it requires fully setup types;
		// it is safe to continue in any case (was go.dev/issue/6667).
		checks.later(func() {
			if !Comparable(typ.key) {
				var why string
				if isTypeParam(typ.key) {
					why = " (missing comparable constraint)"
				}
				checks.errorf(e.Key, IncomparableMapKey, "invalid map key type %s%s", typ.key, why)
			}
		}).describef(e.Key, "check map key %s", typ.key)

		return typ

	case *ast.ChanType:
		typ := new(Chan)
		setDefType(defi, typ)

		dir := SendRecv
		switch e.Dir {
		case ast.SEND | ast.RECV:
			// nothing to do
		case ast.SEND:
			dir = SendOnly
		case ast.RECV:
			dir = RecvOnly
		default:
			checks.errorf(e, InvalidSyntaxTree, "unknown channel direction %d", e.Dir)
			// ok to continue
		}

		typ.dir = dir
		typ.elem = checks.varType(e.Value)
		return typ

	default:
		checks.errorf(e0, NotAType, "%s is not a type", e0)
		checks.use(e0)
	}

	typ := Typ[Invalid]
	setDefType(defi, typ)
	return typ
}

func setDefType(defi *TypeName, typ Type) {
	if defi != nil {
		switch t := defi.typ.(type) {
		case *Alias:
			t.fromRHS = typ
		case *Basic:
			assert(t == Typ[Invalid])
		case *Named:
			t.underlying = typ
		default:
			panic(fmt.Sprintf("unexpected type %T", t))
		}
	}
}

func (checks *Checker) instantiatedType(ix *indexedExpr, defi *TypeName) (res Type) {
	if checks.conf._Trace {
		checks.trace(ix.Pos(), "-- instantiating type %s with %s", ix.x, ix.indices)
		checks.indent++
		defer func() {
			checks.indent--
			// Don't format the underlying here. It will always be nil.
			checks.trace(ix.Pos(), "=> %s", res)
		}()
	}

	defer func() {
		setDefType(defi, res)
	}()

	var cause string
	typ := checks.genericType(ix.x, &cause)
	if cause != "" {
		checks.errorf(ix.orig, NotAGenericType, invalidOp+"%s (%s)", ix.orig, cause)
	}
	if !isValid(typ) {
		return typ // error already reported
	}
	// typ must be a generic Alias or Named type (but not a *Signature)
	if _, ok := typ.(*Signature); ok {
		panic("unexpected generic signature")
	}
	gtyp := typ.(genericType)

	// evaluate arguments
	targs := checks.typeList(ix.indices)
	if targs == nil {
		return Typ[Invalid]
	}

	// create instance
	// The instance is not generic anymore as it has type arguments, but unless
	// instantiation failed, it still satisfies the genericType interface because
	// it has type parameters, too.
	ityp := checks.instance(ix.Pos(), gtyp, targs, nil, checks.context())
	inst, _ := ityp.(genericType)
	if inst == nil {
		return Typ[Invalid]
	}

	// For Named types, orig.tparams may not be set up, so we need to do expansion later.
	checks.later(func() {
		// This is an instance from the source, not from recursive substitution,
		// and so it must be resolved during type-checking so that we can report
		// errors.
		checks.recordInstance(ix.orig, targs, inst)

		name := inst.(interface{ Obj() *TypeName }).Obj().name
		tparams := inst.TypeParams().list()
		if checks.validateTArgLen(ix.Pos(), name, len(tparams), len(targs)) {
			// check type constraints
			if i, err := checks.verify(ix.Pos(), inst.TypeParams().list(), targs, checks.context()); err != nil {
				// best position for error reporting
				pos := ix.Pos()
				if i < len(ix.indices) {
					pos = ix.indices[i].Pos()
				}
				checks.softErrorf(atPos(pos), InvalidTypeArg, "%v", err)
			} else {
				checks.mono.recordInstance(checks.pkg, ix.Pos(), tparams, targs, ix.indices)
			}
		}
	}).describef(ix, "verify instantiation %s", inst)

	return inst
}

// arrayLength type-checks the array length expression e
// and returns the constant length >= 0, or a value < 0
// to indicate an error (and thus an unknown length).
func (checks *Checker) arrayLength(e ast.Expr) int64 {
	// If e is an identifier, the array declaration might be an
	// attempt at a parameterized type declaration with missing
	// constraint. Provide an error message that mentions array
	// length.
	if name, _ := e.(*ast.Ident); name != nil {
		obj := checks.lookup(name.Name)
		if obj == nil {
			checks.errorf(name, InvalidArrayLen, "undefined array length %s or missing type constraint", name.Name)
			return -1
		}
		if _, ok := obj.(*Const); !ok {
			checks.errorf(name, InvalidArrayLen, "invalid array length %s", name.Name)
			return -1
		}
	}

	var x operand
	checks.expr(nil, &x, e)
	if x.mode != constant_ {
		if x.mode != invalid {
			checks.errorf(&x, InvalidArrayLen, "array length %s must be constant", &x)
		}
		return -1
	}

	if isUntyped(x.typ) || isInteger(x.typ) {
		if val := constant.ToInt(x.val); val.Kind() == constant.Int {
			if representableConst(val, checks, Typ[Int], nil) {
				if n, ok := constant.Int64Val(val); ok && n >= 0 {
					return n
				}
			}
		}
	}

	var msg string
	if isInteger(x.typ) {
		msg = "invalid array length %s"
	} else {
		msg = "array length %s must be integer"
	}
	checks.errorf(&x, InvalidArrayLen, msg, &x)
	return -1
}

// typeList provides the list of types corresponding to the incoming expression list.
// If an error occurred, the result is nil, but all list elements were type-checked.
func (checks *Checker) typeList(list []ast.Expr) []Type {
	res := make([]Type, len(list)) // res != nil even if len(list) == 0
	for i, x := range list {
		t := checks.varType(x)
		if !isValid(t) {
			res = nil
		}
		if res != nil {
			res[i] = t
		}
	}
	return res
}
