// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"go/constant"
	"internal/buildcfg"
	. "internal/types/errors"
	"slices"
)

func (checks *Checker) declare(scope *Scope, id *syntax.Name, obj Object, pos syntax.Pos) {
	// spec: "The blank identifier, represented by the underscore
	// character _, may be used in a declaration like any other
	// identifier but the declaration does not introduce a new
	// binding."
	if obj.Name() != "_" {
		if alt := scope.Insert(obj); alt != nil {
			err := checks.newError(DuplicateDecl)
			err.addf(obj, "%s redeclared in this block", obj.Name())
			err.addAltDecl(alt)
			err.report()
			return
		}
		obj.setScopePos(pos)
	}
	if id != nil {
		checks.recordDef(id, obj)
	}
}

// pathString returns a string of the form a->b-> ... ->g for a path [a, b, ... g].
func pathString(path []Object) string {
	var s string
	for i, p := range path {
		if i > 0 {
			s += "->"
		}
		s += p.Name()
	}
	return s
}

// objDecl type-checks the declaration of obj in its respective (file) environment.
// For the meaning of def, see Checker.definedType, in typexpr.go.
func (checks *Checker) objDecl(obj Object, defi *TypeName) {
	if tracePos {
		checks.pushPos(obj.Pos())
		defer func() {
			// If we're panicking, keep stack of source positions.
			if p := recover(); p != nil {
				panic(p)
			}
			checks.popPos()
		}()
	}

	if checks.conf.Trace && obj.Type() == nil {
		if checks.indent == 0 {
			fmt.Println() // empty line between top-level objects for readability
		}
		checks.trace(obj.Pos(), "-- checking %s (%s, objPath = %s)", obj, obj.color(), pathString(checks.objPath))
		checks.indent++
		defer func() {
			checks.indent--
			checks.trace(obj.Pos(), "=> %s (%s)", obj, obj.color())
		}()
	}

	// Checking the declaration of obj means inferring its type
	// (and possibly its value, for constants).
	// An object's type (and thus the object) may be in one of
	// three states which are expressed by colors:
	//
	// - an object whose type is not yet known is painted white (initial color)
	// - an object whose type is in the process of being inferred is painted grey
	// - an object whose type is fully inferred is painted black
	//
	// During type inference, an object's color changes from white to grey
	// to black (pre-declared objects are painted black from the start).
	// A black object (i.e., its type) can only depend on (refer to) other black
	// ones. White and grey objects may depend on white and black objects.
	// A dependency on a grey object indicates a cycle which may or may not be
	// valid.
	//
	// When objects turn grey, they are pushed on the object path (a stack);
	// they are popped again when they turn black. Thus, if a grey object (a
	// cycle) is encountered, it is on the object path, and all the objects
	// it depends on are the remaining objects on that path. Color encoding
	// is such that the color value of a grey object indicates the index of
	// that object in the object path.

	// During type-checking, white objects may be assigned a type without
	// traversing through objDecl; e.g., when initializing constants and
	// variables. Update the colors of those objects here (rather than
	// everywhere where we set the type) to satisfy the color invariants.
	if obj.color() == white && obj.Type() != nil {
		obj.setColor(black)
		return
	}

	switch obj.color() {
	case white:
		assert(obj.Type() == nil)
		// All color values other than white and black are considered grey.
		// Because black and white are < grey, all values >= grey are grey.
		// Use those values to encode the object's index into the object path.
		obj.setColor(grey + color(checks.push(obj)))
		defer func() {
			checks.pop().setColor(black)
		}()

	case black:
		assert(obj.Type() != nil)
		return

	default:
		// Color values other than white or black are considered grey.
		fallthrough

	case grey:
		// We have a (possibly invalid) cycle.
		// In the existing code, this is marked by a non-nil type
		// for the object except for constants and variables whose
		// type may be non-nil (known), or nil if it depends on the
		// not-yet known initialization value.
		// In the former case, set the type to Typ[Invalid] because
		// we have an initialization cycle. The cycle error will be
		// reported later, when determining initialization order.
		// TODO(gri) Report cycle here and simplify initialization
		// order code.
		switch obj := obj.(type) {
		case *Const:
			if !checks.validCycle(obj) || obj.typ == nil {
				obj.typ = Typ[Invalid]
			}

		case *Var:
			if !checks.validCycle(obj) || obj.typ == nil {
				obj.typ = Typ[Invalid]
			}

		case *TypeName:
			if !checks.validCycle(obj) {
				// break cycle
				// (without this, calling underlying()
				// below may lead to an endless loop
				// if we have a cycle for a defined
				// (*Named) type)
				obj.typ = Typ[Invalid]
			}

		case *Func:
			if !checks.validCycle(obj) {
				// Don't set obj.typ to Typ[Invalid] here
				// because plenty of code type-asserts that
				// functions have a *Signature type. Grey
				// functions have their type set to an empty
				// signature which makes it impossible to
				// initialize a variable with the function.
			}

		default:
			panic("unreachable")
		}
		assert(obj.Type() != nil)
		return
	}

	d := checks.objMap[obj]
	if d == nil {
		checks.dump("%v: %s should have been declared", obj.Pos(), obj)
		panic("unreachable")
	}

	// save/restore current environment and set up object environment
	defer func(env environment) {
		checks.environment = env
	}(checks.environment)
	checks.environment = environment{scope: d.file, version: d.version}

	// Const and var declarations must not have initialization
	// cycles. We track them by remembering the current declaration
	// in check.decl. Initialization expressions depending on other
	// consts, vars, or functions, add dependencies to the current
	// check.decl.
	switch obj := obj.(type) {
	case *Const:
		checks.decl = d // new package-level const decl
		checks.constDecl(obj, d.vtyp, d.init, d.inherited)
	case *Var:
		checks.decl = d // new package-level var decl
		checks.varDecl(obj, d.lhs, d.vtyp, d.init)
	case *TypeName:
		// invalid recursive types are detected via path
		checks.typeDecl(obj, d.tdecl, defi)
		checks.collectMethods(obj) // methods can only be added to top-level types
	case *Func:
		// functions may be recursive - no need to track dependencies
		checks.funcDecl(obj, d)
	default:
		panic("unreachable")
	}
}

