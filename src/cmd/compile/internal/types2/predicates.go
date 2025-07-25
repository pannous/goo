// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements commonly used type predicates.

package types2

import (
	"slices"
	"unicode"
)

// isValid reports whether t is a valid type.
func isValid(t Type) bool { return Unalias(t) != Typ[Invalid] }

// The isX predicates below report whether t is an X.
// If t is a type parameter the result is false; i.e.,
// these predicates don't look inside a type parameter.

func isBoolean(t Type) bool        { return isBasic(t, IsBoolean) }
func isInteger(t Type) bool        { return isBasic(t, IsInteger) }
func isUnsigned(t Type) bool       { return isBasic(t, IsUnsigned) }
func isFloat(t Type) bool          { return isBasic(t, IsFloat) }
func isComplex(t Type) bool        { return isBasic(t, IsComplex) }
func isNumeric(t Type) bool        { return isBasic(t, IsNumeric) }
func isString(t Type) bool         { return isBasic(t, IsString) }
func isIntegerOrFloat(t Type) bool { return isBasic(t, IsInteger|IsFloat) }
func isConstType(t Type) bool      { return isBasic(t, IsConstType) }

// isBasic reports whether under(t) is a basic type with the specified info.
// If t is a type parameter the result is false; i.e.,
// isBasic does not look inside a type parameter.
func isBasic(t Type, info BasicInfo) bool {
	u, _ := under(t).(*Basic)
	return u != nil && u.info&info != 0
}

// The allX predicates below report whether t is an X.
// If t is a type parameter the result is true if isX is true
// for all specified types of the type parameter's type set.

func allBoolean(t Type) bool         { return allBasic(t, IsBoolean) }
func allInteger(t Type) bool         { return allBasic(t, IsInteger) }
func allUnsigned(t Type) bool        { return allBasic(t, IsUnsigned) }
func allNumeric(t Type) bool         { return allBasic(t, IsNumeric) }
func allString(t Type) bool          { return allBasic(t, IsString) }
func allOrdered(t Type) bool         { return allBasic(t, IsOrdered) }
func allNumericOrString(t Type) bool { return allBasic(t, IsNumeric|IsString) }

// allBasic reports whether under(t) is a basic type with the specified info.
// If t is a type parameter, the result is true if isBasic(t, info) is true
// for all specific types of the type parameter's type set.
func allBasic(t Type, info BasicInfo) bool {
	if tpar, _ := Unalias(t).(*TypeParam); tpar != nil {
		return tpar.is(func(t *term) bool { return t != nil && isBasic(t.typ, info) })
	}
	return isBasic(t, info)
}

// hasName reports whether t has a name. This includes
// predeclared types, defined types, and type parameters.
// hasName may be called with types that are not fully set up.
func hasName(t Type) bool {
	switch Unalias(t).(type) {
	case *Basic, *Named, *TypeParam:
		return true
	}
	return false
}

// isTypeLit reports whether t is a type literal.
// This includes all non-defined types, but also basic types.
// isTypeLit may be called with types that are not fully set up.
func isTypeLit(t Type) bool {
	switch Unalias(t).(type) {
	case *Named, *TypeParam:
		return false
	}
	return true
}

// isTyped reports whether t is typed; i.e., not an untyped
// constant or boolean.
// Safe to call from types that are not fully set up.
func isTyped(t Type) bool {
	// Alias and named types cannot denote untyped types
	// so there's no need to call Unalias or under, below.
	b, _ := t.(*Basic)
	return b == nil || b.info&IsUntyped == 0
}

// isUntyped(t) is the same as !isTyped(t).
// Safe to call from types that are not fully set up.
func isUntyped(t Type) bool {
	return !isTyped(t)
}

// isUntypedNumeric reports whether t is an untyped numeric type.
// Safe to call from types that are not fully set up.
func isUntypedNumeric(t Type) bool {
	// Alias and named types cannot denote untyped types
	// so there's no need to call Unalias or under, below.
	b, _ := t.(*Basic)
	return b != nil && b.info&IsUntyped != 0 && b.info&IsNumeric != 0
}

// IsInterface reports whether t is an interface type.
func IsInterface(t Type) bool {
	_, ok := under(t).(*Interface)
	return ok
}

// isNonTypeParamInterface reports whether t is an interface type but not a type parameter.
func isNonTypeParamInterface(t Type) bool {
	return !isTypeParam(t) && IsInterface(t)
}

// isTypeParam reports whether t is a type parameter.
func isTypeParam(t Type) bool {
	_, ok := Unalias(t).(*TypeParam)
	return ok
}

