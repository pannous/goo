// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements type parameter inference.

package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"slices"
	"strings"
)

// If enableReverseTypeInference is set, uninstantiated and
// partially instantiated generic functions may be assigned
// (incl. returned) to variables of function type and type
// inference will attempt to infer the missing type arguments.
// Available with go1.21.
const enableReverseTypeInference = true // disable for debugging

// infer attempts to infer the complete set of type arguments for generic function instantiation/call
// based on the given type parameters tparams, type arguments targs, function parameters params, and
// function arguments args, if any. There must be at least one type parameter, no more type arguments
// than type parameters, and params and args must match in number (incl. zero).
// If reverse is set, an error message's contents are reversed for a better error message for some
// errors related to reverse type inference (where the function call is synthetic).
// If successful, infer returns the complete list of given and inferred type arguments, one for each
// type parameter. Otherwise the result is nil. Errors are reported through the err parameter.
// Note: infer may fail (return nil) due to invalid args operands without reporting additional errors.
func (checks *Checker) infer(pos syntax.Pos, tparams []*TypeParam, targs []Type, params *Tuple, args []*operand, reverse bool, err *error_) (inferred []Type) {
	// Don't verify result conditions if there's no error handler installed:
	// in that case, an error leads to an exit panic and the result value may
	// be incorrect. But in that case it doesn't matter because callers won't
	// be able to use it either.
	if checks.conf.Error != nil {
		defer func() {
			assert(inferred == nil || len(inferred) == len(tparams) && !slices.Contains(inferred, nil))
		}()
	}

	if traceInference {
		checks.dump("== infer : %s%s ➞ %s", tparams, params, targs) // aligned with rename print below
		defer func() {
			checks.dump("=> %s ➞ %s\n", tparams, inferred)
		}()
	}

	// There must be at least one type parameter, and no more type arguments than type parameters.
	n := len(tparams)
	assert(n > 0 && len(targs) <= n)

	// Parameters and arguments must match in number.
	assert(params.Len() == len(args))

	// If we already have all type arguments, we're done.
	if len(targs) == n && !slices.Contains(targs, nil) {
		return targs
	}

	// If we have invalid (ordinary) arguments, an error was reported before.
	// Avoid additional inference errors and exit early (go.dev/issue/60434).
	for _, arg := range args {
		if arg.mode == invalid {
			return nil
		}
	}

	// Make sure we have a "full" list of type arguments, some of which may
	// be nil (unknown). Make a copy so as to not clobber the incoming slice.
	if len(targs) < n {
		targs2 := make([]Type, n)
		copy(targs2, targs)
		targs = targs2
	}
	// len(targs) == n

	// Continue with the type arguments we have. Avoid matching generic
	// parameters that already have type arguments against function arguments:
	// It may fail because matching uses type identity while parameter passing
	// uses assignment rules. Instantiate the parameter list with the type
	// arguments we have, and continue with that parameter list.

	// Substitute type arguments for their respective type parameters in params,
	// if any. Note that nil targs entries are ignored by check.subst.
	// We do this for better error messages; it's not needed for correctness.
	// For instance, given:
	//
	//   func f[P, Q any](P, Q) {}
	//
	//   func _(s string) {
	//           f[int](s, s) // ERROR
	//   }
	//
	// With substitution, we get the error:
	//   "cannot use s (variable of type string) as int value in argument to f[int]"
	//
	// Without substitution we get the (worse) error:
	//   "type string of s does not match inferred type int for P"
	// even though the type int was provided (not inferred) for P.
	//
	// TODO(gri) We might be able to finesse this in the error message reporting
	//           (which only happens in case of an error) and then avoid doing
	//           the substitution (which always happens).
	if params.Len() > 0 {
		smap := makeSubstMap(tparams, targs)
		params = checks.subst(nopos, params, smap, nil, checks.context()).(*Tuple)
	}

	// Unify parameter and argument types for generic parameters with typed arguments
	// and collect the indices of generic parameters with untyped arguments.
	// Terminology: generic parameter = function parameter with a type-parameterized type
	u := newUnifier(tparams, targs, checks.allowVersion(go1_21))

	errorf := func(tpar, targ Type, arg *operand) {
		// provide a better error message if we can
		targs := u.inferred(tparams)
		if targs[0] == nil {
			// The first type parameter couldn't be inferred.
			// If none of them could be inferred, don't try
			// to provide the inferred type in the error msg.
			allFailed := true
			for _, targ := range targs {
				if targ != nil {
					allFailed = false
					break
				}
			}
			if allFailed {
				err.addf(arg, "type %s of %s does not match %s (cannot infer %s)", targ, arg.expr, tpar, typeParamsString(tparams))
				return
			}
		}
		smap := makeSubstMap(tparams, targs)
		// TODO(gri): pass a poser here, rather than arg.Pos().
		inferred := checks.subst(arg.Pos(), tpar, smap, nil, checks.context())
		// CannotInferTypeArgs indicates a failure of inference, though the actual
		// error may be better attributed to a user-provided type argument (hence
		// InvalidTypeArg). We can't differentiate these cases, so fall back on
		// the more general CannotInferTypeArgs.
		if inferred != tpar {
			if reverse {
				err.addf(arg, "inferred type %s for %s does not match type %s of %s", inferred, tpar, targ, arg.expr)
			} else {
				err.addf(arg, "type %s of %s does not match inferred type %s for %s", targ, arg.expr, inferred, tpar)
			}
		} else {
			err.addf(arg, "type %s of %s does not match %s", targ, arg.expr, tpar)
		}
	}

	// indices of generic parameters with untyped arguments, for later use
	var untyped []int

	// --- 1 ---
	// use information from function arguments

	if traceInference {
		u.tracef("== function parameters: %s", params)
		u.tracef("-- function arguments : %s", args)
	}

	for i, arg := range args {
		if arg.mode == invalid {
			// An error was reported earlier. Ignore this arg
			// and continue, we may still be able to infer all
			// targs resulting in fewer follow-on errors.
			// TODO(gri) determine if we still need this check
			continue
		}
		par := params.At(i)
		if isParameterized(tparams, par.typ) || isParameterized(tparams, arg.typ) {
			// Function parameters are always typed. Arguments may be untyped.
			// Collect the indices of untyped arguments and handle them later.
			if isTyped(arg.typ) {
				if !u.unify(par.typ, arg.typ, assign) {
					errorf(par.typ, arg.typ, arg)
					return nil
				}
			} else if _, ok := par.typ.(*TypeParam); ok && !arg.isNil() {
				// Since default types are all basic (i.e., non-composite) types, an
				// untyped argument will never match a composite parameter type; the
				// only parameter type it can possibly match against is a *TypeParam.
				// Thus, for untyped arguments we only need to look at parameter types
				// that are single type parameters.
				// Also, untyped nils don't have a default type and can be ignored.
				// Finally, it's not possible to have an alias type denoting a type
				// parameter declared by the current function and use it in the same
				// function signature; hence we don't need to Unalias before the
				// .(*TypeParam) type assertion above.
				untyped = append(untyped, i)
			}
		}
	}

	if traceInference {
		inferred := u.inferred(tparams)
		u.tracef("=> %s ➞ %s\n", tparams, inferred)
	}

	// --- 2 ---
	// use information from type parameter constraints

	if traceInference {
		u.tracef("== type parameters: %s", tparams)
	}

	// Unify type parameters with their constraints as long
	// as progress is being made.
	//
	// This is an O(n^2) algorithm where n is the number of
	// type parameters: if there is progress, at least one
	// type argument is inferred per iteration, and we have
	// a doubly nested loop.
	//
	// In practice this is not a problem because the number
	// of type parameters tends to be very small (< 5 or so).
	// (It should be possible for unification to efficiently
	// signal newly inferred type arguments; then the loops
	// here could handle the respective type parameters only,
	// but that will come at a cost of extra complexity which
	// may not be worth it.)
	for i := 0; ; i++ {
		nn := u.unknowns()
		if traceInference {
			if i > 0 {
				fmt.Println()
			}
			u.tracef("-- iteration %d", i)
		}

		for _, tpar := range tparams {
			tx := u.at(tpar)
			core, single := coreTerm(tpar)
			if traceInference {
				u.tracef("-- type parameter %s = %s: core(%s) = %s, single = %v", tpar, tx, tpar, core, single)
			}

			// If the type parameter's constraint has a core term (i.e., a core type with tilde information)
			// try to unify the type parameter with that core type.
			if core != nil {
				// A type parameter can be unified with its constraint's core type in two cases.
				switch {
				case tx != nil:
					if traceInference {
						u.tracef("-> unify type parameter %s (type %s) with constraint core type %s", tpar, tx, core.typ)
					}
					// The corresponding type argument tx is known. There are 2 cases:
					// 1) If the core type has a tilde, per spec requirement for tilde
					//    elements, the core type is an underlying (literal) type.
					//    And because of the tilde, the underlying type of tx must match
					//    against the core type.
					//    But because unify automatically matches a defined type against
					//    an underlying literal type, we can simply unify tx with the
					//    core type.
					// 2) If the core type doesn't have a tilde, we also must unify tx
					//    with the core type.
					if !u.unify(tx, core.typ, 0) {
						// TODO(gri) Type parameters that appear in the constraint and
						//           for which we have type arguments inferred should
						//           use those type arguments for a better error message.
						err.addf(pos, "%s (type %s) does not satisfy %s", tpar, tx, tpar.Constraint())
						return nil
					}
				case single && !core.tilde:
					if traceInference {
						u.tracef("-> set type parameter %s to constraint's common underlying type %s", tpar, core.typ)
					}
					// The corresponding type argument tx is unknown and the core term
					// describes a single specific type and no tilde.
					// In this case the type argument must be that single type; set it.
					u.set(tpar, core.typ)
				}
			}

			// Independent of whether there is a core term, if the type argument tx is known
			// it must implement the methods of the type constraint, possibly after unification
			// of the relevant method signatures, otherwise tx cannot satisfy the constraint.
			// This unification step may provide additional type arguments.
			//
			// Note: The type argument tx may be known but contain references to other type
			// parameters (i.e., tx may still be parameterized).
			// In this case the methods of tx don't correctly reflect the final method set
			// and we may get a missing method error below. Skip this step in this case.
			//
			// TODO(gri) We should be able continue even with a parameterized tx if we add
			// a simplify step beforehand (see below). This will require factoring out the
			// simplify phase so we can call it from here.
			if tx != nil && !isParameterized(tparams, tx) {
				if traceInference {
					u.tracef("-> unify type parameter %s (type %s) methods with constraint methods", tpar, tx)
				}
				// TODO(gri) Now that unification handles interfaces, this code can
				//           be reduced to calling u.unify(tx, tpar.iface(), assign)
				//           (which will compare signatures exactly as we do below).
				//           We leave it as is for now because missingMethod provides
				//           a failure cause which allows for a better error message.
				//           Eventually, unify should return an error with cause.
				var cause string
				constraint := tpar.iface()
				if !checks.hasAllMethods(tx, constraint, true, func(x, y Type) bool { return u.unify(x, y, exact) }, &cause) {
					// TODO(gri) better error message (see TODO above)
					err.addf(pos, "%s (type %s) does not satisfy %s %s", tpar, tx, tpar.Constraint(), cause)
					return nil
				}
			}
		}

		if u.unknowns() == nn {
			break // no progress
		}
	}

	if traceInference {
		inferred := u.inferred(tparams)
		u.tracef("=> %s ➞ %s\n", tparams, inferred)
	}

	// --- 3 ---
	// use information from untyped constants

	if traceInference {
		u.tracef("== untyped arguments: %v", untyped)
	}

	// Some generic parameters with untyped arguments may have been given a type by now.
	// Collect all remaining parameters that don't have a type yet and determine the
	// maximum untyped type for each of those parameters, if possible.
	var maxUntyped map[*TypeParam]Type // lazily allocated (we may not need it)
	for _, index := range untyped {
		tpar := params.At(index).typ.(*TypeParam) // is type parameter (no alias) by construction of untyped
		if u.at(tpar) == nil {
			arg := args[index] // arg corresponding to tpar
			if maxUntyped == nil {
				maxUntyped = make(map[*TypeParam]Type)
			}
			max := maxUntyped[tpar]
			if max == nil {
				max = arg.typ
			} else {
				m := maxType(max, arg.typ)
				if m == nil {
					err.addf(arg, "mismatched types %s and %s (cannot infer %s)", max, arg.typ, tpar)
					return nil
				}
				max = m
			}
			maxUntyped[tpar] = max
		}
	}
	// maxUntyped contains the maximum untyped type for each type parameter
	// which doesn't have a type yet. Set the respective default types.
	for tpar, typ := range maxUntyped {
		d := Default(typ)
		assert(isTyped(d))
		u.set(tpar, d)
	}

	// --- simplify ---

	// u.inferred(tparams) now contains the incoming type arguments plus any additional type
	// arguments which were inferred. The inferred non-nil entries may still contain
	// references to other type parameters found in constraints.
	// For instance, for [A any, B interface{ []C }, C interface{ *A }], if A == int
	// was given, unification produced the type list [int, []C, *A]. We eliminate the
	// remaining type parameters by substituting the type parameters in this type list
	// until nothing changes anymore.
	inferred = u.inferred(tparams)
	if debug {
		for i, targ := range targs {
			assert(targ == nil || inferred[i] == targ)
		}
	}

	// The data structure of each (provided or inferred) type represents a graph, where
	// each node corresponds to a type and each (directed) vertex points to a component
	// type. The substitution process described above repeatedly replaces type parameter
	// nodes in these graphs with the graphs of the types the type parameters stand for,
	// which creates a new (possibly bigger) graph for each type.
	// The substitution process will not stop if the replacement graph for a type parameter
	// also contains that type parameter.
	// For instance, for [A interface{ *A }], without any type argument provided for A,
	// unification produces the type list [*A]. Substituting A in *A with the value for
	// A will lead to infinite expansion by producing [**A], [****A], [********A], etc.,
	// because the graph A -> *A has a cycle through A.
	// Generally, cycles may occur across multiple type parameters and inferred types
	// (for instance, consider [P interface{ *Q }, Q interface{ func(P) }]).
	// We eliminate cycles by walking the graphs for all type parameters. If a cycle
	// through a type parameter is detected, killCycles nils out the respective type
	// (in the inferred list) which kills the cycle, and marks the corresponding type
	// parameter as not inferred.
	//
	// TODO(gri) If useful, we could report the respective cycle as an error. We don't
	//           do this now because type inference will fail anyway, and furthermore,
	//           constraints with cycles of this kind cannot currently be satisfied by
	//           any user-supplied type. But should that change, reporting an error
	//           would be wrong.
	killCycles(tparams, inferred)

	// dirty tracks the indices of all types that may still contain type parameters.
	// We know that nil type entries and entries corresponding to provided (non-nil)
	// type arguments are clean, so exclude them from the start.
	var dirty []int
	for i, typ := range inferred {
		if typ != nil && (i >= len(targs) || targs[i] == nil) {
			dirty = append(dirty, i)
		}
	}

	for len(dirty) > 0 {
		if traceInference {
			u.tracef("-- simplify %s ➞ %s", tparams, inferred)
		}
		// TODO(gri) Instead of creating a new substMap for each iteration,
		// provide an update operation for substMaps and only change when
		// needed. Optimization.
		smap := makeSubstMap(tparams, inferred)
		n := 0
		for _, index := range dirty {
			t0 := inferred[index]
			if t1 := checks.subst(nopos, t0, smap, nil, checks.context()); t1 != t0 {
				// t0 was simplified to t1.
				// If t0 was a generic function, but the simplified signature t1 does
				// not contain any type parameters anymore, the function is not generic
				// anymore. Remove its type parameters. (go.dev/issue/59953)
				// Note that if t0 was a signature, t1 must be a signature, and t1
				// can only be a generic signature if it originated from a generic
				// function argument. Those signatures are never defined types and
				// thus there is no need to call under below.
				// TODO(gri) Consider doing this in Checker.subst.
				//           Then this would fall out automatically here and also
				//           in instantiation (where we also explicitly nil out
				//           type parameters). See the *Signature TODO in subst.
				if sig, _ := t1.(*Signature); sig != nil && sig.TypeParams().Len() > 0 && !isParameterized(tparams, sig) {
					sig.tparams = nil
				}
				inferred[index] = t1
				dirty[n] = index
				n++
			}
		}
		dirty = dirty[:n]
	}

	// Once nothing changes anymore, we may still have type parameters left;
	// e.g., a constraint with core type *P may match a type parameter Q but
	// we don't have any type arguments to fill in for *P or Q (go.dev/issue/45548).
	// Don't let such inferences escape; instead treat them as unresolved.
	for i, typ := range inferred {
		if typ == nil || isParameterized(tparams, typ) {
			obj := tparams[i].obj
			err.addf(pos, "cannot infer %s (declared at %v)", obj.name, obj.pos)
			return nil
		}
	}

	return
}