// validCycle reports whether the cycle starting with obj is valid and
// reports an error if it is not.
func (checks *Checker) validCycle(obj Object) (valid bool) {
	// The object map contains the package scope objects and the non-interface methods.
	if debug {
		info := checks.objMap[obj]
		inObjMap := info != nil && (info.fdecl == nil || info.fdecl.Recv == nil) // exclude methods
		isPkgObj := obj.Parent() == checks.pkg.scope
		if isPkgObj != inObjMap {
			checks.dump("%v: inconsistent object map for %s (isPkgObj = %v, inObjMap = %v)", obj.Pos(), obj, isPkgObj, inObjMap)
			panic("unreachable")
		}
	}

	// Count cycle objects.
	assert(obj.color() >= grey)
	start := obj.color() - grey // index of obj in objPath
	cycle := checks.objPath[start:]
	tparCycle := false // if set, the cycle is through a type parameter list
	nval := 0          // number of (constant or variable) values in the cycle; valid if !generic
	ndef := 0          // number of type definitions in the cycle; valid if !generic
loop:
	for _, obj := range cycle {
		switch obj := obj.(type) {
		case *Const, *Var:
			nval++
		case *TypeName:
			// If we reach a generic type that is part of a cycle
			// and we are in a type parameter list, we have a cycle
			// through a type parameter list, which is invalid.
			if checks.inTParamList && isGeneric(obj.typ) {
				tparCycle = true
				break loop
			}

			// Determine if the type name is an alias or not. For
			// package-level objects, use the object map which
			// provides syntactic information (which doesn't rely
			// on the order in which the objects are set up). For
			// local objects, we can rely on the order, so use
			// the object's predicate.
			// TODO(gri) It would be less fragile to always access
			// the syntactic information. We should consider storing
			// this information explicitly in the object.
			var alias bool
			if checks.conf.EnableAlias {
				alias = obj.IsAlias()
			} else {
				if d := checks.objMap[obj]; d != nil {
					alias = d.tdecl.Alias // package-level object
				} else {
					alias = obj.IsAlias() // function local object
				}
			}
			if !alias {
				ndef++
			}
		case *Func:
			// ignored for now
		default:
			panic("unreachable")
		}
	}

	if checks.conf.Trace {
		checks.trace(obj.Pos(), "## cycle detected: objPath = %s->%s (len = %d)", pathString(cycle), obj.Name(), len(cycle))
		if tparCycle {
			checks.trace(obj.Pos(), "## cycle contains: generic type in a type parameter list")
		} else {
			checks.trace(obj.Pos(), "## cycle contains: %d values, %d type definitions", nval, ndef)
		}
		defer func() {
			if valid {
				checks.trace(obj.Pos(), "=> cycle is valid")
			} else {
				checks.trace(obj.Pos(), "=> error: cycle is invalid")
			}
		}()
	}

	if !tparCycle {
		// A cycle involving only constants and variables is invalid but we
		// ignore them here because they are reported via the initialization
		// cycle check.
		if nval == len(cycle) {
			return true
		}

		// A cycle involving only types (and possibly functions) must have at least
		// one type definition to be permitted: If there is no type definition, we
		// have a sequence of alias type names which will expand ad infinitum.
		if nval == 0 && ndef > 0 {
			return true
		}
	}

	checks.cycleError(cycle, firstInSrc(cycle))
	return false
}

