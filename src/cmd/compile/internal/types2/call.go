// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements typechecking of call and selector expressions.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
	"strings"
)

// funcInst type-checks a function instantiation.
// The incoming x must be a generic function.
// If inst != nil, it provides some or all of the type arguments (inst.Index).
// If target != nil, it may be used to infer missing type arguments of x, if any.
// At least one of T or inst must be provided.
//
// There are two modes of operation:
//
//  1. If infer == true, funcInst infers missing type arguments as needed and
//     instantiates the function x. The returned results are nil.
//
//  2. If infer == false and inst provides all type arguments, funcInst
//     instantiates the function x. The returned results are nil.
//     If inst doesn't provide enough type arguments, funcInst returns the
//     available arguments; x remains unchanged.
//
// If an error (other than a version error) occurs in any case, it is reported
// and x.mode is set to invalid.
func (checks *Checker) funcInst(T *target, pos syntax.Pos, x *operand, inst *syntax.IndexExpr, infer bool) []Type {
	assert(T != nil || inst != nil)

	var instErrPos poser
	if inst != nil {
		instErrPos = inst.Pos()
		x.expr = inst // if we don't have an index expression, keep the existing expression of x
	} else {
		instErrPos = pos
	}
	versionErr := !checks.verifyVersionf(instErrPos, go1_18, "function instantiation")

	// targs and xlist are the type arguments and corresponding type expressions, or nil.
	var targs []Type
	var xlist []syntax.Expr
	if inst != nil {
		xlist = syntax.UnpackListExpr(inst.Index)
		targs = checks.typeList(xlist)
		if targs == nil {
			x.mode = invalid
			return nil
		}
		assert(len(targs) == len(xlist))
	}

	// Check the number of type arguments (got) vs number of type parameters (want).
	// Note that x is a function value, not a type expression, so we don't need to
	// call under below.
	sig := x.typ.(*Signature)
	got, want := len(targs), sig.TypeParams().Len()
	if got > want {
		// Providing too many type arguments is always an error.
		checks.errorf(xlist[got-1], WrongTypeArgCount, "got %d type arguments but want %d", got, want)
		x.mode = invalid
		return nil
	}

	if got < want {
		if !infer {
			return targs
		}

		// If the uninstantiated or partially instantiated function x is used in
		// an assignment (tsig != nil), infer missing type arguments by treating
		// the assignment
		//
		//    var tvar tsig = x
		//
		// like a call g(tvar) of the synthetic generic function g
		//
		//    func g[type_parameters_of_x](func_type_of_x)
		//
		var args []*operand
		var params []*Var
		var reverse bool
		if T != nil && sig.tparams != nil {
			if !versionErr && !checks.allowVersion(go1_21) {
				if inst != nil {
					checks.versionErrorf(instErrPos, go1_21, "partially instantiated function in assignment")
				} else {
					checks.versionErrorf(instErrPos, go1_21, "implicitly instantiated function in assignment")
				}
			}
			gsig := NewSignatureType(nil, nil, nil, sig.params, sig.results, sig.variadic)
			params = []*Var{NewParam(x.Pos(), checks.pkg, "", gsig)}
			// The type of the argument operand is tsig, which is the type of the LHS in an assignment
			// or the result type in a return statement. Create a pseudo-expression for that operand
			// that makes sense when reported in error messages from infer, below.
			expr := syntax.NewName(x.Pos(), T.desc)
			args = []*operand{{mode: value, expr: expr, typ: T.sig}}
			reverse = true
		}

		// Rename type parameters to avoid problems with recursive instantiations.
		// Note that NewTuple(params...) below is (*Tuple)(nil) if len(params) == 0, as desired.
		tparams, params2 := checks.renameTParams(pos, sig.TypeParams().list(), NewTuple(params...))

		err := checks.newError(CannotInferTypeArgs)
		targs = checks.infer(pos, tparams, targs, params2.(*Tuple), args, reverse, err)
		if targs == nil {
			if !err.empty() {
				err.report()
			}
			x.mode = invalid
			return nil
		}
		got = len(targs)
	}
	assert(got == want)

	// instantiate function signature
	sig = checks.instantiateSignature(x.Pos(), x.expr, sig, targs, xlist)

	x.typ = sig
	x.mode = value
	return nil
}

