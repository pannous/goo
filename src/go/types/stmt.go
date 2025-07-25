// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements typechecking of statements.

package types

import (
	"go/ast"
	"go/constant"
	"go/token"
	. "internal/types/errors"
	"slices"
)

// decl may be nil
func (checks *Checker) funcBody(decl *declInfo, name string, sig *Signature, body *ast.BlockStmt, iota constant.Value) {
	if checks.conf.IgnoreFuncBodies {
		panic("function body not ignored")
	}

	if checks.conf._Trace {
		checks.trace(body.Pos(), "-- %s: %s", name, sig)
	}

	// save/restore current environment and set up function environment
	// (and use 0 indentation at function start)
	defer func(env environment, indent int) {
		checks.environment = env
		checks.indent = indent
	}(checks.environment, checks.indent)
	checks.environment = environment{
		decl:    decl,
		scope:   sig.scope,
		version: checks.version, // TODO(adonovan): would decl.version (if decl != nil) be better?
		iota:    iota,
		sig:     sig,
	}
	checks.indent = 0

	checks.stmtList(0, body.List)

	if checks.hasLabel {
		checks.labels(body)
	}

	if sig.results.Len() > 0 && !checks.isTerminating(body, "") {
		checks.error(atPos(body.Rbrace), MissingReturn, "missing return")
	}

	// spec: "Implementation restriction: A compiler may make it illegal to
	// declare a variable inside a function body if the variable is never used."
	checks.usage(sig.scope)
}

func (checks *Checker) usage(scope *Scope) {
	needUse := func(kind VarKind) bool {
		return !(kind == RecvVar || kind == ParamVar || kind == ResultVar)
	}
	var unused []*Var
	for name, elem := range scope.elems {
		elem = resolve(name, elem)
		if v, _ := elem.(*Var); v != nil && needUse(v.kind) && !checks.usedVars[v] {
			unused = append(unused, v)
		}
	}
	slices.SortFunc(unused, func(a, b *Var) int {
		return cmpPos(a.pos, b.pos)
	})
	for _, v := range unused {
		checks.softErrorf(v, UnusedVar, "declared and not used: %s", v.name)
	}

	for _, scope := range scope.children {
		// Don't go inside function literal scopes a second time;
		// they are handled explicitly by funcBody.
		if !scope.isFunc {
			checks.usage(scope)
		}
	}
}

// stmtContext is a bitset describing which
// control-flow statements are permissible,
// and provides additional context information
// for better error messages.
type stmtContext uint

const (
	// permissible control-flow statements
	breakOk stmtContext = 1 << iota
	continueOk
	fallthroughOk

	// additional context information
	finalSwitchCase
	inTypeSwitch
)

func (checks *Checker) simpleStmt(s ast.Stmt) {
	if s != nil {
		checks.stmt(0, s)
	}
}

func trimTrailingEmptyStmts(list []ast.Stmt) []ast.Stmt {
	for i := len(list); i > 0; i-- {
		if _, ok := list[i-1].(*ast.EmptyStmt); !ok {
			return list[:i]
		}
	}
	return nil
}

func (checks *Checker) stmtList(ctxt stmtContext, list []ast.Stmt) {
	ok := ctxt&fallthroughOk != 0
	inner := ctxt &^ fallthroughOk
	list = trimTrailingEmptyStmts(list) // trailing empty statements are "invisible" to fallthrough analysis
	for i, s := range list {
		inner := inner
		if ok && i+1 == len(list) {
			inner |= fallthroughOk
		}
		checks.stmt(inner, s)
	}
}

func (checks *Checker) multipleDefaults(list []ast.Stmt) {
	var first ast.Stmt
	for _, s := range list {
		var d ast.Stmt
		switch c := s.(type) {
		case *ast.CaseClause:
			if len(c.List) == 0 {
				d = s
			}
		case *ast.CommClause:
			if c.Comm == nil {
				d = s
			}
		default:
			checks.error(s, InvalidSyntaxTree, "case/communication clause expected")
		}
		if d != nil {
			if first != nil {
				checks.errorf(d, DuplicateDefault, "multiple defaults (first at %s)", checks.fset.Position(first.Pos()))
			} else {
				first = d
			}
		}
	}
}