// cycleError reports a declaration cycle starting with the object at cycle[start].
func (checks *Checker) cycleError(cycle []Object, start int) {
	// name returns the (possibly qualified) object name.
	// This is needed because with generic types, cycles
	// may refer to imported types. See go.dev/issue/50788.
	// TODO(gri) This functionality is used elsewhere. Factor it out.
	name := func(obj Object) string {
		return packagePrefix(obj.Pkg(), checks.qualifier) + obj.Name()
	}

	// If obj is a type alias, mark it as valid (not broken) in order to avoid follow-on errors.
	obj := cycle[start]
	tname, _ := obj.(*TypeName)
	if tname != nil && tname.IsAlias() {
		// If we use Alias nodes, it is initialized with Typ[Invalid].
		// TODO(gri) Adjust this code if we initialize with nil.
		if !checks.conf.EnableAlias {
			checks.validAlias(tname, Typ[Invalid])
		}
	}

	// report a more concise error for self references
	if len(cycle) == 1 {
		if tname != nil {
			checks.errorf(obj, InvalidDeclCycle, "invalid recursive type: %s refers to itself", name(obj))
		} else {
			checks.errorf(obj, InvalidDeclCycle, "invalid cycle in declaration: %s refers to itself", name(obj))
		}
		return
	}

	err := checks.newError(InvalidDeclCycle)
	if tname != nil {
		err.addf(obj, "invalid recursive type %s", name(obj))
	} else {
		err.addf(obj, "invalid cycle in declaration of %s", name(obj))
	}
	// "cycle[i] refers to cycle[j]" for (i,j) = (s,s+1), (s+1,s+2), ..., (n-1,0), (0,1), ..., (s-1,s) for len(cycle) = n, s = start.
	for i := range cycle {
		next := cycle[(start+i+1)%len(cycle)]
		err.addf(obj, "%s refers to %s", name(obj), name(next))
		obj = next
	}
	err.report()
}

// firstInSrc reports the index of the object with the "smallest"
// source position in path. path must not be empty.
func firstInSrc(path []Object) int {
	fst, pos := 0, path[0].Pos()
	for i, t := range path[1:] {
		if cmpPos(t.Pos(), pos) < 0 {
			fst, pos = i+1, t.Pos()
		}
	}
	return fst
}