// renameTParams renames the type parameters in the given type such that each type
// parameter is given a new identity. renameTParams returns the new type parameters
// and updated type. If the result type is unchanged from the argument type, none
// of the type parameters in tparams occurred in the type.
// If typ is a generic function, type parameters held with typ are not changed and
// must be updated separately if desired.
// The positions is only used for debug traces.
func (checks *Checker) renameTParams(pos syntax.Pos, tparams []*TypeParam, typ Type) ([]*TypeParam, Type) {
	// For the purpose of type inference we must differentiate type parameters
	// occurring in explicit type or value function arguments from the type
	// parameters we are solving for via unification because they may be the
	// same in self-recursive calls:
	//
	//   func f[P constraint](x P) {
	//           f(x)
	//   }
	//
	// In this example, without type parameter renaming, the P used in the
	// instantiation f[P] has the same pointer identity as the P we are trying
	// to solve for through type inference. This causes problems for type
	// unification. Because any such self-recursive call is equivalent to
	// a mutually recursive call, type parameter renaming can be used to
	// create separate, disentangled type parameters. The above example
	// can be rewritten into the following equivalent code:
	//
	//   func f[P constraint](x P) {
	//           f2(x)
	//   }
	//
	//   func f2[P2 constraint](x P2) {
	//           f(x)
	//   }
	//
	// Type parameter renaming turns the first example into the second
	// example by renaming the type parameter P into P2.
	if len(tparams) == 0 {
		return nil, typ // nothing to do
	}

	tparams2 := make([]*TypeParam, len(tparams))
	for i, tparam := range tparams {
		tname := NewTypeName(tparam.Obj().Pos(), tparam.Obj().Pkg(), tparam.Obj().Name(), nil)
		tparams2[i] = NewTypeParam(tname, nil)
		tparams2[i].index = tparam.index // == i
	}

	renameMap := makeRenameMap(tparams, tparams2)
	for i, tparam := range tparams {
		tparams2[i].bound = checks.subst(pos, tparam.bound, renameMap, nil, checks.context())
	}

	return tparams2, checks.subst(pos, typ, renameMap, nil, checks.context())
}