// hasEmptyTypeset reports whether t is a type parameter with an empty type set.
// The function does not force the computation of the type set and so is safe to
// use anywhere, but it may report a false negative if the type set has not been
// computed yet.
func hasEmptyTypeset(t Type) bool {
	if tpar, _ := Unalias(t).(*TypeParam); tpar != nil && tpar.bound != nil {
		iface, _ := safeUnderlying(tpar.bound).(*Interface)
		return iface != nil && iface.tset != nil && iface.tset.IsEmpty()
	}
	return false
}

// isGeneric reports whether a type is a generic, uninstantiated type
// (generic signatures are not included).
// TODO(gri) should we include signatures or assert that they are not present?
func isGeneric(t Type) bool {
	// A parameterized type is only generic if it doesn't have an instantiation already.
	if alias, _ := t.(*Alias); alias != nil && alias.tparams != nil && alias.targs == nil {
		return true
	}
	named := asNamed(t)
	return named != nil && named.obj != nil && named.inst == nil && named.TypeParams().Len() > 0
}

// Comparable reports whether values of type T are comparable.
func Comparable(T Type) bool {
	return comparableType(T, true, nil) == nil
}

// If T is comparable, comparableType returns nil.
// Otherwise it returns a type error explaining why T is not comparable.
// If dynamic is set, non-type parameter interfaces are always comparable.
func comparableType(T Type, dynamic bool, seen map[Type]bool) *typeError {
	if seen[T] {
		return nil
	}
	if seen == nil {
		seen = make(map[Type]bool)
	}
	seen[T] = true

	switch t := under(T).(type) {
	case *Basic:
		// assume invalid types to be comparable to avoid follow-up errors
		if t.kind == UntypedNil {
			return typeErrorf("")
		}

	case *Pointer, *Chan:
		// always comparable

	case *Struct:
		for _, f := range t.fields {
			if comparableType(f.typ, dynamic, seen) != nil {
				return typeErrorf("struct containing %s cannot be compared", f.typ)
			}
		}

	case *Array:
		if comparableType(t.elem, dynamic, seen) != nil {
			return typeErrorf("%s cannot be compared", T)
		}

	case *Interface:
		if dynamic && !isTypeParam(T) || t.typeSet().IsComparable(seen) {
			return nil
		}
		var cause string
		if t.typeSet().IsEmpty() {
			cause = "empty type set"
		} else {
			cause = "incomparable types in type set"
		}
		return typeErrorf(cause)

	case *Slice:
		// Slices are now comparable if element type is comparable
		if comparableType(t.elem, dynamic, seen) != nil {
			return typeErrorf("slice with non-comparable element type %s", t.elem)
		}
		return nil

	default:
		return typeErrorf("")
	}

	return nil
}

// hasNil reports whether type t includes the nil value.
func hasNil(t Type) bool {
	switch u := under(t).(type) {
	case *Basic:
		return u.kind == UnsafePointer
	case *Slice, *Pointer, *Signature, *Map, *Chan:
		return true
	case *Interface:
		return !isTypeParam(t) || underIs(t, func(u Type) bool {
			return u != nil && hasNil(u)
		})
	}
	return false
}

// samePkg reports whether packages a and b are the same.
func samePkg(a, b *Package) bool {
	// package is nil for objects in universe scope
	if a == nil || b == nil {
		return a == b
	}
	// a != nil && b != nil
	return a.path == b.path
}

// An ifacePair is a node in a stack of interface type pairs compared for identity.
type ifacePair struct {
	x, y *Interface
	prev *ifacePair
}

func (p *ifacePair) identical(q *ifacePair) bool {
	return p.x == q.x && p.y == q.y || p.x == q.y && p.y == q.x
}

// A comparer is used to compare types.
type comparer struct {
	ignoreTags     bool // if set, identical ignores struct tags
	ignoreInvalids bool // if set, identical treats an invalid type as identical to any type
}