func (checks *Checker) constDecl(obj *Const, typ, init syntax.Expr, inherited bool) {
	assert(obj.typ == nil)

	// use the correct value of iota and errpos
	defer func(iota constant.Value, errpos syntax.Pos) {
		checks.iota = iota
		checks.errpos = errpos
	}(checks.iota, checks.errpos)
	checks.iota = obj.val
	checks.errpos = nopos

	// provide valid constant value under all circumstances
	obj.val = constant.MakeUnknown()

	// determine type, if any
	if typ != nil {
		t := checks.typ(typ)
		if !isConstType(t) {
			// don't report an error if the type is an invalid C (defined) type
			// (go.dev/issue/22090)
			if isValid(under(t)) {
				checks.errorf(typ, InvalidConstType, "invalid constant type %s", t)
			}
			obj.typ = Typ[Invalid]
			return
		}
		obj.typ = t
	}

	// check initialization
	var x operand
	if init != nil {
		if inherited {
			// The initialization expression is inherited from a previous
			// constant declaration, and (error) positions refer to that
			// expression and not the current constant declaration. Use
			// the constant identifier position for any errors during
			// init expression evaluation since that is all we have
			// (see issues go.dev/issue/42991, go.dev/issue/42992).
			checks.errpos = obj.pos
		}
		checks.expr(nil, &x, init)
	}
	checks.initConst(obj, &x)
}

func (checks *Checker) varDecl(obj *Var, lhs []*Var, typ, init syntax.Expr) {
	assert(obj.typ == nil)

	// determine type, if any
	if typ != nil {
		obj.typ = checks.varType(typ)
		// We cannot spread the type to all lhs variables if there
		// are more than one since that would mark them as checked
		// (see Checker.objDecl) and the assignment of init exprs,
		// if any, would not be checked.
		//
		// TODO(gri) If we have no init expr, we should distribute
		// a given type otherwise we need to re-evaluate the type
		// expr for each lhs variable, leading to duplicate work.
	}

	// check initialization
	if init == nil {
		if typ == nil {
			// error reported before by arityMatch
			obj.typ = Typ[Invalid]
		}
		return
	}

	if lhs == nil || len(lhs) == 1 {
		assert(lhs == nil || lhs[0] == obj)
		var x operand
		checks.expr(newTarget(obj.typ, obj.name), &x, init)
		checks.initVar(obj, &x, "variable declaration")
		return
	}

	if debug {
		// obj must be one of lhs
		if !slices.Contains(lhs, obj) {
			panic("inconsistent lhs")
		}
	}

	// We have multiple variables on the lhs and one init expr.
	// Make sure all variables have been given the same type if
	// one was specified, otherwise they assume the type of the
	// init expression values (was go.dev/issue/15755).
	if typ != nil {
		for _, lhs := range lhs {
			lhs.typ = obj.typ
		}
	}

	checks.initVars(lhs, []syntax.Expr{init}, nil)
}

// isImportedConstraint reports whether typ is an imported type constraint.
func (checks *Checker) isImportedConstraint(typ Type) bool {
	named := asNamed(typ)
	if named == nil || named.obj.pkg == checks.pkg || named.obj.pkg == nil {
		return false
	}
	u, _ := named.under().(*Interface)
	return u != nil && !u.IsMethodSet()
}