// typeParamsString produces a string containing all the type parameter names
// in list suitable for human consumption.
func typeParamsString(list []*TypeParam) string {
	// common cases
	n := len(list)
	switch n {
	case 0:
		return ""
	case 1:
		return list[0].obj.name
	case 2:
		return list[0].obj.name + " and " + list[1].obj.name
	}

	// general case (n > 2)
	var buf strings.Builder
	for i, tname := range list[:n-1] {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(tname.obj.name)
	}
	buf.WriteString(", and ")
	buf.WriteString(list[n-1].obj.name)
	return buf.String()
}

// isParameterized reports whether typ contains any of the type parameters of tparams.
// If typ is a generic function, isParameterized ignores the type parameter declarations;
// it only considers the signature proper (incoming and result parameters).
func isParameterized(tparams []*TypeParam, typ Type) bool {
	w := tpWalker{
		tparams: tparams,
		seen:    make(map[Type]bool),
	}
	return w.isParameterized(typ)
}

type tpWalker struct {
	tparams []*TypeParam
	seen    map[Type]bool
}

func (w *tpWalker) isParameterized(typ Type) (res bool) {
	// detect cycles
	if x, ok := w.seen[typ]; ok {
		return x
	}
	w.seen[typ] = false
	defer func() {
		w.seen[typ] = res
	}()

	switch t := typ.(type) {
	case *Basic:
		// nothing to do

	case *Alias:
		return w.isParameterized(Unalias(t))

	case *Array:
		return w.isParameterized(t.elem)

	case *Slice:
		return w.isParameterized(t.elem)

	case *Struct:
		return w.varList(t.fields)

	case *Pointer:
		return w.isParameterized(t.base)

	case *Tuple:
		// This case does not occur from within isParameterized
		// because tuples only appear in signatures where they
		// are handled explicitly. But isParameterized is also
		// called by Checker.callExpr with a function result tuple
		// if instantiation failed (go.dev/issue/59890).
		return t != nil && w.varList(t.vars)

	case *Signature:
		// t.tparams may not be nil if we are looking at a signature
		// of a generic function type (or an interface method) that is
		// part of the type we're testing. We don't care about these type
		// parameters.
		// Similarly, the receiver of a method may declare (rather than
		// use) type parameters, we don't care about those either.
		// Thus, we only need to look at the input and result parameters.
		return t.params != nil && w.varList(t.params.vars) || t.results != nil && w.varList(t.results.vars)

	case *Interface:
		tset := t.typeSet()
		for _, m := range tset.methods {
			if w.isParameterized(m.typ) {
				return true
			}
		}
		return tset.is(func(t *term) bool {
			return t != nil && w.isParameterized(t.typ)
		})

	case *Map:
		return w.isParameterized(t.key) || w.isParameterized(t.elem)

	case *Chan:
		return w.isParameterized(t.elem)

	case *Named:
		for _, t := range t.TypeArgs().list() {
			if w.isParameterized(t) {
				return true
			}
		}

	case *TypeParam:
		return slices.Index(w.tparams, t) >= 0

	default:
		panic(fmt.Sprintf("unexpected %T", typ))
	}

	return false
}