func (checks *Checker) openScope(node ast.Node, comment string) {
	scope := NewScope(checks.scope, node.Pos(), node.End(), comment)
	checks.recordScope(node, scope)
	checks.scope = scope
}

func (checks *Checker) closeScope() {
	checks.scope = checks.scope.Parent()
}

func assignOp(op token.Token) token.Token {
	// token_test.go verifies the token ordering this function relies on
	if token.ADD_ASSIGN <= op && op <= token.AND_NOT_ASSIGN {
		return op + (token.ADD - token.ADD_ASSIGN)
	}
	return token.ILLEGAL
}

func (checks *Checker) suspendedCall(keyword string, call *ast.CallExpr) {
	var x operand
	var msg string
	var code Code
	switch checks.rawExpr(nil, &x, call, nil, false) {
	case conversion:
		msg = "requires function call, not conversion"
		code = InvalidDefer
		if keyword == "go" {
			code = InvalidGo
		}
	case expression:
		msg = "discards result of"
		code = UnusedResults
	case statement:
		return
	default:
		panic("unreachable")
	}
	checks.errorf(&x, code, "%s %s %s", keyword, msg, &x)
}

// goVal returns the Go value for val, or nil.
func goVal(val constant.Value) any {
	// val should exist, but be conservative and check
	if val == nil {
		return nil
	}
	// Match implementation restriction of other compilers.
	// gc only checks duplicates for integer, floating-point
	// and string values, so only create Go values for these
	// types.
	switch val.Kind() {
	case constant.Int:
		if x, ok := constant.Int64Val(val); ok {
			return x
		}
		if x, ok := constant.Uint64Val(val); ok {
			return x
		}
	case constant.Float:
		if x, ok := constant.Float64Val(val); ok {
			return x
		}
	case constant.String:
		return constant.StringVal(val)
	}
	return nil
}

// A valueMap maps a case value (of a basic Go type) to a list of positions
// where the same case value appeared, together with the corresponding case
// types.
// Since two case values may have the same "underlying" value but different
// types we need to also check the value's types (e.g., byte(1) vs myByte(1))
// when the switch expression is of interface type.
type (
	valueMap  map[any][]valueType // underlying Go value -> valueType
	valueType struct {
		pos token.Pos
		typ Type
	}
)

func (checks *Checker) caseValues(x *operand, values []ast.Expr, seen valueMap) {
L:
	for _, e := range values {
		var v operand
		checks.expr(nil, &v, e)
		if x.mode == invalid || v.mode == invalid {
			continue L
		}
		checks.convertUntyped(&v, x.typ)
		if v.mode == invalid {
			continue L
		}
		// Order matters: By comparing v against x, error positions are at the case values.
		res := v // keep original v unchanged
		checks.comparison(&res, x, token.EQL, true)
		if res.mode == invalid {
			continue L
		}
		if v.mode != constant_ {
			continue L // we're done
		}
		// look for duplicate values
		if val := goVal(v.val); val != nil {
			// look for duplicate types for a given value
			// (quadratic algorithm, but these lists tend to be very short)
			for _, vt := range seen[val] {
				if Identical(v.typ, vt.typ) {
					err := checks.newError(DuplicateCase)
					err.addf(&v, "duplicate case %s in expression switch", &v)
					err.addf(atPos(vt.pos), "previous case")
					err.report()
					continue L
				}
			}
			seen[val] = append(seen[val], valueType{v.Pos(), v.typ})
		}
	}
}