// For changes to this code the corresponding changes should be made to unifier.nify.
func (c *comparer) identical(x, y Type, p *ifacePair) bool {
	x = Unalias(x)
	y = Unalias(y)

	if x == y {
		return true
	}

	if c.ignoreInvalids && (!isValid(x) || !isValid(y)) {
		return true
	}

	switch x := x.(type) {
	case *Basic:
		// Basic types are singletons except for the rune and byte
		// aliases, thus we cannot solely rely on the x == y check
		// above. See also comment in TypeName.IsAlias.
		if y, ok := y.(*Basic); ok {
			return x.kind == y.kind
		}

	case *Array:
		// Two array types are identical if they have identical element types
		// and the same array length.
		if y, ok := y.(*Array); ok {
			// If one or both array lengths are unknown (< 0) due to some error,
			// assume they are the same to avoid spurious follow-on errors.
			return (x.len < 0 || y.len < 0 || x.len == y.len) && c.identical(x.elem, y.elem, p)
		}

	case *Slice:
		// Two slice types are identical if they have identical element types.
		if y, ok := y.(*Slice); ok {
			return c.identical(x.elem, y.elem, p)
		}

	case *Struct:
		// Two struct types are identical if they have the same sequence of fields,
		// and if corresponding fields have the same names, and identical types,
		// and identical tags. Two embedded fields are considered to have the same
		// name. Lower-case field names from different packages are always different.
		if y, ok := y.(*Struct); ok {
			if x.NumFields() == y.NumFields() {
				for i, f := range x.fields {
					g := y.fields[i]
					if f.embedded != g.embedded ||
						!c.ignoreTags && x.Tag(i) != y.Tag(i) ||
						!f.sameId(g.pkg, g.name, false) ||
						!c.identical(f.typ, g.typ, p) {
						return false
					}
				}
				return true
			}
		}

	case *Pointer:
		// Two pointer types are identical if they have identical base types.
		if y, ok := y.(*Pointer); ok {
			return c.identical(x.base, y.base, p)
		}

	case *Tuple:
		// Two tuples types are identical if they have the same number of elements
		// and corresponding elements have identical types.
		if y, ok := y.(*Tuple); ok {
			if x.Len() == y.Len() {
				if x != nil {
					for i, v := range x.vars {
						w := y.vars[i]
						if !c.identical(v.typ, w.typ, p) {
							return false
						}
					}
				}
				return true
			}
		}

	case *Signature:
		y, _ := y.(*Signature)
		if y == nil {
			return false
		}

		// Two function types are identical if they have the same number of
		// parameters and result values, corresponding parameter and result types
		// are identical, and either both functions are variadic or neither is.
		// Parameter and result names are not required to match, and type
		// parameters are considered identical modulo renaming.

		if x.TypeParams().Len() != y.TypeParams().Len() {
			return false
		}

		// In the case of generic signatures, we will substitute in yparams and
		// yresults.
		yparams := y.params
		yresults := y.results

		if x.TypeParams().Len() > 0 {
			// We must ignore type parameter names when comparing x and y. The
			// easiest way to do this is to substitute x's type parameters for y's.
			xtparams := x.TypeParams().list()
			ytparams := y.TypeParams().list()

			var targs []Type
			for i := range xtparams {
				targs = append(targs, x.TypeParams().At(i))
			}
			smap := makeSubstMap(ytparams, targs)

			var checks *Checker  // ok to call subst on a nil *Checker
			ctxt := NewContext() // need a non-nil Context for the substitution below

			// Constraints must be pair-wise identical, after substitution.
			for i, xtparam := range xtparams {
				ybound := checks.subst(nopos, ytparams[i].bound, smap, nil, ctxt)
				if !c.identical(xtparam.bound, ybound, p) {
					return false
				}
			}

			yparams = checks.subst(nopos, y.params, smap, nil, ctxt).(*Tuple)
			yresults = checks.subst(nopos, y.results, smap, nil, ctxt).(*Tuple)
		}

		return x.variadic == y.variadic &&
			c.identical(x.params, yparams, p) &&
			c.identical(x.results, yresults, p)

	case *Union:
		if y, _ := y.(*Union); y != nil {
			// TODO(rfindley): can this be reached during type checking? If so,
			// consider passing a type set map.
			unionSets := make(map[*Union]*_TypeSet)
			xset := computeUnionTypeSet(nil, unionSets, nopos, x)
			yset := computeUnionTypeSet(nil, unionSets, nopos, y)
			return xset.terms.equal(yset.terms)
		}

	case *Interface:
		// Two interface types are identical if they describe the same type sets.
		// With the existing implementation restriction, this simplifies to:
		//
		// Two interface types are identical if they have the same set of methods with
		// the same names and identical function types, and if any type restrictions
		// are the same. Lower-case method names from different packages are always
		// different. The order of the methods is irrelevant.
		if y, ok := y.(*Interface); ok {
			xset := x.typeSet()
			yset := y.typeSet()
			if xset.comparable != yset.comparable {
				return false
			}
			if !xset.terms.equal(yset.terms) {
				return false
			}
			a := xset.methods
			b := yset.methods
			if len(a) == len(b) {
				// Interface types are the only types where cycles can occur
				// that are not "terminated" via named types; and such cycles
				// can only be created via method parameter types that are
				// anonymous interfaces (directly or indirectly) embedding
				// the current interface. Example:
				//
				//    type T interface {
				//        m() interface{T}
				//    }
				//
				// If two such (differently named) interfaces are compared,
				// endless recursion occurs if the cycle is not detected.
				//
				// If x and y were compared before, they must be equal
				// (if they were not, the recursion would have stopped);
				// search the ifacePair stack for the same pair.
				//
				// This is a quadratic algorithm, but in practice these stacks
				// are extremely short (bounded by the nesting depth of interface
				// type declarations that recur via parameter types, an extremely
				// rare occurrence). An alternative implementation might use a
				// "visited" map, but that is probably less efficient overall.
				q := &ifacePair{x, y, p}
				for p != nil {
					if p.identical(q) {
						return true // same pair was compared before
					}
					p = p.prev
				}
				if debug {
					assertSortedMethods(a)
					assertSortedMethods(b)
				}
				for i, f := range a {
					g := b[i]
					if f.Id() != g.Id() || !c.identical(f.typ, g.typ, q) {
						return false
					}
				}
				return true
			}
		}

	case *Map:
		// Two map types are identical if they have identical key and value types.
		if y, ok := y.(*Map); ok {
			return c.identical(x.key, y.key, p) && c.identical(x.elem, y.elem, p)
		}

	case *Chan:
		// Two channel types are identical if they have identical value types
		// and the same direction.
		if y, ok := y.(*Chan); ok {
			return x.dir == y.dir && c.identical(x.elem, y.elem, p)
		}

	case *Named:
		// Two named types are identical if their type names originate
		// in the same type declaration; if they are instantiated they
		// must have identical type argument lists.
		if y := asNamed(y); y != nil {
			// check type arguments before origins to match unifier
			// (for correct source code we need to do all checks so
			// order doesn't matter)
			xargs := x.TypeArgs().list()
			yargs := y.TypeArgs().list()
			if len(xargs) != len(yargs) {
				return false
			}
			for i, xarg := range xargs {
				if !Identical(xarg, yargs[i]) {
					return false
				}
			}
			return identicalOrigin(x, y)
		}

	case *TypeParam:
		// nothing to do (x and y being equal is caught in the very beginning of this function)

	case nil:
		// avoid a crash in case of nil type

	default:
		panic("unreachable")
	}

	return false
}