func (w *tpWalker) varList(list []*Var) bool {
	for _, v := range list {
		if w.isParameterized(v.typ) {
			return true
		}
	}
	return false
}

// If the type parameter has a single specific type S, coreTerm returns (S, true).
// Otherwise, if tpar has a core type T, it returns a term corresponding to that
// core type and false. In that case, if any term of tpar has a tilde, the core
// term has a tilde. In all other cases coreTerm returns (nil, false).
func coreTerm(tpar *TypeParam) (*term, bool) {
	n := 0
	var single *term // valid if n == 1
	var tilde bool
	tpar.is(func(t *term) bool {
		if t == nil {
			assert(n == 0)
			return false // no terms
		}
		n++
		single = t
		if t.tilde {
			tilde = true
		}
		return true
	})
	if n == 1 {
		if debug {
			u, _ := commonUnder(tpar, nil)
			assert(under(single.typ) == u)
		}
		return single, true
	}
	if typ, _ := commonUnder(tpar, nil); typ != nil {
		// A core type is always an underlying type.
		// If any term of tpar has a tilde, we don't
		// have a precise core type and we must return
		// a tilde as well.
		return &term{tilde, typ}, false
	}
	return nil, false
}

// killCycles walks through the given type parameters and looks for cycles
// created by type parameters whose inferred types refer back to that type
// parameter, either directly or indirectly. If such a cycle is detected,
// it is killed by setting the corresponding inferred type to nil.
//
// TODO(gri) Determine if we can simply abort inference as soon as we have
// found a single cycle.
func killCycles(tparams []*TypeParam, inferred []Type) {
	w := cycleFinder{tparams, inferred, make(map[Type]bool)}
	for _, t := range tparams {
		w.typ(t) // t != nil
	}
}