func (checks *Checker) typeDecl(obj *TypeName, tdecl *syntax.TypeDecl, defi *TypeName) {
	assert(obj.typ == nil)

	// Only report a version error if we have not reported one already.
	versionErr := false

	var rhs Type
	checks.later(func() {
		if t := asNamed(obj.typ); t != nil { // type may be invalid
			checks.validType(t)
		}
		// If typ is local, an error was already reported where typ is specified/defined.
		_ = !versionErr && checks.isImportedConstraint(rhs) && checks.verifyVersionf(tdecl.Type, go1_18, "using type constraint %s", rhs)
	}).describef(obj, "validType(%s)", obj.Name())

	// First type parameter, or nil.
	var tparam0 *syntax.Field
	if len(tdecl.TParamList) > 0 {
		tparam0 = tdecl.TParamList[0]
	}

	// alias declaration
	if tdecl.Alias {
		// Report highest version requirement first so that fixing a version issue
		// avoids possibly two -lang changes (first to Go 1.9 and then to Go 1.23).
		if !versionErr && tparam0 != nil && !checks.verifyVersionf(tparam0, go1_23, "generic type alias") {
			versionErr = true
		}
		if !versionErr && !checks.verifyVersionf(tdecl, go1_9, "type alias") {
			versionErr = true
		}

		if checks.conf.EnableAlias {
			// TODO(gri) Should be able to use nil instead of Typ[Invalid] to mark
			//           the alias as incomplete. Currently this causes problems
			//           with certain cycles. Investigate.
			//
			// NOTE(adonovan): to avoid the Invalid being prematurely observed
			// by (e.g.) a var whose type is an unfinished cycle,
			// Unalias does not memoize if Invalid. Perhaps we should use a
			// special sentinel distinct from Invalid.
			alias := checks.newAlias(obj, Typ[Invalid])
			setDefType(defi, alias)

			// handle type parameters even if not allowed (Alias type is supported)
			if tparam0 != nil {
				if !versionErr && !buildcfg.Experiment.AliasTypeParams {
					checks.error(tdecl, UnsupportedFeature, "generic type alias requires GOEXPERIMENT=aliastypeparams")
					versionErr = true
				}
				checks.openScope(tdecl, "type parameters")
				defer checks.closeScope()
				checks.collectTypeParams(&alias.tparams, tdecl.TParamList)
			}

			rhs = checks.definedType(tdecl.Type, obj)
			assert(rhs != nil)
			alias.fromRHS = rhs
			Unalias(alias) // resolve alias.actual
		} else {
			if !versionErr && tparam0 != nil {
				checks.error(tdecl, UnsupportedFeature, "generic type alias requires GODEBUG=gotypesalias=1 or unset")
				versionErr = true
			}

			checks.brokenAlias(obj)
			rhs = checks.typ(tdecl.Type)
			checks.validAlias(obj, rhs)
		}
		return
	}

	// type definition or generic type declaration
	if !versionErr && tparam0 != nil && !checks.verifyVersionf(tparam0, go1_18, "type parameter") {
		versionErr = true
	}

	named := checks.newNamed(obj, nil, nil)
	setDefType(defi, named)

	if tdecl.TParamList != nil {
		checks.openScope(tdecl, "type parameters")
		defer checks.closeScope()
		checks.collectTypeParams(&named.tparams, tdecl.TParamList)
	}

	// determine underlying type of named
	rhs = checks.definedType(tdecl.Type, obj)
	assert(rhs != nil)
	named.fromRHS = rhs

	// If the underlying type was not set while type-checking the right-hand
	// side, it is invalid and an error should have been reported elsewhere.
	if named.underlying == nil {
		named.underlying = Typ[Invalid]
	}

	// Disallow a lone type parameter as the RHS of a type declaration (go.dev/issue/45639).
	// We don't need this restriction anymore if we make the underlying type of a type
	// parameter its constraint interface: if the RHS is a lone type parameter, we will
	// use its underlying type (like we do for any RHS in a type declaration), and its
	// underlying type is an interface and the type declaration is well defined.
	if isTypeParam(rhs) {
		checks.error(tdecl.Type, MisplacedTypeParam, "cannot use a type parameter as RHS in type declaration")
		named.underlying = Typ[Invalid]
	}
}