func (checks *Checker) instantiateSignature(pos syntax.Pos, expr syntax.Expr, typ *Signature, targs []Type, xlist []syntax.Expr) (res *Signature) {
	assert(checks != nil)
	assert(len(targs) == typ.TypeParams().Len())

	if checks.conf.Trace {
		checks.trace(pos, "-- instantiating signature %s with %s", typ, targs)
		checks.indent++
		defer func() {
			checks.indent--
			checks.trace(pos, "=> %s (under = %s)", res, res.Underlying())
		}()
	}

	// For signatures, Checker.instance will always succeed because the type argument
	// count is correct at this point (see assertion above); hence the type assertion
	// to *Signature will always succeed.
	inst := checks.instance(pos, typ, targs, nil, checks.context()).(*Signature)
	assert(inst.TypeParams().Len() == 0) // signature is not generic anymore
	checks.recordInstance(expr, targs, inst)
	assert(len(xlist) <= len(targs))

	// verify instantiation lazily (was go.dev/issue/50450)
	checks.later(func() {
		tparams := typ.TypeParams().list()
		// check type constraints
		if i, err := checks.verify(pos, tparams, targs, checks.context()); err != nil {
			// best position for error reporting
			pos := pos
			if i < len(xlist) {
				pos = syntax.StartPos(xlist[i])
			}
			checks.softErrorf(pos, InvalidTypeArg, "%s", err)
		} else {
			checks.mono.recordInstance(checks.pkg, pos, tparams, targs, xlist)
		}
	}).describef(pos, "verify instantiation")

	return inst
}