// identicalOrigin reports whether x and y originated in the same declaration.
func identicalOrigin(x, y *Named) bool {
	// TODO(gri) is this correct?
	return x.Origin().obj == y.Origin().obj
}

// identicalInstance reports if two type instantiations are identical.
// Instantiations are identical if their origin and type arguments are
// identical.
func identicalInstance(xorig Type, xargs []Type, yorig Type, yargs []Type) bool {
	if !slices.EqualFunc(xargs, yargs, Identical) {
		return false
	}

	return Identical(xorig, yorig)
}

// Default returns the default "typed" type for an "untyped" type;
// it returns the incoming type for all other types. The default type
// for untyped nil is untyped nil.
func Default(t Type) Type {
	// Alias and named types cannot denote untyped types
	// so there's no need to call Unalias or under, below.
	if t, _ := t.(*Basic); t != nil {
		switch t.kind {
		case UntypedBool:
			return Typ[Bool]
		case UntypedInt:
			return Typ[Int]
		case UntypedRune:
			return universeRune // use 'rune' name
		case UntypedFloat:
			return Typ[Float64]
		case UntypedComplex:
			return Typ[Complex128]
		case UntypedString:
			return Typ[String]
		}
	}
	return t
}

// maxType returns the "largest" type that encompasses both x and y.
// If x and y are different untyped numeric types, the result is the type of x or y
// that appears later in this list: integer, rune, floating-point, complex.
// Otherwise, if x != y, the result is nil.
func maxType(x, y Type) Type {
	// We only care about untyped types (for now), so == is good enough.
	// TODO(gri) investigate generalizing this function to simplify code elsewhere
	if x == y {
		return x
	}
	if isUntypedNumeric(x) && isUntypedNumeric(y) {
		// untyped types are basic types
		if x.(*Basic).kind > y.(*Basic).kind {
			return x
		}
		return y
	}
	return nil
}

// clone makes a "flat copy" of *p and returns a pointer to the copy.
func clone[P *T, T any](p P) P {
	c := *p
	return &c
}

// isValidName reports whether s is a valid Go identifier.
func isValidName(s string) bool {
	for i, ch := range s {
		if !(unicode.IsLetter(ch) || ch == '_' || i > 0 && unicode.IsDigit(ch)) {
			return false
		}
	}
	return true
}
