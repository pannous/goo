// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements typechecking of builtin function calls.

package types2

import (
	"cmd/compile/internal/syntax"
	"go/constant"
	"go/token"
	. "internal/types/errors"
)

// builtin type-checks a call to the built-in specified by id and
// reports whether the call is valid, with *x holding the result;
// but x.expr is not set. If the call is invalid, the result is
// false, and *x is undefined.
func (checks *Checker) builtin(x *operand, call *syntax.CallExpr, id builtinId) (_ bool) {
	argList := call.ArgList

	// append is the only built-in that permits the use of ... for the last argument
	bin := predeclaredFuncs[id]
	if hasDots(call) && id != _Append {
		checks.errorf(dddErrPos(call),
			InvalidDotDotDot,
			invalidOp+"invalid use of ... with built-in %s", bin.name)
		checks.use(argList...)
		return
	}

	// For len(x) and cap(x) we need to know if x contains any function calls or
	// receive operations. Save/restore current setting and set hasCallOrRecv to
	// false for the evaluation of x so that we can check it afterwards.
	// Note: We must do this _before_ calling exprList because exprList evaluates
	//       all arguments.
	if id == _Len || id == _Cap {
		defer func(b bool) {
			checks.hasCallOrRecv = b
		}(checks.hasCallOrRecv)
		checks.hasCallOrRecv = false
	}

	// Evaluate arguments for built-ins that use ordinary (value) arguments.
	// For built-ins with special argument handling (make, new, etc.),
	// evaluation is done by the respective built-in code.
	var args []*operand // not valid for _Make, _New, _Offsetof, _Trace
	var nargs int
	switch id {
	default:
		// check all arguments
		args = checks.exprList(argList)
		nargs = len(args)
		for _, a := range args {
			if a.mode == invalid {
				return
			}
		}
		// first argument is always in x
		if nargs > 0 {
			*x = *args[0]
		}
	case _Make, _New, _Offsetof, _Trace:
		// arguments require special handling
		nargs = len(argList)
	}

	// check argument count
	{
		msg := ""
		if nargs < bin.nargs {
			msg = "not enough"
		} else if !bin.variadic && nargs > bin.nargs {
			msg = "too many"
		}
		if msg != "" {
			checks.errorf(argErrPos(call), WrongArgCount, invalidOp+"%s arguments for %v (expected %d, found %d)", msg, call, bin.nargs, nargs)
			return
		}
	}

	switch id {
	case _Append:
		// append(s S, x ...E) S, where E is the element type of S
		// spec: "The variadic function append appends zero or more values x to
		// a slice s of type S and returns the resulting slice, also of type S.
		// The values x are passed to a parameter of type ...E where E is the
		// element type of S and the respective parameter passing rules apply.
		// As a special case, append also accepts a first argument assignable
		// to type []byte with a second argument of string type followed by ... .
		// This form appends the bytes of the string."

		// Handle append(bytes, y...) special case, where
		// the type set of y is {string} or {string, []byte}.
		var sig *Signature
		if nargs == 2 && hasDots(call) {
			if ok, _ := x.assignableTo(checks, NewSlice(universeByte), nil); ok {
				y := args[1]
				hasString := false
				typeset(y.typ, func(_, u Type) bool {
					if s, _ := u.(*Slice); s != nil && Identical(s.elem, universeByte) {
						return true
					}
					if isString(u) {
						hasString = true
						return true
					}
					y = nil
					return false
				})
				if y != nil && hasString {
					// setting the signature also signals that we're done
					sig = makeSig(x.typ, x.typ, y.typ)
					sig.variadic = true
				}
			}
		}

		// general case
		if sig == nil {
			// spec: "If S is a type parameter, all types in its type set
			// must have the same underlying slice type []E."
			E, err := sliceElem(x)
			if err != nil {
				checks.errorf(x, InvalidAppend, "invalid append: %s", err.format(checks))
				return
			}
			// check arguments by creating custom signature
			sig = makeSig(x.typ, x.typ, NewSlice(E)) // []E required for variadic signature
			sig.variadic = true
			checks.arguments(call, sig, nil, nil, args, nil) // discard result (we know the result type)
			// ok to continue even if check.arguments reported errors
		}

		if checks.recordTypes() {
			checks.recordBuiltinType(call.Fun, sig)
		}
		x.mode = value
		// x.typ is unchanged

	case _Cap, _Len:
		// cap(x)
		// len(x)
		mode := invalid
		var val constant.Value
		switch t := arrayPtrDeref(under(x.typ)).(type) {
		case *Basic:
			if isString(t) && id == _Len {
				if x.mode == constant_ {
					mode = constant_
					val = constant.MakeInt64(int64(len(constant.StringVal(x.val))))
				} else {
					mode = value
				}
			}

		case *Array:
			mode = value
			// spec: "The expressions len(s) and cap(s) are constants
			// if the type of s is an array or pointer to an array and
			// the expression s does not contain channel receives or
			// function calls; in this case s is not evaluated."
			if !checks.hasCallOrRecv {
				mode = constant_
				if t.len >= 0 {
					val = constant.MakeInt64(t.len)
				} else {
					val = constant.MakeUnknown()
				}
			}

		case *Slice, *Chan:
			mode = value

		case *Map:
			if id == _Len {
				mode = value
			}

		case *Interface:
			if !isTypeParam(x.typ) {
				break
			}
			if underIs(x.typ, func(u Type) bool {
				switch t := arrayPtrDeref(u).(type) {
				case *Basic:
					if isString(t) && id == _Len {
						return true
					}
				case *Array, *Slice, *Chan:
					return true
				case *Map:
					if id == _Len {
						return true
					}
				}
				return false
			}) {
				mode = value
			}
		}

		if mode == invalid {
			// avoid error if underlying type is invalid
			if isValid(under(x.typ)) {
				code := InvalidCap
				if id == _Len {
					code = InvalidLen
				}
				checks.errorf(x, code, invalidArg+"%s for built-in %s", x, bin.name)
			}
			return
		}

		// record the signature before changing x.typ
		if checks.recordTypes() && mode != constant_ {
			checks.recordBuiltinType(call.Fun, makeSig(Typ[Int], x.typ))
		}

		x.mode = mode
		x.typ = Typ[Int]
		x.val = val

	case _Clear:
		// clear(m)
		checks.verifyVersionf(call.Fun, go1_21, "clear")

		if !underIs(x.typ, func(u Type) bool {
			switch u.(type) {
			case *Map, *Slice:
				return true
			}
			checks.errorf(x, InvalidClear, invalidArg+"cannot clear %s: argument must be (or constrained by) map or slice", x)
			return false
		}) {
			return
		}

		x.mode = novalue
		if checks.recordTypes() {
			checks.recordBuiltinType(call.Fun, makeSig(nil, x.typ))
		}

	case _Close:
		// close(c)
		if !underIs(x.typ, func(u Type) bool {
			uch, _ := u.(*Chan)
			if uch == nil {
				checks.errorf(x, InvalidClose, invalidOp+"cannot close non-channel %s", x)
				return false
			}
			if uch.dir == RecvOnly {
				checks.errorf(x, InvalidClose, invalidOp+"cannot close receive-only channel %s", x)
				return false
			}
			return true
		}) {
			return
		}
		x.mode = novalue
		if checks.recordTypes() {
			checks.recordBuiltinType(call.Fun, makeSig(nil, x.typ))
		}

	case _Complex:
		// complex(x, y floatT) complexT
		y := args[1]

		// convert or check untyped arguments
		d := 0
		if isUntyped(x.typ) {
			d |= 1
		}
		if isUntyped(y.typ) {
			d |= 2
		}
		switch d {
		case 0:
			// x and y are typed => nothing to do
		case 1:
			// only x is untyped => convert to type of y
			checks.convertUntyped(x, y.typ)
		case 2:
			// only y is untyped => convert to type of x
			checks.convertUntyped(y, x.typ)
		case 3:
			// x and y are untyped =>
			// 1) if both are constants, convert them to untyped
			//    floating-point numbers if possible,
			// 2) if one of them is not constant (possible because
			//    it contains a shift that is yet untyped), convert
			//    both of them to float64 since they must have the
			//    same type to succeed (this will result in an error
			//    because shifts of floats are not permitted)
			if x.mode == constant_ && y.mode == constant_ {
				toFloat := func(x *operand) {
					if isNumeric(x.typ) && constant.Sign(constant.Imag(x.val)) == 0 {
						x.typ = Typ[UntypedFloat]
					}
				}
				toFloat(x)
				toFloat(y)
			} else {
				checks.convertUntyped(x, Typ[Float64])
				checks.convertUntyped(y, Typ[Float64])
				// x and y should be invalid now, but be conservative
				// and check below
			}
		}
		if x.mode == invalid || y.mode == invalid {
			return
		}

		// both argument types must be identical
		if !Identical(x.typ, y.typ) {
			checks.errorf(x, InvalidComplex, invalidOp+"%v (mismatched types %s and %s)", call, x.typ, y.typ)
			return
		}

		// the argument types must be of floating-point type
		// (applyTypeFunc never calls f with a type parameter)
		f := func(typ Type) Type {
			assert(!isTypeParam(typ))
			if t, _ := under(typ).(*Basic); t != nil {
				switch t.kind {
				case Float32:
					return Typ[Complex64]
				case Float64:
					return Typ[Complex128]
				case UntypedFloat:
					return Typ[UntypedComplex]
				}
			}
			return nil
		}
		resTyp := checks.applyTypeFunc(f, x, id)
		if resTyp == nil {
			checks.errorf(x, InvalidComplex, invalidArg+"arguments have type %s, expected floating-point", x.typ)
			return
		}

		// if both arguments are constants, the result is a constant
		if x.mode == constant_ && y.mode == constant_ {
			x.val = constant.BinaryOp(constant.ToFloat(x.val), token.ADD, constant.MakeImag(constant.ToFloat(y.val)))
		} else {
			x.mode = value
		}

		if checks.recordTypes() && x.mode != constant_ {
			checks.recordBuiltinType(call.Fun, makeSig(resTyp, x.typ, x.typ))
		}

		x.typ = resTyp

	case _Copy:
		// copy(x, y []E) int
		// spec: "The function copy copies slice elements from a source src to a destination
		// dst and returns the number of elements copied. Both arguments must have identical
		// element type E and must be assignable to a slice of type []E.
		// The number of elements copied is the minimum of len(src) and len(dst).
		// As a special case, copy also accepts a destination argument assignable to type
		// []byte with a source argument of a string type.
		// This form copies the bytes from the string into the byte slice."

		// get special case out of the way
		y := args[1]
		var special bool
		if ok, _ := x.assignableTo(checks, NewSlice(universeByte), nil); ok {
			special = true
			typeset(y.typ, func(_, u Type) bool {
				if s, _ := u.(*Slice); s != nil && Identical(s.elem, universeByte) {
					return true
				}
				if isString(u) {
					return true
				}
				special = false
				return false
			})
		}

		// general case
		if !special {
			// spec: "If the type of one or both arguments is a type parameter, all types
			// in their respective type sets must have the same underlying slice type []E."
			dstE, err := sliceElem(x)
			if err != nil {
				checks.errorf(x, InvalidCopy, "invalid copy: %s", err.format(checks))
				return
			}
			srcE, err := sliceElem(y)
			if err != nil {
				// If we have a string, for a better error message proceed with byte element type.
				if !allString(y.typ) {
					checks.errorf(y, InvalidCopy, "invalid copy: %s", err.format(checks))
					return
				}
				srcE = universeByte
			}
			if !Identical(dstE, srcE) {
				checks.errorf(x, InvalidCopy, "invalid copy: arguments %s and %s have different element types %s and %s", x, y, dstE, srcE)
				return
			}
		}

		if checks.recordTypes() {
			checks.recordBuiltinType(call.Fun, makeSig(Typ[Int], x.typ, y.typ))
		}
		x.mode = value
		x.typ = Typ[Int]

	case _Delete:
		// delete(map_, key)
		// map_ must be a map type or a type parameter describing map types.
		// The key cannot be a type parameter for now.
		map_ := x.typ
		var key Type
		if !underIs(map_, func(u Type) bool {
			map_, _ := u.(*Map)
			if map_ == nil {
				checks.errorf(x, InvalidDelete, invalidArg+"%s is not a map", x)
				return false
			}
			if key != nil && !Identical(map_.key, key) {
				checks.errorf(x, InvalidDelete, invalidArg+"maps of %s must have identical key types", x)
				return false
			}
			key = map_.key
			return true
		}) {
			return
		}

		*x = *args[1] // key
		checks.assignment(x, key, "argument to delete")
		if x.mode == invalid {
			return
		}

		x.mode = novalue
		if checks.recordTypes() {
			checks.recordBuiltinType(call.Fun, makeSig(nil, map_, key))
		}

	case _Imag, _Real:
		// imag(complexT) floatT
		// real(complexT) floatT

		// convert or check untyped argument
		if isUntyped(x.typ) {
			if x.mode == constant_ {
				// an untyped constant number can always be considered
				// as a complex constant
				if isNumeric(x.typ) {
					x.typ = Typ[UntypedComplex]
				}
			} else {
				// an untyped non-constant argument may appear if
				// it contains a (yet untyped non-constant) shift
				// expression: convert it to complex128 which will
				// result in an error (shift of complex value)
				checks.convertUntyped(x, Typ[Complex128])
				// x should be invalid now, but be conservative and check
				if x.mode == invalid {
					return
				}
			}
		}

		// the argument must be of complex type
		// (applyTypeFunc never calls f with a type parameter)
		f := func(typ Type) Type {
			assert(!isTypeParam(typ))
			if t, _ := under(typ).(*Basic); t != nil {
				switch t.kind {
				case Complex64:
					return Typ[Float32]
				case Complex128:
					return Typ[Float64]
				case UntypedComplex:
					return Typ[UntypedFloat]
				}
			}
			return nil
		}
		resTyp := checks.applyTypeFunc(f, x, id)
		if resTyp == nil {
			code := InvalidImag
			if id == _Real {
				code = InvalidReal
			}
			checks.errorf(x, code, invalidArg+"argument has type %s, expected complex type", x.typ)
			return
		}

		// if the argument is a constant, the result is a constant
		if x.mode == constant_ {
			if id == _Real {
				x.val = constant.Real(x.val)
			} else {
				x.val = constant.Imag(x.val)
			}
		} else {
			x.mode = value
		}

		if checks.recordTypes() && x.mode != constant_ {
			checks.recordBuiltinType(call.Fun, makeSig(resTyp, x.typ))
		}

		x.typ = resTyp

	case _Make:
		// make(T, n)
		// make(T, n, m)
		// (no argument evaluated yet)
		arg0 := argList[0]
		T := checks.varType(arg0)
		if !isValid(T) {
			return
		}

		u, err := commonUnder(T, func(_, u Type) *typeError {
			switch u.(type) {
			case *Slice, *Map, *Chan:
				return nil // ok
			case nil:
				return typeErrorf("no specific type")
			default:
				return typeErrorf("type must be slice, map, or channel")
			}
		})
		if err != nil {
			checks.errorf(arg0, InvalidMake, invalidArg+"cannot make %s: %s", arg0, err.format(checks))
			return
		}

		var min int // minimum number of arguments
		switch u.(type) {
		case *Slice:
			min = 2
		case *Map, *Chan:
			min = 1
		default:
			// any other type was excluded above
			panic("unreachable")
		}
		if nargs < min || min+1 < nargs {
			checks.errorf(call, WrongArgCount, invalidOp+"%v expects %d or %d arguments; found %d", call, min, min+1, nargs)
			return
		}

		types := []Type{T}
		var sizes []int64 // constant integer arguments, if any
		for _, arg := range argList[1:] {
			typ, size := checks.index(arg, -1) // ok to continue with typ == Typ[Invalid]
			types = append(types, typ)
			if size >= 0 {
				sizes = append(sizes, size)
			}
		}
		if len(sizes) == 2 && sizes[0] > sizes[1] {
			checks.error(argList[1], SwappedMakeArgs, invalidArg+"length and capacity swapped")
			// safe to continue
		}
		x.mode = value
		x.typ = T
		if checks.recordTypes() {
			checks.recordBuiltinType(call.Fun, makeSig(x.typ, types...))
		}

	case _Max, _Min:
		// max(x, ...)
		// min(x, ...)
		checks.verifyVersionf(call.Fun, go1_21, "built-in %s", bin.name)

		op := token.LSS
		if id == _Max {
			op = token.GTR
		}

		for i, a := range args {
			if a.mode == invalid {
				return
			}

			if !allOrdered(a.typ) {
				checks.errorf(a, InvalidMinMaxOperand, invalidArg+"%s cannot be ordered", a)
				return
			}

			// The first argument is already in x and there's nothing left to do.
			if i > 0 {
				checks.matchTypes(x, a)
				if x.mode == invalid {
					return
				}

				if !Identical(x.typ, a.typ) {
					checks.errorf(a, MismatchedTypes, invalidArg+"mismatched types %s (previous argument) and %s (type of %s)", x.typ, a.typ, a.expr)
					return
				}

				if x.mode == constant_ && a.mode == constant_ {
					if constant.Compare(a.val, op, x.val) {
						*x = *a
					}
				} else {
					x.mode = value
				}
			}
		}

		// If nargs == 1, make sure x.mode is either a value or a constant.
		if x.mode != constant_ {
			x.mode = value
			// A value must not be untyped.
			checks.assignment(x, &emptyInterface, "argument to built-in "+bin.name)
			if x.mode == invalid {
				return
			}
		}

		// Use the final type computed above for all arguments.
		for _, a := range args {
			checks.updateExprType(a.expr, x.typ, true)
		}

		if checks.recordTypes() && x.mode != constant_ {
			types := make([]Type, nargs)
			for i := range types {
				types[i] = x.typ
			}
			checks.recordBuiltinType(call.Fun, makeSig(x.typ, types...))
		}

	case _New:
		// new(T)
		// (no argument evaluated yet)
		T := checks.varType(argList[0])
		if !isValid(T) {
			return
		}

		x.mode = value
		x.typ = &Pointer{base: T}
		if checks.recordTypes() {
			checks.recordBuiltinType(call.Fun, makeSig(x.typ, T))
		}

	case _Panic:
		// panic(x)
		// record panic call if inside a function with result parameters
		// (for use in Checker.isTerminating)
		if checks.sig != nil && checks.sig.results.Len() > 0 {
			// function has result parameters
			p := checks.isPanic
			if p == nil {
				// allocate lazily
				p = make(map[*syntax.CallExpr]bool)
				checks.isPanic = p
			}
			p[call] = true
		}

		checks.assignment(x, &emptyInterface, "argument to panic")
		if x.mode == invalid {
			return
		}

		x.mode = novalue
		if checks.recordTypes() {
			checks.recordBuiltinType(call.Fun, makeSig(nil, &emptyInterface))
		}


	case _Print, _Println:
		// print(x, y, ...)
		// println(x, y, ...)
		var params []Type
		if nargs > 0 {
			params = make([]Type, nargs)
			for i, a := range args {
				checks.assignment(a, nil, "argument to built-in "+predeclaredFuncs[id].name)
				if a.mode == invalid {
					return
				}
				params[i] = a.typ
			}
		}

		x.mode = novalue
		if checks.recordTypes() {
			checks.recordBuiltinType(call.Fun, makeSig(nil, params...))
		}

	case _Recover:
		// recover() interface{}
		x.mode = value
		x.typ = &emptyInterface
		if checks.recordTypes() {
			checks.recordBuiltinType(call.Fun, makeSig(x.typ))
		}

	case _Typeof:
		// typeof(x) string
		x.mode = constant_
		x.typ = Typ[String]
		x.val = constant.MakeString(args[0].typ.String())
		if checks.recordTypes() {
			checks.recordBuiltinType(call.Fun, makeSig(Typ[String], args[0].typ))
		}

	case _Add:
		// unsafe.Add(ptr unsafe.Pointer, len IntegerType) unsafe.Pointer
		checks.verifyVersionf(call.Fun, go1_17, "unsafe.Add")

		checks.assignment(x, Typ[UnsafePointer], "argument to unsafe.Add")
		if x.mode == invalid {
			return
		}

		y := args[1]
		if !checks.isValidIndex(y, InvalidUnsafeAdd, "length", true) {
			return
		}

		x.mode = value
		x.typ = Typ[UnsafePointer]
		if checks.recordTypes() {
			checks.recordBuiltinType(call.Fun, makeSig(x.typ, x.typ, y.typ))
		}

	case _Alignof:
		// unsafe.Alignof(x T) uintptr
		checks.assignment(x, nil, "argument to unsafe.Alignof")
		if x.mode == invalid {
			return
		}

		if hasVarSize(x.typ, nil) {
			x.mode = value
			if checks.recordTypes() {
				checks.recordBuiltinType(call.Fun, makeSig(Typ[Uintptr], x.typ))
			}
		} else {
			x.mode = constant_
			x.val = constant.MakeInt64(checks.conf.alignof(x.typ))
			// result is constant - no need to record signature
		}
		x.typ = Typ[Uintptr]

	case _Offsetof:
		// unsafe.Offsetof(x T) uintptr, where x must be a selector
		// (no argument evaluated yet)
		arg0 := argList[0]
		selx, _ := syntax.Unparen(arg0).(*syntax.SelectorExpr)
		if selx == nil {
			checks.errorf(arg0, BadOffsetofSyntax, invalidArg+"%s is not a selector expression", arg0)
			checks.use(arg0)
			return
		}

		checks.expr(nil, x, selx.X)
		if x.mode == invalid {
			return
		}

		base := derefStructPtr(x.typ)
		sel := selx.Sel.Value
		obj, index, indirect := lookupFieldOrMethod(base, false, checks.pkg, sel, false)
		switch obj.(type) {
		case nil:
			checks.errorf(x, MissingFieldOrMethod, invalidArg+"%s has no single field %s", base, sel)
			return
		case *Func:
			// TODO(gri) Using derefStructPtr may result in methods being found
			// that don't actually exist. An error either way, but the error
			// message is confusing. See: https://play.golang.org/p/al75v23kUy ,
			// but go/types reports: "invalid argument: x.m is a method value".
			checks.errorf(arg0, InvalidOffsetof, invalidArg+"%s is a method value", arg0)
			return
		}
		if indirect {
			checks.errorf(x, InvalidOffsetof, invalidArg+"field %s is embedded via a pointer in %s", sel, base)
			return
		}

		// TODO(gri) Should we pass x.typ instead of base (and have indirect report if derefStructPtr indirected)?
		checks.recordSelection(selx, FieldVal, base, obj, index, false)

		// record the selector expression (was bug - go.dev/issue/47895)
		{
			mode := value
			if x.mode == variable || indirect {
				mode = variable
			}
			checks.record(&operand{mode, selx, obj.Type(), nil, 0})
		}

		// The field offset is considered a variable even if the field is declared before
		// the part of the struct which is variable-sized. This makes both the rules
		// simpler and also permits (or at least doesn't prevent) a compiler from re-
		// arranging struct fields if it wanted to.
		if hasVarSize(base, nil) {
			x.mode = value
			if checks.recordTypes() {
				checks.recordBuiltinType(call.Fun, makeSig(Typ[Uintptr], obj.Type()))
			}
		} else {
			offs := checks.conf.offsetof(base, index)
			if offs < 0 {
				checks.errorf(x, TypeTooLarge, "%s is too large", x)
				return
			}
			x.mode = constant_
			x.val = constant.MakeInt64(offs)
			// result is constant - no need to record signature
		}
		x.typ = Typ[Uintptr]

	case _Sizeof:
		// unsafe.Sizeof(x T) uintptr
		checks.assignment(x, nil, "argument to unsafe.Sizeof")
		if x.mode == invalid {
			return
		}

		if hasVarSize(x.typ, nil) {
			x.mode = value
			if checks.recordTypes() {
				checks.recordBuiltinType(call.Fun, makeSig(Typ[Uintptr], x.typ))
			}
		} else {
			size := checks.conf.sizeof(x.typ)
			if size < 0 {
				checks.errorf(x, TypeTooLarge, "%s is too large", x)
				return
			}
			x.mode = constant_
			x.val = constant.MakeInt64(size)
			// result is constant - no need to record signature
		}
		x.typ = Typ[Uintptr]

	case _Slice:
		// unsafe.Slice(ptr *T, len IntegerType) []T
		checks.verifyVersionf(call.Fun, go1_17, "unsafe.Slice")

		u, _ := commonUnder(x.typ, nil)
		ptr, _ := u.(*Pointer)
		if ptr == nil {
			checks.errorf(x, InvalidUnsafeSlice, invalidArg+"%s is not a pointer", x)
			return
		}

		y := args[1]
		if !checks.isValidIndex(y, InvalidUnsafeSlice, "length", false) {
			return
		}

		x.mode = value
		x.typ = NewSlice(ptr.base)
		if checks.recordTypes() {
			checks.recordBuiltinType(call.Fun, makeSig(x.typ, ptr, y.typ))
		}

	case _SliceData:
		// unsafe.SliceData(slice []T) *T
		checks.verifyVersionf(call.Fun, go1_20, "unsafe.SliceData")

		u, _ := commonUnder(x.typ, nil)
		slice, _ := u.(*Slice)
		if slice == nil {
			checks.errorf(x, InvalidUnsafeSliceData, invalidArg+"%s is not a slice", x)
			return
		}

		x.mode = value
		x.typ = NewPointer(slice.elem)
		if checks.recordTypes() {
			checks.recordBuiltinType(call.Fun, makeSig(x.typ, slice))
		}

	case _String:
		// unsafe.String(ptr *byte, len IntegerType) string
		checks.verifyVersionf(call.Fun, go1_20, "unsafe.String")

		checks.assignment(x, NewPointer(universeByte), "argument to unsafe.String")
		if x.mode == invalid {
			return
		}

		y := args[1]
		if !checks.isValidIndex(y, InvalidUnsafeString, "length", false) {
			return
		}

		x.mode = value
		x.typ = Typ[String]
		if checks.recordTypes() {
			checks.recordBuiltinType(call.Fun, makeSig(x.typ, NewPointer(universeByte), y.typ))
		}

	case _StringData:
		// unsafe.StringData(str string) *byte
		checks.verifyVersionf(call.Fun, go1_20, "unsafe.StringData")

		checks.assignment(x, Typ[String], "argument to unsafe.StringData")
		if x.mode == invalid {
			return
		}

		x.mode = value
		x.typ = NewPointer(universeByte)
		if checks.recordTypes() {
			checks.recordBuiltinType(call.Fun, makeSig(x.typ, Typ[String]))
		}

	case _Assert:
		// assert(pred) panics at runtime if pred is false.
		// For compile-time constants, check at compile time.
		if !isBoolean(x.typ) {
			checks.errorf(x, Test, invalidArg+"%s is not a boolean expression", x)
			return
		}

		// If it's a compile-time constant, check it now
		if x.mode == constant_ {
			if x.val.Kind() != constant.Bool {
				checks.errorf(x, Test, "internal error: value of %s should be a boolean constant", x)
				return
			}
			if !constant.BoolVal(x.val) {
				checks.errorf(call, Test, "%v failed", call)
				// compile-time assertion failure - safe to continue
			}
		}

		// For runtime expressions, the check will be done by the backend
		x.mode = novalue

	case _Trace:
		// trace(x, y, z, ...) dumps the positions, expressions, and
		// values of its arguments. The result of trace is the value
		// of the first argument.
		// Note: trace is only available in self-test mode.
		// (no argument evaluated yet)
		if nargs == 0 {
			checks.dump("%v: trace() without arguments", atPos(call))
			x.mode = novalue
			break
		}
		var t operand
		x1 := x
		for _, arg := range argList {
			checks.rawExpr(nil, x1, arg, nil, false) // permit trace for types, e.g.: new(trace(T))
			checks.dump("%v: %s", atPos(x1), x1)
			x1 = &t // use incoming x only for first argument
		}
		if x.mode == invalid {
			return
		}
		// trace is only available in test mode - no need to record signature

	default:
		panic("unreachable")
	}

	assert(x.mode != invalid)
	return true
}