func (checks *Checker) callExpr(x *operand, call *syntax.CallExpr) exprKind {
	// Transform printf calls to fmt.Printf calls
	if name, ok := call.Fun.(*syntax.Name); ok && name.Value == "printf" {
		// Check if this is the builtin printf (not a user-defined function)
		_, obj := checks.lookupScope("printf")
		if obj != nil && obj.Parent() == Universe {
			// For now, require manual fmt import until auto-import is working
			if checks.lookup("fmt") != nil {
				// Transform printf(...) to fmt.Printf(...) by modifying call in place
				fmtName := syntax.NewName(name.Pos(), "fmt")
				printfName := syntax.NewName(name.Pos(), "Printf")
				selector := &syntax.SelectorExpr{
					X:   fmtName,
					Sel: printfName,
				}
				selector.SetPos(name.Pos())

				// Replace the function in the original call
				call.Fun = selector
			} else {
				// Provide helpful error message
				checks.errorf(name, UndeclaredName, "printf requires 'import \"fmt\"' - automatic import only for .goo files")
				return expression
			}
		}
	}

	var inst *syntax.IndexExpr // function instantiation, if any
	if iexpr, _ := call.Fun.(*syntax.IndexExpr); iexpr != nil {
		if checks.indexExpr(x, iexpr) {
			// Delay function instantiation to argument checking,
			// where we combine type and value arguments for type
			// inference.
			assert(x.mode == value)
			inst = iexpr
		}
		x.expr = iexpr
		checks.record(x)
	} else {
		checks.exprOrType(x, call.Fun, true)
	}
	// x.typ may be generic

	switch x.mode {
	case invalid:
		checks.use(call.ArgList...)
		x.expr = call
		return statement

	case typexpr:
		// conversion
		checks.nonGeneric(nil, x)
		if x.mode == invalid {
			return conversion
		}
		T := x.typ
		x.mode = invalid
		switch n := len(call.ArgList); n {
		case 0:
			checks.errorf(call, WrongArgCount, "missing argument in conversion to %s", T)
		case 1:
			checks.expr(nil, x, call.ArgList[0])
			if x.mode != invalid {
				if t, _ := under(T).(*Interface); t != nil && !isTypeParam(T) {
					if !t.IsMethodSet() {
						checks.errorf(call, MisplacedConstraintIface, "cannot use interface %s in conversion (contains specific type constraints or is comparable)", T)
						break
					}
				}
				if hasDots(call) {
					checks.errorf(call.ArgList[0], BadDotDotDotSyntax, "invalid use of ... in conversion to %s", T)
					break
				}
				checks.conversion(x, T)
			}
		default:
			checks.use(call.ArgList...)
			checks.errorf(call.ArgList[n-1], WrongArgCount, "too many arguments in conversion to %s", T)
		}
		x.expr = call
		return conversion

	case builtin:
		// no need to check for non-genericity here
		id := x.id
		if !checks.builtin(x, call, id) {
			x.mode = invalid
		}
		x.expr = call
		// a non-constant result implies a function call
		if x.mode != invalid && x.mode != constant_ {
			checks.hasCallOrRecv = true
		}
		return predeclaredFuncs[id].kind
	}

	// ordinary function/method call
	// signature may be generic
	cgocall := x.mode == cgofunc

	// If the operand type is a type parameter, all types in its type set
	// must have a common underlying type, which must be a signature.
	u, err := commonUnder(x.typ, func(t, u Type) *typeError {
		if _, ok := u.(*Signature); u != nil && !ok {
			return typeErrorf("%s is not a function", t)
		}
		return nil
	})
	if err != nil {
		checks.errorf(x, InvalidCall, invalidOp+"cannot call %s: %s", x, err.format(checks))
		x.mode = invalid
		x.expr = call
		return statement
	}
	sig := u.(*Signature) // u must be a signature per the commonUnder condition

	// Capture wasGeneric before sig is potentially instantiated below.
	wasGeneric := sig.TypeParams().Len() > 0

	// evaluate type arguments, if any
	var xlist []syntax.Expr
	var targs []Type
	if inst != nil {
		xlist = syntax.UnpackListExpr(inst.Index)
		targs = checks.typeList(xlist)
		if targs == nil {
			checks.use(call.ArgList...)
			x.mode = invalid
			x.expr = call
			return statement
		}
		assert(len(targs) == len(xlist))

		// check number of type arguments (got) vs number of type parameters (want)
		got, want := len(targs), sig.TypeParams().Len()
		if got > want {
			checks.errorf(xlist[want], WrongTypeArgCount, "got %d type arguments but want %d", got, want)
			checks.use(call.ArgList...)
			x.mode = invalid
			x.expr = call
			return statement
		}

		// If sig is generic and all type arguments are provided, preempt function
		// argument type inference by explicitly instantiating the signature. This
		// ensures that we record accurate type information for sig, even if there
		// is an error checking its arguments (for example, if an incorrect number
		// of arguments is supplied).
		if got == want && want > 0 {
			checks.verifyVersionf(inst, go1_18, "function instantiation")
			sig = checks.instantiateSignature(inst.Pos(), inst, sig, targs, xlist)
			// targs have been consumed; proceed with checking arguments of the
			// non-generic signature.
			targs = nil
			xlist = nil
		}
	}

	// evaluate arguments
	args, atargs := checks.genericExprList(call.ArgList)
	sig = checks.arguments(call, sig, targs, xlist, args, atargs)

	if wasGeneric && sig.TypeParams().Len() == 0 {
		// update the recorded type of call.Fun to its instantiated type
		checks.recordTypeAndValue(call.Fun, value, sig, nil)
	}

	// determine result
	switch sig.results.Len() {
	case 0:
		x.mode = novalue
	case 1:
		if cgocall {
			x.mode = commaerr
		} else {
			x.mode = value
		}
		x.typ = sig.results.vars[0].typ // unpack tuple
	default:
		x.mode = value
		x.typ = sig.results
	}
	x.expr = call
	checks.hasCallOrRecv = true

	// if type inference failed, a parameterized result must be invalidated
	// (operands cannot have a parameterized type)
	if x.mode == value && sig.TypeParams().Len() > 0 && isParameterized(sig.TypeParams().list(), x.typ) {
		x.mode = invalid
	}

	return statement
}

// exprList evaluates a list of expressions and returns the corresponding operands.
// A single-element expression list may evaluate to multiple operands.
func (checks *Checker) exprList(elist []syntax.Expr) (xlist []*operand) {
	if n := len(elist); n == 1 {
		xlist, _ = checks.multiExpr(elist[0], false)
	} else if n > 1 {
		// multiple (possibly invalid) values
		xlist = make([]*operand, n)
		for i, e := range elist {
			var x operand
			checks.expr(nil, &x, e)
			xlist[i] = &x
		}
	}
	return
}