func (checks *Checker) collectTypeParams(dst **TypeParamList, list []*syntax.Field) {
	tparams := make([]*TypeParam, len(list))

	// Declare type parameters up-front.
	// The scope of type parameters starts at the beginning of the type parameter
	// list (so we can have mutually recursive parameterized type bounds).
	if len(list) > 0 {
		scopePos := list[0].Pos()
		for i, f := range list {
			tparams[i] = checks.declareTypeParam(f.Name, scopePos)
		}
	}

	// Set the type parameters before collecting the type constraints because
	// the parameterized type may be used by the constraints (go.dev/issue/47887).
	// Example: type T[P T[P]] any
	*dst = bindTParams(tparams)

	// Signal to cycle detection that we are in a type parameter list.
	// We can only be inside one type parameter list at any given time:
	// function closures may appear inside a type parameter list but they
	// cannot be generic, and their bodies are processed in delayed and
	// sequential fashion. Note that with each new declaration, we save
	// the existing environment and restore it when done; thus inTParamList
	// is true exactly only when we are in a specific type parameter list.
	assert(!checks.inTParamList)
	checks.inTParamList = true
	defer func() {
		checks.inTParamList = false
	}()

	// Keep track of bounds for later validation.
	var bound Type
	for i, f := range list {
		// Optimization: Re-use the previous type bound if it hasn't changed.
		// This also preserves the grouped output of type parameter lists
		// when printing type strings.
		if i == 0 || f.Type != list[i-1].Type {
			bound = checks.bound(f.Type)
			if isTypeParam(bound) {
				// We may be able to allow this since it is now well-defined what
				// the underlying type and thus type set of a type parameter is.
				// But we may need some additional form of cycle detection within
				// type parameter lists.
				checks.error(f.Type, MisplacedTypeParam, "cannot use a type parameter as constraint")
				bound = Typ[Invalid]
			}
		}
		tparams[i].bound = bound
	}
}

func (checks *Checker) bound(x syntax.Expr) Type {
	// A type set literal of the form ~T and A|B may only appear as constraint;
	// embed it in an implicit interface so that only interface type-checking
	// needs to take care of such type expressions.
	if op, _ := x.(*syntax.Operation); op != nil && (op.Op == syntax.Tilde || op.Op == syntax.Or) {
		t := checks.typ(&syntax.InterfaceType{MethodList: []*syntax.Field{{Type: x}}})
		// mark t as implicit interface if all went well
		if t, _ := t.(*Interface); t != nil {
			t.implicit = true
		}
		return t
	}
	return checks.typ(x)
}

func (checks *Checker) declareTypeParam(name *syntax.Name, scopePos syntax.Pos) *TypeParam {
	// Use Typ[Invalid] for the type constraint to ensure that a type
	// is present even if the actual constraint has not been assigned
	// yet.
	// TODO(gri) Need to systematically review all uses of type parameter
	//           constraints to make sure we don't rely on them if they
	//           are not properly set yet.
	tname := NewTypeName(name.Pos(), checks.pkg, name.Value, nil)
	tpar := checks.newTypeParam(tname, Typ[Invalid]) // assigns type to tname as a side-effect
	checks.declare(checks.scope, name, tname, scopePos)
	return tpar
}