// sliceElem returns the slice element type for a slice operand x
// or a type error if x is not a slice (or a type set of slices).
func sliceElem(x *operand) (Type, *typeError) {
	var E Type
	var err *typeError
	typeset(x.typ, func(_, u Type) bool {
		s, _ := u.(*Slice)
		if s == nil {
			if x.isNil() {
				// Printing x in this case would just print "nil".
				// Special case this so we can emphasize "untyped".
				err = typeErrorf("argument must be a slice; have untyped nil")
			} else {
				err = typeErrorf("argument must be a slice; have %s", x)
			}
			return false
		}
		if E == nil {
			E = s.elem
		} else if !Identical(E, s.elem) {
			err = typeErrorf("mismatched slice element types %s and %s in %s", E, s.elem, x)
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return E, nil
}

// hasVarSize reports if the size of type t is variable due to type parameters
// or if the type is infinitely-sized due to a cycle for which the type has not
// yet been checked.
func hasVarSize(t Type, seen map[*Named]bool) (varSized bool) {
	// Cycles are only possible through *Named types.
	// The seen map is used to detect cycles and track
	// the results of previously seen types.
	if named := asNamed(t); named != nil {
		if v, ok := seen[named]; ok {
			return v
		}
		if seen == nil {
			seen = make(map[*Named]bool)
		}
		seen[named] = true // possibly cyclic until proven otherwise
		defer func() {
			seen[named] = varSized // record final determination for named
		}()
	}

	switch u := under(t).(type) {
	case *Array:
		return hasVarSize(u.elem, seen)
	case *Struct:
		for _, f := range u.fields {
			if hasVarSize(f.typ, seen) {
				return true
			}
		}
	case *Interface:
		return isTypeParam(t)
	case *Named, *Union:
		panic("unreachable")
	}
	return false
}

// applyTypeFunc applies f to x. If x is a type parameter,
// the result is a type parameter constrained by a new
// interface bound. The type bounds for that interface
// are computed by applying f to each of the type bounds
// of x. If any of these applications of f return nil,
// applyTypeFunc returns nil.
// If x is not a type parameter, the result is f(x).
func (checks *Checker) applyTypeFunc(f func(Type) Type, x *operand, id builtinId) Type {
	if tp, _ := Unalias(x.typ).(*TypeParam); tp != nil {
		// Test if t satisfies the requirements for the argument
		// type and collect possible result types at the same time.
		var terms []*Term
		if !tp.is(func(t *term) bool {
			if t == nil {
				return false
			}
			if r := f(t.typ); r != nil {
				terms = append(terms, NewTerm(t.tilde, r))
				return true
			}
			return false
		}) {
			return nil
		}

		// We can type-check this fine but we're introducing a synthetic
		// type parameter for the result. It's not clear what the API
		// implications are here. Report an error for 1.18 (see go.dev/issue/50912),
		// but continue type-checking.
		var code Code
		switch id {
		case _Real:
			code = InvalidReal
		case _Imag:
			code = InvalidImag
		case _Complex:
			code = InvalidComplex
		default:
			panic("unreachable")
		}
		checks.softErrorf(x, code, "%s not supported as argument to built-in %s for go1.18 (see go.dev/issue/50937)", x, predeclaredFuncs[id].name)

		// Construct a suitable new type parameter for the result type.
		// The type parameter is placed in the current package so export/import
		// works as expected.
		tpar := NewTypeName(nopos, checks.pkg, tp.obj.name, nil)
		ptyp := checks.newTypeParam(tpar, NewInterfaceType(nil, []Type{NewUnion(terms)})) // assigns type to tpar as a side-effect
		ptyp.index = tp.index

		return ptyp
	}

	return f(x.typ)
}

// makeSig makes a signature for the given argument and result types.
// Default types are used for untyped arguments, and res may be nil.
func makeSig(res Type, args ...Type) *Signature {
	list := make([]*Var, len(args))
	for i, param := range args {
		list[i] = NewParam(nopos, nil, "", Default(param))
	}
	params := NewTuple(list...)
	var result *Tuple
	if res != nil {
		assert(!isUntyped(res))
		result = NewTuple(newVar(ResultVar, nopos, nil, "", res))
	}
	return &Signature{params: params, results: result}
}

// arrayPtrDeref returns A if typ is of the form *A and A is an array;
// otherwise it returns typ.
func arrayPtrDeref(typ Type) Type {
	if p, ok := Unalias(typ).(*Pointer); ok {
		if a, _ := under(p.base).(*Array); a != nil {
			return a
		}
	}
	return typ
}