// genericExprList is like exprList but result operands may be uninstantiated or partially
// instantiated generic functions (where constraint information is insufficient to infer
// the missing type arguments) for Go 1.21 and later.
// For each non-generic or uninstantiated generic operand, the corresponding targsList and
// elements do not exist (targsList is nil) or the elements are nil.
// For each partially instantiated generic function operand, the corresponding
// targsList elements are the operand's partial type arguments.
func (checks *Checker) genericExprList(elist []syntax.Expr) (resList []*operand, targsList [][]Type) {
	if debug {
		defer func() {
			// type arguments must only exist for partially instantiated functions
			for i, x := range resList {
				if i < len(targsList) {
					if n := len(targsList[i]); n > 0 {
						// x must be a partially instantiated function
						assert(n < x.typ.(*Signature).TypeParams().Len())
					}
				}
			}
		}()
	}

	// Before Go 1.21, uninstantiated or partially instantiated argument functions are
	// nor permitted. Checker.funcInst must infer missing type arguments in that case.
	infer := true // for -lang < go1.21
	n := len(elist)
	if n > 0 && checks.allowVersion(go1_21) {
		infer = false
	}

	if n == 1 {
		// single value (possibly a partially instantiated function), or a multi-valued expression
		e := elist[0]
		var x operand
		if inst, _ := e.(*syntax.IndexExpr); inst != nil && checks.indexExpr(&x, inst) {
			// x is a generic function.
			targs := checks.funcInst(nil, x.Pos(), &x, inst, infer)
			if targs != nil {
				// x was not instantiated: collect the (partial) type arguments.
				targsList = [][]Type{targs}
				// Update x.expr so that we can record the partially instantiated function.
				x.expr = inst
			} else {
				// x was instantiated: we must record it here because we didn't
				// use the usual expression evaluators.
				checks.record(&x)
			}
			resList = []*operand{&x}
		} else {
			// x is not a function instantiation (it may still be a generic function).
			checks.rawExpr(nil, &x, e, nil, true)
			checks.exclude(&x, 1<<novalue|1<<builtin|1<<typexpr)
			if t, ok := x.typ.(*Tuple); ok && x.mode != invalid {
				// x is a function call returning multiple values; it cannot be generic.
				resList = make([]*operand, t.Len())
				for i, v := range t.vars {
					resList[i] = &operand{mode: value, expr: e, typ: v.typ}
				}
			} else {
				// x is exactly one value (possibly invalid or uninstantiated generic function).
				resList = []*operand{&x}
			}
		}
	} else if n > 1 {
		// multiple values
		resList = make([]*operand, n)
		targsList = make([][]Type, n)
		for i, e := range elist {
			var x operand
			if inst, _ := e.(*syntax.IndexExpr); inst != nil && checks.indexExpr(&x, inst) {
				// x is a generic function.
				targs := checks.funcInst(nil, x.Pos(), &x, inst, infer)
				if targs != nil {
					// x was not instantiated: collect the (partial) type arguments.
					targsList[i] = targs
					// Update x.expr so that we can record the partially instantiated function.
					x.expr = inst
				} else {
					// x was instantiated: we must record it here because we didn't
					// use the usual expression evaluators.
					checks.record(&x)
				}
			} else {
				// x is exactly one value (possibly invalid or uninstantiated generic function).
				checks.genericExpr(&x, e)
			}
			resList[i] = &x
		}
	}

	return
}