func (checks *Checker) collectMethods(obj *TypeName) {
	// get associated methods
	// (Checker.collectObjects only collects methods with non-blank names;
	// Checker.resolveBaseTypeName ensures that obj is not an alias name
	// if it has attached methods.)
	methods := checks.methods[obj]
	if methods == nil {
		return
	}
	delete(checks.methods, obj)
	assert(!checks.objMap[obj].tdecl.Alias) // don't use TypeName.IsAlias (requires fully set up object)

	// use an objset to check for name conflicts
	var mset objset

	// spec: "If the base type is a struct type, the non-blank method
	// and field names must be distinct."
	base := asNamed(obj.typ) // shouldn't fail but be conservative
	if base != nil {
		assert(base.TypeArgs().Len() == 0) // collectMethods should not be called on an instantiated type

		// See go.dev/issue/52529: we must delay the expansion of underlying here, as
		// base may not be fully set-up.
		checks.later(func() {
			checks.checkFieldUniqueness(base)
		}).describef(obj, "verifying field uniqueness for %v", base)

		// Checker.Files may be called multiple times; additional package files
		// may add methods to already type-checked types. Add pre-existing methods
		// so that we can detect redeclarations.
		for i := 0; i < base.NumMethods(); i++ {
			m := base.Method(i)
			assert(m.name != "_")
			assert(mset.insert(m) == nil)
		}
	}

	// add valid methods
	for _, m := range methods {
		// spec: "For a base type, the non-blank names of methods bound
		// to it must be unique."
		assert(m.name != "_")
		if alt := mset.insert(m); alt != nil {
			if alt.Pos().IsKnown() {
				checks.errorf(m.pos, DuplicateMethod, "method %s.%s already declared at %v", obj.Name(), m.name, alt.Pos())
			} else {
				checks.errorf(m.pos, DuplicateMethod, "method %s.%s already declared", obj.Name(), m.name)
			}
			continue
		}

		if base != nil {
			base.AddMethod(m)
		}
	}
}

func (checks *Checker) checkFieldUniqueness(base *Named) {
	if t, _ := base.under().(*Struct); t != nil {
		var mset objset
		for i := 0; i < base.NumMethods(); i++ {
			m := base.Method(i)
			assert(m.name != "_")
			assert(mset.insert(m) == nil)
		}

		// Check that any non-blank field names of base are distinct from its
		// method names.
		for _, fld := range t.fields {
			if fld.name != "_" {
				if alt := mset.insert(fld); alt != nil {
					// Struct fields should already be unique, so we should only
					// encounter an alternate via collision with a method name.
					_ = alt.(*Func)

					// For historical consistency, we report the primary error on the
					// method, and the alt decl on the field.
					err := checks.newError(DuplicateFieldAndMethod)
					err.addf(alt, "field and method with the same name %s", fld.name)
					err.addAltDecl(fld)
					err.report()
				}
			}
		}
	}
}

func (checks *Checker) funcDecl(obj *Func, decl *declInfo) {
	assert(obj.typ == nil)

	// func declarations cannot use iota
	assert(checks.iota == nil)

	sig := new(Signature)
	obj.typ = sig // guard against cycles

	// Avoid cycle error when referring to method while type-checking the signature.
	// This avoids a nuisance in the best case (non-parameterized receiver type) and
	// since the method is not a type, we get an error. If we have a parameterized
	// receiver type, instantiating the receiver type leads to the instantiation of
	// its methods, and we don't want a cycle error in that case.
	// TODO(gri) review if this is correct and/or whether we still need this?
	saved := obj.color_
	obj.color_ = black
	fdecl := decl.fdecl
	checks.funcType(sig, fdecl.Recv, fdecl.TParamList, fdecl.Type)
	obj.color_ = saved

	// Set the scope's extent to the complete "func (...) { ... }"
	// so that Scope.Innermost works correctly.
	sig.scope.pos = fdecl.Pos()
	sig.scope.end = syntax.EndPos(fdecl)

	if len(fdecl.TParamList) > 0 && fdecl.Body == nil {
		checks.softErrorf(fdecl, BadDecl, "generic function is missing function body")
	}

	// function body must be type-checked after global declarations
	// (functions implemented elsewhere have no body)
	if !checks.conf.IgnoreFuncBodies && fdecl.Body != nil {
		checks.later(func() {
			checks.funcBody(decl, obj.name, sig, fdecl.Body, nil)
		}).describef(obj, "func %s", obj.name)
	}
}