// isNil reports whether the expression e denotes the predeclared value nil.
func (checks *Checker) isNil(e ast.Expr) bool {
	// The only way to express the nil value is by literally writing nil (possibly in parentheses).
	if name, _ := ast.Unparen(e).(*ast.Ident); name != nil {
		_, ok := checks.lookup(name.Name).(*Nil)
		return ok
	}
	return false
}

// caseTypes typechecks the type expressions of a type case, checks for duplicate types
// using the seen map, and verifies that each type is valid with respect to the type of
// the operand x corresponding to the type switch expression. If that expression is not
// valid, x must be nil.
//
//	switch <x>.(type) {
//	case <types>: ...
//	...
//	}
//
// caseTypes returns the case-specific type for a variable v introduced through a short
// variable declaration by the type switch:
//
//	switch v := <x>.(type) {
//	case <types>: // T is the type of <v> in this case
//	...
//	}
//
// If there is exactly one type expression, T is the type of that expression. If there
// are multiple type expressions, or if predeclared nil is among the types, the result
// is the type of x. If x is invalid (nil), the result is the invalid type.
func (checks *Checker) caseTypes(x *operand, types []ast.Expr, seen map[Type]ast.Expr) Type {
	var T Type
	var dummy operand
L:
	for _, e := range types {
		// The spec allows the value nil instead of a type.
		if checks.isNil(e) {
			T = nil
			checks.expr(nil, &dummy, e) // run e through expr so we get the usual Info recordings
		} else {
			T = checks.varType(e)
			if !isValid(T) {
				continue L
			}
		}
		// look for duplicate types
		// (quadratic algorithm, but type switches tend to be reasonably small)
		for t, other := range seen {
			if T == nil && t == nil || T != nil && t != nil && Identical(T, t) {
				// talk about "case" rather than "type" because of nil case
				Ts := "nil"
				if T != nil {
					Ts = TypeString(T, checks.qualifier)
				}
				err := checks.newError(DuplicateCase)
				err.addf(e, "duplicate case %s in type switch", Ts)
				err.addf(other, "previous case")
				err.report()
				continue L
			}
		}
		seen[T] = e
		if x != nil && T != nil {
			checks.typeAssertion(e, x, T, true)
		}
	}

	// spec: "In clauses with a case listing exactly one type, the variable has that type;
	// otherwise, the variable has the type of the expression in the TypeSwitchGuard.
	if len(types) != 1 || T == nil {
		T = Typ[Invalid]
		if x != nil {
			T = x.typ
		}
	}

	assert(T != nil)
	return T
}

// TODO(gri) Once we are certain that typeHash is correct in all situations, use this version of caseTypes instead.
// (Currently it may be possible that different types have identical names and import paths due to ImporterFrom.)
func (checks *Checker) caseTypes_currently_unused(x *operand, xtyp *Interface, types []ast.Expr, seen map[string]ast.Expr) Type {
	var T Type
	var dummy operand
L:
	for _, e := range types {
		// The spec allows the value nil instead of a type.
		var hash string
		if checks.isNil(e) {
			checks.expr(nil, &dummy, e) // run e through expr so we get the usual Info recordings
			T = nil
			hash = "<nil>" // avoid collision with a type named nil
		} else {
			T = checks.varType(e)
			if !isValid(T) {
				continue L
			}
			panic("enable typeHash(T, nil)")
			// hash = typeHash(T, nil)
		}
		// look for duplicate types
		if other := seen[hash]; other != nil {
			// talk about "case" rather than "type" because of nil case
			Ts := "nil"
			if T != nil {
				Ts = TypeString(T, checks.qualifier)
			}
			err := checks.newError(DuplicateCase)
			err.addf(e, "duplicate case %s in type switch", Ts)
			err.addf(other, "previous case")
			err.report()
			continue L
		}
		seen[hash] = e
		if T != nil {
			checks.typeAssertion(e, x, T, true)
		}
	}

	// spec: "In clauses with a case listing exactly one type, the variable has that type;
	// otherwise, the variable has the type of the expression in the TypeSwitchGuard.
	if len(types) != 1 || T == nil {
		T = Typ[Invalid]
		if x != nil {
			T = x.typ
		}
	}

	assert(T != nil)
	return T
}