// arguments type-checks arguments passed to a function call with the given signature.
// The function and its arguments may be generic, and possibly partially instantiated.
// targs and xlist are the function's type arguments (and corresponding expressions).
// args are the function arguments. If an argument args[i] is a partially instantiated
// generic function, atargs[i] are the corresponding type arguments.
// If the callee is variadic, arguments adjusts its signature to match the provided
// arguments. The type parameters and arguments of the callee and all its arguments
// are used together to infer any missing type arguments, and the callee and argument
// functions are instantiated as necessary.
// The result signature is the (possibly adjusted and instantiated) function signature.
// If an error occurred, the result signature is the incoming sig.
func (checks *Checker) arguments(call *syntax.CallExpr, sig *Signature, targs []Type, xlist []syntax.Expr, args []*operand, atargs [][]Type) (rsig *Signature) {
	rsig = sig

	// Function call argument/parameter count requirements
	//
	//               | standard call    | dotdotdot call |
	// --------------+------------------+----------------+
	// standard func | nargs == npars   | invalid        |
	// --------------+------------------+----------------+
	// variadic func | nargs >= npars-1 | nargs == npars |
	// --------------+------------------+----------------+

	nargs := len(args)
	npars := sig.params.Len()
	ddd := hasDots(call)

	// set up parameters
	sigParams := sig.params // adjusted for variadic functions (may be nil for empty parameter lists!)
	adjusted := false       // indicates if sigParams is different from sig.params
	if sig.variadic {
		if ddd {
			// variadic_func(a, b, c...)
			if len(call.ArgList) == 1 && nargs > 1 {
				// f()... is not permitted if f() is multi-valued
				//check.errorf(call.Ellipsis, "cannot use ... with %d-valued %s", nargs, call.ArgList[0])
				checks.errorf(call, InvalidDotDotDot, "cannot use ... with %d-valued %s", nargs, call.ArgList[0])
				return
			}
		} else {
			// variadic_func(a, b, c)
			if nargs >= npars-1 {
				// Create custom parameters for arguments: keep
				// the first npars-1 parameters and add one for
				// each argument mapping to the ... parameter.
				vars := make([]*Var, npars-1) // npars > 0 for variadic functions
				copy(vars, sig.params.vars)
				last := sig.params.vars[npars-1]
				typ := last.typ.(*Slice).elem
				for len(vars) < nargs {
					vars = append(vars, NewParam(last.pos, last.pkg, last.name, typ))
				}
				sigParams = NewTuple(vars...) // possibly nil!
				adjusted = true
				npars = nargs
			} else {
				// nargs < npars-1
				npars-- // for correct error message below
			}
		}
	} else {
		if ddd {
			// standard_func(a, b, c...)
			//check.errorf(call.Ellipsis, "cannot use ... in call to non-variadic %s", call.Fun)
			checks.errorf(call, NonVariadicDotDotDot, "cannot use ... in call to non-variadic %s", call.Fun)
			return
		}
		// standard_func(a, b, c)
	}

	// check argument count
	if nargs != npars {
		var at poser = call
		qualifier := "not enough"
		if nargs > npars {
			at = args[npars].expr // report at first extra argument
			qualifier = "too many"
		} else if nargs > 0 {
			at = args[nargs-1].expr // report at last argument
		}
		// take care of empty parameter lists represented by nil tuples
		var params []*Var
		if sig.params != nil {
			params = sig.params.vars
		}
		err := checks.newError(WrongArgCount)
		err.addf(at, "%s arguments in call to %s", qualifier, call.Fun)
		err.addf(nopos, "have %s", checks.typesSummary(operandTypes(args), false, ddd))
		err.addf(nopos, "want %s", checks.typesSummary(varTypes(params), sig.variadic, false))
		err.report()
		return
	}

	// collect type parameters of callee and generic function arguments
	var tparams []*TypeParam

	// collect type parameters of callee
	n := sig.TypeParams().Len()
	if n > 0 {
		if !checks.allowVersion(go1_18) {
			if iexpr, _ := call.Fun.(*syntax.IndexExpr); iexpr != nil {
				checks.versionErrorf(iexpr, go1_18, "function instantiation")
			} else {
				checks.versionErrorf(call, go1_18, "implicit function instantiation")
			}
		}
		// rename type parameters to avoid problems with recursive calls
		var tmp Type
		tparams, tmp = checks.renameTParams(call.Pos(), sig.TypeParams().list(), sigParams)
		sigParams = tmp.(*Tuple)
		// make sure targs and tparams have the same length
		for len(targs) < len(tparams) {
			targs = append(targs, nil)
		}
	}
	assert(len(tparams) == len(targs))

	// collect type parameters from generic function arguments
	var genericArgs []int // indices of generic function arguments
	if enableReverseTypeInference {
		for i, arg := range args {
			// generic arguments cannot have a defined (*Named) type - no need for underlying type below
			if asig, _ := arg.typ.(*Signature); asig != nil && asig.TypeParams().Len() > 0 {
				// The argument type is a generic function signature. This type is
				// pointer-identical with (it's copied from) the type of the generic
				// function argument and thus the function object.
				// Before we change the type (type parameter renaming, below), make
				// a clone of it as otherwise we implicitly modify the object's type
				// (go.dev/issues/63260).
				asig = clone(asig)
				// Rename type parameters for cases like f(g, g); this gives each
				// generic function argument a unique type identity (go.dev/issues/59956).
				// TODO(gri) Consider only doing this if a function argument appears
				//           multiple times, which is rare (possible optimization).
				atparams, tmp := checks.renameTParams(call.Pos(), asig.TypeParams().list(), asig)
				asig = tmp.(*Signature)
				asig.tparams = &TypeParamList{atparams} // renameTParams doesn't touch associated type parameters
				arg.typ = asig                          // new type identity for the function argument
				tparams = append(tparams, atparams...)
				// add partial list of type arguments, if any
				if i < len(atargs) {
					targs = append(targs, atargs[i]...)
				}
				// make sure targs and tparams have the same length
				for len(targs) < len(tparams) {
					targs = append(targs, nil)
				}
				genericArgs = append(genericArgs, i)
			}
		}
	}
	assert(len(tparams) == len(targs))

	// at the moment we only support implicit instantiations of argument functions
	_ = len(genericArgs) > 0 && checks.verifyVersionf(args[genericArgs[0]], go1_21, "implicitly instantiated function as argument")

	// tparams holds the type parameters of the callee and generic function arguments, if any:
	// the first n type parameters belong to the callee, followed by mi type parameters for each
	// of the generic function arguments, where mi = args[i].typ.(*Signature).TypeParams().Len().

	// infer missing type arguments of callee and function arguments
	if len(tparams) > 0 {
		err := checks.newError(CannotInferTypeArgs)
		targs = checks.infer(call.Pos(), tparams, targs, sigParams, args, false, err)
		if targs == nil {
			// TODO(gri) If infer inferred the first targs[:n], consider instantiating
			//           the call signature for better error messages/gopls behavior.
			//           Perhaps instantiate as much as we can, also for arguments.
			//           This will require changes to how infer returns its results.
			if !err.empty() {
				checks.errorf(err.pos(), CannotInferTypeArgs, "in call to %s, %s", call.Fun, err.msg())
			}
			return
		}

		// update result signature: instantiate if needed
		if n > 0 {
			rsig = checks.instantiateSignature(call.Pos(), call.Fun, sig, targs[:n], xlist)
			// If the callee's parameter list was adjusted we need to update (instantiate)
			// it separately. Otherwise we can simply use the result signature's parameter
			// list.
			if adjusted {
				sigParams = checks.subst(call.Pos(), sigParams, makeSubstMap(tparams[:n], targs[:n]), nil, checks.context()).(*Tuple)
			} else {
				sigParams = rsig.params
			}
		}

		// compute argument signatures: instantiate if needed
		j := n
		for _, i := range genericArgs {
			arg := args[i]
			asig := arg.typ.(*Signature)
			k := j + asig.TypeParams().Len()
			// targs[j:k] are the inferred type arguments for asig
			arg.typ = checks.instantiateSignature(call.Pos(), arg.expr, asig, targs[j:k], nil) // TODO(gri) provide xlist if possible (partial instantiations)
			checks.record(arg)                                                                 // record here because we didn't use the usual expr evaluators
			j = k
		}
	}

	// check arguments
	if len(args) > 0 {
		context := checks.sprintf("argument to %s", call.Fun)
		for i, a := range args {
			checks.assignment(a, sigParams.vars[i].typ, context)
		}
	}

	return
}