func (checks *Checker) declStmt(list []syntax.Decl) {
	pkg := checks.pkg

	first := -1                // index of first ConstDecl in the current group, or -1
	var last *syntax.ConstDecl // last ConstDecl with init expressions, or nil
	for index, decl := range list {
		if _, ok := decl.(*syntax.ConstDecl); !ok {
			first = -1 // we're not in a constant declaration
		}

		switch s := decl.(type) {
		case *syntax.ConstDecl:
			top := len(checks.delayed)

			// iota is the index of the current constDecl within the group
			if first < 0 || s.Group == nil || list[index-1].(*syntax.ConstDecl).Group != s.Group {
				first = index
				last = nil
			}
			iota := constant.MakeInt64(int64(index - first))

			// determine which initialization expressions to use
			inherited := true
			switch {
			case s.Type != nil || s.Values != nil:
				last = s
				inherited = false
			case last == nil:
				last = new(syntax.ConstDecl) // make sure last exists
				inherited = false
			}

			// declare all constants
			lhs := make([]*Const, len(s.NameList))
			values := syntax.UnpackListExpr(last.Values)
			for i, name := range s.NameList {
				obj := NewConst(name.Pos(), pkg, name.Value, nil, iota)
				lhs[i] = obj

				var init syntax.Expr
				if i < len(values) {
					init = values[i]
				}

				checks.constDecl(obj, last.Type, init, inherited)
			}

			// Constants must always have init values.
			checks.arity(s.Pos(), s.NameList, values, true, inherited)

			// process function literals in init expressions before scope changes
			checks.processDelayed(top)

			// spec: "The scope of a constant or variable identifier declared
			// inside a function begins at the end of the ConstSpec or VarSpec
			// (ShortVarDecl for short variable declarations) and ends at the
			// end of the innermost containing block."
			scopePos := syntax.EndPos(s)
			for i, name := range s.NameList {
				checks.declare(checks.scope, name, lhs[i], scopePos)
			}

		case *syntax.VarDecl:
			top := len(checks.delayed)

			lhs0 := make([]*Var, len(s.NameList))
			for i, name := range s.NameList {
				lhs0[i] = newVar(LocalVar, name.Pos(), pkg, name.Value, nil)
			}

			// initialize all variables
			values := syntax.UnpackListExpr(s.Values)
			for i, obj := range lhs0 {
				var lhs []*Var
				var init syntax.Expr
				switch len(values) {
				case len(s.NameList):
					// lhs and rhs match
					init = values[i]
				case 1:
					// rhs is expected to be a multi-valued expression
					lhs = lhs0
					init = values[0]
				default:
					if i < len(values) {
						init = values[i]
					}
				}
				checks.varDecl(obj, lhs, s.Type, init)
				if len(values) == 1 {
					// If we have a single lhs variable we are done either way.
					// If we have a single rhs expression, it must be a multi-
					// valued expression, in which case handling the first lhs
					// variable will cause all lhs variables to have a type
					// assigned, and we are done as well.
					if debug {
						for _, obj := range lhs0 {
							assert(obj.typ != nil)
						}
					}
					break
				}
			}

			// If we have no type, we must have values.
			if s.Type == nil || values != nil {
				checks.arity(s.Pos(), s.NameList, values, false, false)
			}

			// process function literals in init expressions before scope changes
			checks.processDelayed(top)

			// declare all variables
			// (only at this point are the variable scopes (parents) set)
			scopePos := syntax.EndPos(s) // see constant declarations
			for i, name := range s.NameList {
				// see constant declarations
				checks.declare(checks.scope, name, lhs0[i], scopePos)
			}

		case *syntax.TypeDecl:
			obj := NewTypeName(s.Name.Pos(), pkg, s.Name.Value, nil)
			// spec: "The scope of a type identifier declared inside a function
			// begins at the identifier in the TypeSpec and ends at the end of
			// the innermost containing block."
			scopePos := s.Name.Pos()
			checks.declare(checks.scope, s.Name, obj, scopePos)
			// mark and unmark type before calling typeDecl; its type is still nil (see Checker.objDecl)
			obj.setColor(grey + color(checks.push(obj)))
			checks.typeDecl(obj, s, nil)
			checks.pop().setColor(black)

		default:
			checks.errorf(s, InvalidSyntaxTree, "unknown syntax.Decl node %T", s)
		}
	}
}