type cycleFinder struct {
	tparams  []*TypeParam
	inferred []Type
	seen     map[Type]bool
}

func (w *cycleFinder) typ(typ Type) {
	typ = Unalias(typ)
	if w.seen[typ] {
		// We have seen typ before. If it is one of the type parameters
		// in w.tparams, iterative substitution will lead to infinite expansion.
		// Nil out the corresponding type which effectively kills the cycle.
		if tpar, _ := typ.(*TypeParam); tpar != nil {
			if i := slices.Index(w.tparams, tpar); i >= 0 {
				// cycle through tpar
				w.inferred[i] = nil
			}
		}
		// If we don't have one of our type parameters, the cycle is due
		// to an ordinary recursive type and we can just stop walking it.
		return
	}
	w.seen[typ] = true
	defer delete(w.seen, typ)

	switch t := typ.(type) {
	case *Basic:
		// nothing to do

	// *Alias:
	//      This case should not occur because of Unalias(typ) at the top.

	case *Array:
		w.typ(t.elem)

	case *Slice:
		w.typ(t.elem)

	case *Struct:
		w.varList(t.fields)

	case *Pointer:
		w.typ(t.base)

	// case *Tuple:
	//      This case should not occur because tuples only appear
	//      in signatures where they are handled explicitly.

	case *Signature:
		if t.params != nil {
			w.varList(t.params.vars)
		}
		if t.results != nil {
			w.varList(t.results.vars)
		}

	case *Union:
		for _, t := range t.terms {
			w.typ(t.typ)
		}

	case *Interface:
		for _, m := range t.methods {
			w.typ(m.typ)
		}
		for _, t := range t.embeddeds {
			w.typ(t)
		}

	case *Map:
		w.typ(t.key)
		w.typ(t.elem)

	case *Chan:
		w.typ(t.elem)

	case *Named:
		for _, tpar := range t.TypeArgs().list() {
			w.typ(tpar)
		}

	case *TypeParam:
		if i := slices.Index(w.tparams, t); i >= 0 && w.inferred[i] != nil {
			w.typ(w.inferred[i])
		}

	default:
		panic(fmt.Sprintf("unexpected %T", typ))
	}
}

func (w *cycleFinder) varList(list []*Var) {
	for _, v := range list {
		w.typ(v.typ)
	}
}