// stmt typechecks statement s.
func (checks *Checker) stmt(ctxt stmtContext, s ast.Stmt) {
	// statements must end with the same top scope as they started with
	if debug {
		defer func(scope *Scope) {
			// don't check if code is panicking
			if p := recover(); p != nil {
				panic(p)
			}
			assert(scope == checks.scope)
		}(checks.scope)
	}

	// process collected function literals before scope changes
	defer checks.processDelayed(len(checks.delayed))

	// reset context for statements of inner blocks
	inner := ctxt &^ (fallthroughOk | finalSwitchCase | inTypeSwitch)

	switch s := s.(type) {
	case *ast.BadStmt, *ast.EmptyStmt:
		// ignore

	case *ast.DeclStmt:
		checks.declStmt(s.Decl)

	case *ast.LabeledStmt:
		checks.hasLabel = true
		checks.stmt(ctxt, s.Stmt)

	case *ast.ExprStmt:
		// spec: "With the exception of specific built-in functions,
		// function and method calls and receive operations can appear
		// in statement context. Such statements may be parenthesized."
		var x operand
		kind := checks.rawExpr(nil, &x, s.X, nil, false)
		var msg string
		var code Code
		switch x.mode {
		default:
			if kind == statement {
				return
			}
			msg = "is not used"
			code = UnusedExpr
		case builtin:
			msg = "must be called"
			code = UncalledBuiltin
		case typexpr:
			msg = "is not an expression"
			code = NotAnExpr
		}
		checks.errorf(&x, code, "%s %s", &x, msg)

	case *ast.SendStmt:
		var ch, val operand
		checks.expr(nil, &ch, s.Chan)
		checks.expr(nil, &val, s.Value)
		if ch.mode == invalid || val.mode == invalid {
			return
		}
		if elem := checks.chanElem(inNode(s, s.Arrow), &ch, false); elem != nil {
			checks.assignment(&val, elem, "send")
		}

	case *ast.IncDecStmt:
		var op token.Token
		switch s.Tok {
		case token.INC:
			op = token.ADD
		case token.DEC:
			op = token.SUB
		default:
			checks.errorf(inNode(s, s.TokPos), InvalidSyntaxTree, "unknown inc/dec operation %s", s.Tok)
			return
		}

		var x operand
		checks.expr(nil, &x, s.X)
		if x.mode == invalid {
			return
		}
		if !allNumeric(x.typ) {
			checks.errorf(s.X, NonNumericIncDec, invalidOp+"%s%s (non-numeric type %s)", s.X, s.Tok, x.typ)
			return
		}

		Y := &ast.BasicLit{ValuePos: s.X.Pos(), Kind: token.INT, Value: "1"} // use x's position
		checks.binary(&x, nil, s.X, Y, op, s.TokPos)
		if x.mode == invalid {
			return
		}
		checks.assignVar(s.X, nil, &x, "assignment")

	case *ast.AssignStmt:
		switch s.Tok {
		case token.ASSIGN, token.DEFINE:
			if len(s.Lhs) == 0 {
				checks.error(s, InvalidSyntaxTree, "missing lhs in assignment")
				return
			}
			if s.Tok == token.DEFINE {
				checks.shortVarDecl(inNode(s, s.TokPos), s.Lhs, s.Rhs)
			} else {
				// regular assignment
				checks.assignVars(s.Lhs, s.Rhs)
			}

		default:
			// assignment operations
			if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
				checks.errorf(inNode(s, s.TokPos), MultiValAssignOp, "assignment operation %s requires single-valued expressions", s.Tok)
				return
			}
			op := assignOp(s.Tok)
			if op == token.ILLEGAL {
				checks.errorf(atPos(s.TokPos), InvalidSyntaxTree, "unknown assignment operation %s", s.Tok)
				return
			}
			var x operand
			checks.binary(&x, nil, s.Lhs[0], s.Rhs[0], op, s.TokPos)
			if x.mode == invalid {
				return
			}
			checks.assignVar(s.Lhs[0], nil, &x, "assignment")
		}

	case *ast.GoStmt:
		checks.suspendedCall("go", s.Call)

	case *ast.DeferStmt:
		checks.suspendedCall("defer", s.Call)

	case *ast.ReturnStmt:
		res := checks.sig.results
		// Return with implicit results allowed for function with named results.
		// (If one is named, all are named.)
		if len(s.Results) == 0 && res.Len() > 0 && res.vars[0].name != "" {
			// spec: "Implementation restriction: A compiler may disallow an empty expression
			// list in a "return" statement if a different entity (constant, type, or variable)
			// with the same name as a result parameter is in scope at the place of the return."
			for _, obj := range res.vars {
				if alt := checks.lookup(obj.name); alt != nil && alt != obj {
					err := checks.newError(OutOfScopeResult)
					err.addf(s, "result parameter %s not in scope at return", obj.name)
					err.addf(alt, "inner declaration of %s", obj)
					err.report()
					// ok to continue
				}
			}
		} else {
			var lhs []*Var
			if res.Len() > 0 {
				lhs = res.vars
			}
			checks.initVars(lhs, s.Results, s)
		}

	case *ast.BranchStmt:
		if s.Label != nil {
			checks.hasLabel = true
			return // checked in 2nd pass (check.labels)
		}
		switch s.Tok {
		case token.BREAK:
			if ctxt&breakOk == 0 {
				checks.error(s, MisplacedBreak, "break not in for, switch, or select statement")
			}
		case token.CONTINUE:
			if ctxt&continueOk == 0 {
				checks.error(s, MisplacedContinue, "continue not in for statement")
			}
		case token.FALLTHROUGH:
			if ctxt&fallthroughOk == 0 {
				var msg string
				switch {
				case ctxt&finalSwitchCase != 0:
					msg = "cannot fallthrough final case in switch"
				case ctxt&inTypeSwitch != 0:
					msg = "cannot fallthrough in type switch"
				default:
					msg = "fallthrough statement out of place"
				}
				checks.error(s, MisplacedFallthrough, msg)
			}
		default:
			checks.errorf(s, InvalidSyntaxTree, "branch statement: %s", s.Tok)
		}

	case *ast.BlockStmt:
		checks.openScope(s, "block")
		defer checks.closeScope()

		checks.stmtList(inner, s.List)

	case *ast.IfStmt:
		checks.openScope(s, "if")
		defer checks.closeScope()

		checks.simpleStmt(s.Init)
		var x operand
		checks.expr(nil, &x, s.Cond)
		if x.mode != invalid && !allBoolean(x.typ) {
			checks.error(s.Cond, InvalidCond, "non-boolean condition in if statement")
		}
		checks.stmt(inner, s.Body)
		// The parser produces a correct AST but if it was modified
		// elsewhere the else branch may be invalid. Check again.
		switch s.Else.(type) {
		case nil, *ast.BadStmt:
			// valid or error already reported
		case *ast.IfStmt, *ast.BlockStmt:
			checks.stmt(inner, s.Else)
		default:
			checks.error(s.Else, InvalidSyntaxTree, "invalid else branch in if statement")
		}

	case *ast.SwitchStmt:
		inner |= breakOk
		checks.openScope(s, "switch")
		defer checks.closeScope()

		checks.simpleStmt(s.Init)
		var x operand
		if s.Tag != nil {
			checks.expr(nil, &x, s.Tag)
			// By checking assignment of x to an invisible temporary
			// (as a compiler would), we get all the relevant checks.
			checks.assignment(&x, nil, "switch expression")
			if x.mode != invalid && !Comparable(x.typ) && !hasNil(x.typ) {
				checks.errorf(&x, InvalidExprSwitch, "cannot switch on %s (%s is not comparable)", &x, x.typ)
				x.mode = invalid
			}
		} else {
			// spec: "A missing switch expression is
			// equivalent to the boolean value true."
			x.mode = constant_
			x.typ = Typ[Bool]
			x.val = constant.MakeBool(true)
			x.expr = &ast.Ident{NamePos: s.Body.Lbrace, Name: "true"}
		}

		checks.multipleDefaults(s.Body.List)

		seen := make(valueMap) // map of seen case values to positions and types
		for i, c := range s.Body.List {
			clause, _ := c.(*ast.CaseClause)
			if clause == nil {
				checks.error(c, InvalidSyntaxTree, "incorrect expression switch case")
				continue
			}
			checks.caseValues(&x, clause.List, seen)
			checks.openScope(clause, "case")
			inner := inner
			if i+1 < len(s.Body.List) {
				inner |= fallthroughOk
			} else {
				inner |= finalSwitchCase
			}
			checks.stmtList(inner, clause.Body)
			checks.closeScope()
		}

	case *ast.TypeSwitchStmt:
		inner |= breakOk | inTypeSwitch
		checks.openScope(s, "type switch")
		defer checks.closeScope()

		checks.simpleStmt(s.Init)

		// A type switch guard must be of the form:
		//
		//     TypeSwitchGuard = [ identifier ":=" ] PrimaryExpr "." "(" "type" ")" .
		//
		// The parser is checking syntactic correctness;
		// remaining syntactic errors are considered AST errors here.
		// TODO(gri) better factoring of error handling (invalid ASTs)
		//
		var lhs *ast.Ident // lhs identifier or nil
		var rhs ast.Expr
		switch guard := s.Assign.(type) {
		case *ast.ExprStmt:
			rhs = guard.X
		case *ast.AssignStmt:
			if len(guard.Lhs) != 1 || guard.Tok != token.DEFINE || len(guard.Rhs) != 1 {
				checks.error(s, InvalidSyntaxTree, "incorrect form of type switch guard")
				return
			}

			lhs, _ = guard.Lhs[0].(*ast.Ident)
			if lhs == nil {
				checks.error(s, InvalidSyntaxTree, "incorrect form of type switch guard")
				return
			}

			if lhs.Name == "_" {
				// _ := x.(type) is an invalid short variable declaration
				checks.softErrorf(lhs, NoNewVar, "no new variable on left side of :=")
				lhs = nil // avoid declared and not used error below
			} else {
				checks.recordDef(lhs, nil) // lhs variable is implicitly declared in each cause clause
			}

			rhs = guard.Rhs[0]

		default:
			checks.error(s, InvalidSyntaxTree, "incorrect form of type switch guard")
			return
		}

		// rhs must be of the form: expr.(type) and expr must be an ordinary interface
		expr, _ := rhs.(*ast.TypeAssertExpr)
		if expr == nil || expr.Type != nil {
			checks.error(s, InvalidSyntaxTree, "incorrect form of type switch guard")
			return
		}

		var sx *operand // switch expression against which cases are compared against; nil if invalid
		{
			var x operand
			checks.expr(nil, &x, expr.X)
			if x.mode != invalid {
				if isTypeParam(x.typ) {
					checks.errorf(&x, InvalidTypeSwitch, "cannot use type switch on type parameter value %s", &x)
				} else if IsInterface(x.typ) {
					sx = &x
				} else {
					checks.errorf(&x, InvalidTypeSwitch, "%s is not an interface", &x)
				}
			}
		}

		checks.multipleDefaults(s.Body.List)

		var lhsVars []*Var              // list of implicitly declared lhs variables
		seen := make(map[Type]ast.Expr) // map of seen types to positions
		for _, s := range s.Body.List {
			clause, _ := s.(*ast.CaseClause)
			if clause == nil {
				checks.error(s, InvalidSyntaxTree, "incorrect type switch case")
				continue
			}
			// Check each type in this type switch case.
			T := checks.caseTypes(sx, clause.List, seen)
			checks.openScope(clause, "case")
			// If lhs exists, declare a corresponding variable in the case-local scope.
			if lhs != nil {
				obj := newVar(LocalVar, lhs.Pos(), checks.pkg, lhs.Name, T)
				checks.declare(checks.scope, nil, obj, clause.Colon)
				checks.recordImplicit(clause, obj)
				// For the "declared and not used" error, all lhs variables act as
				// one; i.e., if any one of them is 'used', all of them are 'used'.
				// Collect them for later analysis.
				lhsVars = append(lhsVars, obj)
			}
			checks.stmtList(inner, clause.Body)
			checks.closeScope()
		}

		// If lhs exists, we must have at least one lhs variable that was used.
		// (We can't use check.usage because that only looks at one scope; and
		// we don't want to use the same variable for all scopes and change the
		// variable type underfoot.)
		if lhs != nil {
			var used bool
			for _, v := range lhsVars {
				if checks.usedVars[v] {
					used = true
				}
				checks.usedVars[v] = true // avoid usage error when checking entire function
			}
			if !used {
				checks.softErrorf(lhs, UnusedVar, "%s declared and not used", lhs.Name)
			}
		}

	case *ast.SelectStmt:
		inner |= breakOk

		checks.multipleDefaults(s.Body.List)

		for _, s := range s.Body.List {
			clause, _ := s.(*ast.CommClause)
			if clause == nil {
				continue // error reported before
			}

			// clause.Comm must be a SendStmt, RecvStmt, or default case
			valid := false
			var rhs ast.Expr // rhs of RecvStmt, or nil
			switch s := clause.Comm.(type) {
			case nil, *ast.SendStmt:
				valid = true
			case *ast.AssignStmt:
				if len(s.Rhs) == 1 {
					rhs = s.Rhs[0]
				}
			case *ast.ExprStmt:
				rhs = s.X
			}

			// if present, rhs must be a receive operation
			if rhs != nil {
				if x, _ := ast.Unparen(rhs).(*ast.UnaryExpr); x != nil && x.Op == token.ARROW {
					valid = true
				}
			}

			if !valid {
				checks.error(clause.Comm, InvalidSelectCase, "select case must be send or receive (possibly with assignment)")
				continue
			}

			checks.openScope(s, "case")
			if clause.Comm != nil {
				checks.stmt(inner, clause.Comm)
			}
			checks.stmtList(inner, clause.Body)
			checks.closeScope()
		}

	case *ast.ForStmt:
		inner |= breakOk | continueOk
		checks.openScope(s, "for")
		defer checks.closeScope()

		checks.simpleStmt(s.Init)
		if s.Cond != nil {
			var x operand
			checks.expr(nil, &x, s.Cond)
			if x.mode != invalid && !allBoolean(x.typ) {
				checks.error(s.Cond, InvalidCond, "non-boolean condition in for statement")
			}
		}
		checks.simpleStmt(s.Post)
		// spec: "The init statement may be a short variable
		// declaration, but the post statement must not."
		if s, _ := s.Post.(*ast.AssignStmt); s != nil && s.Tok == token.DEFINE {
			checks.softErrorf(s, InvalidPostDecl, "cannot declare in post statement")
			// Don't call useLHS here because we want to use the lhs in
			// this erroneous statement so that we don't get errors about
			// these lhs variables being declared and not used.
			checks.use(s.Lhs...) // avoid follow-up errors
		}
		checks.stmt(inner, s.Body)

	case *ast.RangeStmt:
		inner |= breakOk | continueOk
		checks.rangeStmt(inner, s, inNode(s, s.TokPos), s.Key, s.Value, nil, s.X, s.Tok == token.DEFINE)

	default:
		checks.error(s, InvalidSyntaxTree, "invalid statement")
	}
}