var cgoPrefixes = [...]string{
	"_Ciconst_",
	"_Cfconst_",
	"_Csconst_",
	"_Ctype_",
	"_Cvar_", // actually a pointer to the var
	"_Cfpvar_fp_",
	"_Cfunc_",
	"_Cmacro_", // function to evaluate the expanded expression
}

func (checks *Checker) selector(x *operand, e *syntax.SelectorExpr, defi *TypeName, wantType bool) {
	// these must be declared before the "goto Error" statements
	var (
		obj      Object
		index    []int
		indirect bool
	)

	sel := e.Sel.Value
	// If the identifier refers to a package, handle everything here
	// so we don't need a "package" mode for operands: package names
	// can only appear in qualified identifiers which are mapped to
	// selector expressions.
	if ident, ok := e.X.(*syntax.Name); ok {
		obj := checks.lookup(ident.Value)
		if pname, _ := obj.(*PkgName); pname != nil {
			assert(pname.pkg == checks.pkg)
			checks.recordUse(ident, pname)
			checks.usedPkgNames[pname] = true
			pkg := pname.imported

			var exp Object
			funcMode := value
			if pkg.cgo {
				// cgo special cases C.malloc: it's
				// rewritten to _CMalloc and does not
				// support two-result calls.
				if sel == "malloc" {
					sel = "_CMalloc"
				} else {
					funcMode = cgofunc
				}
				for _, prefix := range cgoPrefixes {
					// cgo objects are part of the current package (in file
					// _cgo_gotypes.go). Use regular lookup.
					exp = checks.lookup(prefix + sel)
					if exp != nil {
						break
					}
				}
				if exp == nil {
					if isValidName(sel) {
						checks.errorf(e.Sel, UndeclaredImportedName, "undefined: %s", syntax.Expr(e)) // cast to syntax.Expr to silence vet
					}
					goto Error
				}
				checks.objDecl(exp, nil)
			} else {
				exp = pkg.scope.Lookup(sel)
				if exp == nil {
					if !pkg.fake && isValidName(sel) {
						// Try to give a better error message when selector matches an object name ignoring case.
						exps := pkg.scope.lookupIgnoringCase(sel, true)
						if len(exps) >= 1 {
							// report just the first one
							checks.errorf(e.Sel, UndeclaredImportedName, "undefined: %s (but have %s)", syntax.Expr(e), exps[0].Name())
						} else {
							checks.errorf(e.Sel, UndeclaredImportedName, "undefined: %s", syntax.Expr(e))
						}
					}
					goto Error
				}
				if !exp.Exported() {
					checks.errorf(e.Sel, UnexportedName, "name %s not exported by package %s", sel, pkg.name)
					// ok to continue
				}
			}
			checks.recordUse(e.Sel, exp)

			// Simplified version of the code for *syntax.Names:
			// - imported objects are always fully initialized
			switch exp := exp.(type) {
			case *Const:
				assert(exp.Val() != nil)
				x.mode = constant_
				x.typ = exp.typ
				x.val = exp.val
			case *TypeName:
				x.mode = typexpr
				x.typ = exp.typ
			case *Var:
				x.mode = variable
				x.typ = exp.typ
				if pkg.cgo && strings.HasPrefix(exp.name, "_Cvar_") {
					x.typ = x.typ.(*Pointer).base
				}
			case *Func:
				x.mode = funcMode
				x.typ = exp.typ
				if pkg.cgo && strings.HasPrefix(exp.name, "_Cmacro_") {
					x.mode = value
					x.typ = x.typ.(*Signature).results.vars[0].typ
				}
			case *Builtin:
				x.mode = builtin
				x.typ = exp.typ
				x.id = exp.id
			default:
				checks.dump("%v: unexpected object %v", atPos(e.Sel), exp)
				panic("unreachable")
			}
			x.expr = e
			return
		}
	}

	checks.exprOrType(x, e.X, false)
	switch x.mode {
	case typexpr:
		// don't crash for "type T T.x" (was go.dev/issue/51509)
		if defi != nil && defi.typ == x.typ {
			checks.cycleError([]Object{defi}, 0)
			goto Error
		}
	case builtin:
		checks.errorf(e.Pos(), UncalledBuiltin, "invalid use of %s in selector expression", x)
		goto Error
	case invalid:
		goto Error
	}

	// Avoid crashing when checking an invalid selector in a method declaration
	// (i.e., where def is not set):
	//
	//   type S[T any] struct{}
	//   type V = S[any]
	//   func (fs *S[T]) M(x V.M) {}
	//
	// All codepaths below return a non-type expression. If we get here while
	// expecting a type expression, it is an error.
	//
	// See go.dev/issue/57522 for more details.
	//
	// TODO(rfindley): We should do better by refusing to check selectors in all cases where
	// x.typ is incomplete.
	if wantType {
		checks.errorf(e.Sel, NotAType, "%s is not a type", syntax.Expr(e))
		goto Error
	}

	obj, index, indirect = lookupFieldOrMethod(x.typ, x.mode == variable, checks.pkg, sel, false)
	if obj == nil {
		// Don't report another error if the underlying type was invalid (go.dev/issue/49541).
		if !isValid(under(x.typ)) {
			goto Error
		}

		if index != nil {
			// TODO(gri) should provide actual type where the conflict happens
			checks.errorf(e.Sel, AmbiguousSelector, "ambiguous selector %s.%s", x.expr, sel)
			goto Error
		}

		if indirect {
			if x.mode == typexpr {
				checks.errorf(e.Sel, InvalidMethodExpr, "invalid method expression %s.%s (needs pointer receiver (*%s).%s)", x.typ, sel, x.typ, sel)
			} else {
				checks.errorf(e.Sel, InvalidMethodExpr, "cannot call pointer method %s on %s", sel, x.typ)
			}
			goto Error
		}

		var why string
		if isInterfacePtr(x.typ) {
			why = checks.interfacePtrError(x.typ)
		} else {
			alt, _, _ := lookupFieldOrMethod(x.typ, x.mode == variable, checks.pkg, sel, true)
			why = checks.lookupError(x.typ, sel, alt, false)
		}
		checks.errorf(e.Sel, MissingFieldOrMethod, "%s.%s undefined (%s)", x.expr, sel, why)
		goto Error
	}

	// methods may not have a fully set up signature yet
	if m, _ := obj.(*Func); m != nil {
		checks.objDecl(m, nil)
	}

	if x.mode == typexpr {
		// method expression
		m, _ := obj.(*Func)
		if m == nil {
			checks.errorf(e.Sel, MissingFieldOrMethod, "%s.%s undefined (type %s has no method %s)", x.expr, sel, x.typ, sel)
			goto Error
		}

		checks.recordSelection(e, MethodExpr, x.typ, m, index, indirect)

		sig := m.typ.(*Signature)
		if sig.recv == nil {
			checks.error(e, InvalidDeclCycle, "illegal cycle in method declaration")
			goto Error
		}

		// The receiver type becomes the type of the first function
		// argument of the method expression's function type.
		var params []*Var
		if sig.params != nil {
			params = sig.params.vars
		}
		// Be consistent about named/unnamed parameters. This is not needed
		// for type-checking, but the newly constructed signature may appear
		// in an error message and then have mixed named/unnamed parameters.
		// (An alternative would be to not print parameter names in errors,
		// but it's useful to see them; this is cheap and method expressions
		// are rare.)
		name := ""
		if len(params) > 0 && params[0].name != "" {
			// name needed
			name = sig.recv.name
			if name == "" {
				name = "_"
			}
		}
		params = append([]*Var{NewParam(sig.recv.pos, sig.recv.pkg, name, x.typ)}, params...)
		x.mode = value
		x.typ = &Signature{
			tparams:  sig.tparams,
			params:   NewTuple(params...),
			results:  sig.results,
			variadic: sig.variadic,
		}

		checks.addDeclDep(m)

	} else {
		// regular selector
		switch obj := obj.(type) {
		case *Var:
			checks.recordSelection(e, FieldVal, x.typ, obj, index, indirect)
			if x.mode == variable || indirect {
				x.mode = variable
			} else {
				x.mode = value
			}
			x.typ = obj.typ

		case *Func:
			// TODO(gri) If we needed to take into account the receiver's
			// addressability, should we report the type &(x.typ) instead?
			checks.recordSelection(e, MethodVal, x.typ, obj, index, indirect)

			x.mode = value

			// remove receiver
			sig := *obj.typ.(*Signature)
			sig.recv = nil
			x.typ = &sig

			checks.addDeclDep(obj)

		default:
			panic("unreachable")
		}
	}

	// everything went well
	x.expr = e
	return

Error:
	x.mode = invalid
	x.expr = e
}

// use type-checks each argument.
// Useful to make sure expressions are evaluated
// (and variables are "used") in the presence of
// other errors. Arguments may be nil.
// Reports if all arguments evaluated without error.
func (checks *Checker) use(args ...syntax.Expr) bool { return checks.useN(args, false) }

// useLHS is like use, but doesn't "use" top-level identifiers.
// It should be called instead of use if the arguments are
// expressions on the lhs of an assignment.
func (checks *Checker) useLHS(args ...syntax.Expr) bool { return checks.useN(args, true) }

func (checks *Checker) useN(args []syntax.Expr, lhs bool) bool {
	ok := true
	for _, e := range args {
		if !checks.use1(e, lhs) {
			ok = false
		}
	}
	return ok
}

func (checks *Checker) use1(e syntax.Expr, lhs bool) bool {
	var x operand
	x.mode = value // anything but invalid
	switch n := syntax.Unparen(e).(type) {
	case nil:
		// nothing to do
	case *syntax.Name:
		// don't report an error evaluating blank
		if n.Value == "_" {
			break
		}
		// If the lhs is an identifier denoting a variable v, this assignment
		// is not a 'use' of v. Remember current value of v.used and restore
		// after evaluating the lhs via check.rawExpr.
		var v *Var
		var v_used bool
		if lhs {
			if obj := checks.lookup(n.Value); obj != nil {
				// It's ok to mark non-local variables, but ignore variables
				// from other packages to avoid potential race conditions with
				// dot-imported variables.
				if w, _ := obj.(*Var); w != nil && w.pkg == checks.pkg {
					v = w
					v_used = checks.usedVars[v]
				}
			}
		}
		checks.exprOrType(&x, n, true)
		if v != nil {
			checks.usedVars[v] = v_used // restore v.used
		}
	case *syntax.ListExpr:
		return checks.useN(n.ElemList, lhs)
	default:
		checks.rawExpr(nil, &x, e, nil, true)
	}
	return x.mode != invalid
}
