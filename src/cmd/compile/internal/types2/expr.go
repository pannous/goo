// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements typechecking of expressions.

package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"go/constant"
	"go/token"
	. "internal/types/errors"
)

/*
Basic algorithm:

Expressions are checked recursively, top down. Expression checker functions
are generally of the form:

  func f(x *operand, e *syntax.Expr, ...)

where e is the expression to be checked, and x is the result of the check.
The check performed by f may fail in which case x.mode == invalid, and
related error messages will have been issued by f.

If a hint argument is present, it is the composite literal element type
of an outer composite literal; it is used to type-check composite literal
elements that have no explicit type specification in the source
(e.g.: []T{{...}, {...}}, the hint is the type T in this case).

All expressions are checked via rawExpr, which dispatches according
to expression kind. Upon returning, rawExpr is recording the types and
constant values for all expressions that have an untyped type (those types
may change on the way up in the expression tree). Usually these are constants,
but the results of comparisons or non-constant shifts of untyped constants
may also be untyped, but not constant.

Untyped expressions may eventually become fully typed (i.e., not untyped),
typically when the value is assigned to a variable, or is used otherwise.
The updateExprType method is used to record this final type and update
the recorded types: the type-checked expression tree is again traversed down,
and the new type is propagated as needed. Untyped constant expression values
that become fully typed must now be representable by the full type (constant
sub-expression trees are left alone except for their roots). This mechanism
ensures that a client sees the actual (run-time) type an untyped value would
have. It also permits type-checking of lhs shift operands "as if the shift
were not present": when updateExprType visits an untyped lhs shift operand
and assigns it it's final type, that type must be an integer type, and a
constant lhs must be representable as an integer.

When an expression gets its final type, either on the way out from rawExpr,
on the way down in updateExprType, or at the end of the type checker run,
the type (and constant value, if any) is recorded via Info.Types, if present.
*/

type opPredicates map[syntax.Operator]func(Type) bool

var unaryOpPredicates opPredicates

func init() {
	// Setting unaryOpPredicates in init avoids declaration cycles.
	unaryOpPredicates = opPredicates{
		syntax.Add: allNumeric,
		syntax.Sub: allNumeric,
		syntax.Xor: allInteger,
		syntax.Not: allBoolean,
	}
}

func (checks *Checker) op(m opPredicates, x *operand, op syntax.Operator) bool {
	if pred := m[op]; pred != nil {
		if !pred(x.typ) {
			checks.errorf(x, UndefinedOp, invalidOp+"operator %s not defined on %s", op, x)
			return false
		}
	} else {
		checks.errorf(x, InvalidSyntaxTree, "unknown operator %s", op)
		return false
	}
	return true
}

// opPos returns the position of the operator if x is an operation;
// otherwise it returns the start position of x.
func opPos(x syntax.Expr) syntax.Pos {
	switch op := x.(type) {
	case nil:
		return nopos // don't crash
	case *syntax.Operation:
		return op.Pos()
	default:
		return syntax.StartPos(x)
	}
}

// opName returns the name of the operation if x is an operation
// that might overflow; otherwise it returns the empty string.
func opName(x syntax.Expr) string {
	if e, _ := x.(*syntax.Operation); e != nil {
		op := int(e.Op)
		if e.Y == nil {
			if op < len(op2str1) {
				return op2str1[op]
			}
		} else {
			if op < len(op2str2) {
				return op2str2[op]
			}
		}
	}
	return ""
}

var op2str1 = [...]string{
	syntax.Xor: "bitwise complement",
}

// This is only used for operations that may cause overflow.
var op2str2 = [...]string{
	syntax.Add: "addition",
	syntax.Sub: "subtraction",
	syntax.Xor: "bitwise XOR",
	syntax.Mul: "multiplication",
	syntax.Shl: "shift",
}

func (checks *Checker) unary(x *operand, e *syntax.Operation) {
	checks.expr(nil, x, e.X)
	if x.mode == invalid {
		return
	}

	op := e.Op
	switch op {
	case syntax.And:
		// spec: "As an exception to the addressability
		// requirement x may also be a composite literal."
		if _, ok := syntax.Unparen(e.X).(*syntax.CompositeLit); !ok && x.mode != variable {
			checks.errorf(x, UnaddressableOperand, invalidOp+"cannot take address of %s", x)
			x.mode = invalid
			return
		}
		x.mode = value
		x.typ = &Pointer{base: x.typ}
		return

	case syntax.Recv:
		if elem := checks.chanElem(x, x, true); elem != nil {
			x.mode = commaok
			x.typ = elem
			checks.hasCallOrRecv = true
			return
		}
		x.mode = invalid
		return

	case syntax.Tilde:
		// Provide a better error position and message than what check.op below would do.
		if !allInteger(x.typ) {
			checks.error(e, UndefinedOp, "cannot use ~ outside of interface or type constraint")
			x.mode = invalid
			return
		}
		checks.error(e, UndefinedOp, "cannot use ~ outside of interface or type constraint (use ^ for bitwise complement)")
		op = syntax.Xor
	}

	if !checks.op(unaryOpPredicates, x, op) {
		x.mode = invalid
		return
	}

	if x.mode == constant_ {
		if x.val.Kind() == constant.Unknown {
			// nothing to do (and don't cause an error below in the overflow check)
			return
		}
		var prec uint
		if isUnsigned(x.typ) {
			prec = uint(checks.conf.sizeof(x.typ) * 8)
		}
		x.val = constant.UnaryOp(op2tok[op], x.val, prec)
		x.expr = e
		checks.overflow(x, opPos(x.expr))
		return
	}

	x.mode = value
	// x.typ remains unchanged
}

// chanElem returns the channel element type of x for a receive from x (recv == true)
// or send to x (recv == false) operation. If the operation is not valid, chanElem
// reports an error and returns nil.
func (checks *Checker) chanElem(pos poser, x *operand, recv bool) Type {
	u, err := commonUnder(x.typ, func(t, u Type) *typeError {
		if u == nil {
			return typeErrorf("no specific channel type")
		}
		ch, _ := u.(*Chan)
		if ch == nil {
			return typeErrorf("non-channel %s", t)
		}
		if recv && ch.dir == SendOnly {
			return typeErrorf("send-only channel %s", t)
		}
		if !recv && ch.dir == RecvOnly {
			return typeErrorf("receive-only channel %s", t)
		}
		return nil
	})

	if u != nil {
		return u.(*Chan).elem
	}

	cause := err.format(checks)
	if recv {
		if isTypeParam(x.typ) {
			checks.errorf(pos, InvalidReceive, invalidOp+"cannot receive from %s: %s", x, cause)
		} else {
			// In this case, only the non-channel and send-only channel error are possible.
			checks.errorf(pos, InvalidReceive, invalidOp+"cannot receive from %s %s", cause, x)
		}
	} else {
		if isTypeParam(x.typ) {
			checks.errorf(pos, InvalidSend, invalidOp+"cannot send to %s: %s", x, cause)
		} else {
			// In this case, only the non-channel and receive-only channel error are possible.
			checks.errorf(pos, InvalidSend, invalidOp+"cannot send to %s %s", cause, x)
		}
	}
	return nil
}

func isShift(op syntax.Operator) bool {
	return op == syntax.Shl || op == syntax.Shr
}

func isComparison(op syntax.Operator) bool {
	// Note: tokens are not ordered well to make this much easier
	switch op {
	case syntax.Eql, syntax.Neq, syntax.Lss, syntax.Leq, syntax.Gtr, syntax.Geq:
		return true
	}
	return false
}

// updateExprType updates the type of x to typ and invokes itself
// recursively for the operands of x, depending on expression kind.
// If typ is still an untyped and not the final type, updateExprType
// only updates the recorded untyped type for x and possibly its
// operands. Otherwise (i.e., typ is not an untyped type anymore,
// or it is the final type for x), the type and value are recorded.
// Also, if x is a constant, it must be representable as a value of typ,
// and if x is the (formerly untyped) lhs operand of a non-constant
// shift, it must be an integer value.
func (checks *Checker) updateExprType(x syntax.Expr, typ Type, final bool) {
	old, found := checks.untyped[x]
	if !found {
		return // nothing to do
	}

	// update operands of x if necessary
	switch x := x.(type) {
	case *syntax.BadExpr,
		*syntax.FuncLit,
		*syntax.CompositeLit,
		*syntax.IndexExpr,
		*syntax.SliceExpr,
		*syntax.AssertExpr,
		*syntax.ListExpr,
		//*syntax.StarExpr,
		*syntax.KeyValueExpr,
		*syntax.ArrayType,
		*syntax.StructType,
		*syntax.FuncType,
		*syntax.InterfaceType,
		*syntax.MapType,
		*syntax.ChanType:
		// These expression are never untyped - nothing to do.
		// The respective sub-expressions got their final types
		// upon assignment or use.
		if debug {
			checks.dump("%v: found old type(%s): %s (new: %s)", atPos(x), x, old.typ, typ)
			panic("unreachable")
		}
		return

	case *syntax.CallExpr:
		// Resulting in an untyped constant (e.g., built-in complex).
		// The respective calls take care of calling updateExprType
		// for the arguments if necessary.

	case *syntax.Name, *syntax.BasicLit, *syntax.SelectorExpr:
		// An identifier denoting a constant, a constant literal,
		// or a qualified identifier (imported untyped constant).
		// No operands to take care of.

	case *syntax.ParenExpr:
		checks.updateExprType(x.X, typ, final)

	// case *syntax.UnaryExpr:
	// 	// If x is a constant, the operands were constants.
	// 	// The operands don't need to be updated since they
	// 	// never get "materialized" into a typed value. If
	// 	// left in the untyped map, they will be processed
	// 	// at the end of the type check.
	// 	if old.val != nil {
	// 		break
	// 	}
	// 	check.updateExprType(x.X, typ, final)

	case *syntax.Operation:
		if x.Y == nil {
			// unary expression
			if x.Op == syntax.Mul {
				// see commented out code for StarExpr above
				// TODO(gri) needs cleanup
				if debug {
					panic("unimplemented")
				}
				return
			}
			// If x is a constant, the operands were constants.
			// The operands don't need to be updated since they
			// never get "materialized" into a typed value. If
			// left in the untyped map, they will be processed
			// at the end of the type check.
			if old.val != nil {
				break
			}
			checks.updateExprType(x.X, typ, final)
			break
		}

		// binary expression
		if old.val != nil {
			break // see comment for unary expressions
		}
		if isComparison(x.Op) {
			// The result type is independent of operand types
			// and the operand types must have final types.
		} else if isShift(x.Op) {
			// The result type depends only on lhs operand.
			// The rhs type was updated when checking the shift.
			checks.updateExprType(x.X, typ, final)
		} else {
			// The operand types match the result type.
			checks.updateExprType(x.X, typ, final)
			checks.updateExprType(x.Y, typ, final)
		}

	default:
		panic("unreachable")
	}

	// If the new type is not final and still untyped, just
	// update the recorded type.
	if !final && isUntyped(typ) {
		old.typ = under(typ).(*Basic)
		checks.untyped[x] = old
		return
	}

	// Otherwise we have the final (typed or untyped type).
	// Remove it from the map of yet untyped expressions.
	delete(checks.untyped, x)

	if old.isLhs {
		// If x is the lhs of a shift, its final type must be integer.
		// We already know from the shift check that it is representable
		// as an integer if it is a constant.
		if !allInteger(typ) {
			checks.errorf(x, InvalidShiftOperand, invalidOp+"shifted operand %s (type %s) must be integer", x, typ)
			return
		}
		// Even if we have an integer, if the value is a constant we
		// still must check that it is representable as the specific
		// int type requested (was go.dev/issue/22969). Fall through here.
	}
	if old.val != nil {
		// If x is a constant, it must be representable as a value of typ.
		c := operand{old.mode, x, old.typ, old.val, 0}
		checks.convertUntyped(&c, typ)
		if c.mode == invalid {
			return
		}
	}

	// Everything's fine, record final type and value for x.
	checks.recordTypeAndValue(x, old.mode, typ, old.val)
}

// updateExprVal updates the value of x to val.
func (checks *Checker) updateExprVal(x syntax.Expr, val constant.Value) {
	if info, ok := checks.untyped[x]; ok {
		info.val = val
		checks.untyped[x] = info
	}
}

// implicitTypeAndValue returns the implicit type of x when used in a context
// where the target type is expected. If no such implicit conversion is
// possible, it returns a nil Type and non-zero error code.
//
// If x is a constant operand, the returned constant.Value will be the
// representation of x in this context.
func (checks *Checker) implicitTypeAndValue(x *operand, target Type) (Type, constant.Value, Code) {
	if x.mode == invalid || isTyped(x.typ) || !isValid(target) {
		return x.typ, nil, 0
	}
	// x is untyped

	if isUntyped(target) {
		// both x and target are untyped
		if m := maxType(x.typ, target); m != nil {
			return m, nil, 0
		}
		return nil, nil, InvalidUntypedConversion
	}

	if x.isNil() {
		assert(isUntyped(x.typ))
		if hasNil(target) {
			return target, nil, 0
		}
		return nil, nil, InvalidUntypedConversion
	}

	switch u := under(target).(type) {
	case *Basic:
		if x.mode == constant_ {
			v, code := checks.representation(x, u)
			if code != 0 {
				return nil, nil, code
			}
			return target, v, code
		}
		// Non-constant untyped values may appear as the
		// result of comparisons (untyped bool), intermediate
		// (delayed-checked) rhs operands of shifts, and as
		// the value nil.
		switch x.typ.(*Basic).kind {
		case UntypedBool:
			if !isBoolean(target) {
				return nil, nil, InvalidUntypedConversion
			}
		case UntypedInt, UntypedRune, UntypedFloat, UntypedComplex:
			if !isNumeric(target) {
				return nil, nil, InvalidUntypedConversion
			}
		case UntypedString:
			// Non-constant untyped string values are not permitted by the spec and
			// should not occur during normal typechecking passes, but this path is
			// reachable via the AssignableTo API.
			if !isString(target) {
				return nil, nil, InvalidUntypedConversion
			}
		default:
			return nil, nil, InvalidUntypedConversion
		}
	case *Interface:
		if isTypeParam(target) {
			if !underIs(target, func(u Type) bool {
				if u == nil {
					return false
				}
				t, _, _ := checks.implicitTypeAndValue(x, u)
				return t != nil
			}) {
				return nil, nil, InvalidUntypedConversion
			}
			break
		}
		// Update operand types to the default type rather than the target
		// (interface) type: values must have concrete dynamic types.
		// Untyped nil was handled upfront.
		if !u.Empty() {
			return nil, nil, InvalidUntypedConversion // cannot assign untyped values to non-empty interfaces
		}
		return Default(x.typ), nil, 0 // default type for nil is nil
	default:
		return nil, nil, InvalidUntypedConversion
	}
	return target, nil, 0
}

// If switchCase is true, the operator op is ignored.
func (checks *Checker) comparison(x, y *operand, op syntax.Operator, switchCase bool) {
	// Avoid spurious errors if any of the operands has an invalid type (go.dev/issue/54405).
	if !isValid(x.typ) || !isValid(y.typ) {
		x.mode = invalid
		return
	}

	if switchCase {
		op = syntax.Eql
	}

	errOp := x  // operand for which error is reported, if any
	cause := "" // specific error cause, if any

	// spec: "In any comparison, the first operand must be assignable
	// to the type of the second operand, or vice versa."
	code := MismatchedTypes
	ok, _ := x.assignableTo(checks, y.typ, nil)
	if !ok {
		ok, _ = y.assignableTo(checks, x.typ, nil)
	}
	if !ok {
		// Report the error on the 2nd operand since we only
		// know after seeing the 2nd operand whether we have
		// a type mismatch.
		errOp = y
		cause = checks.sprintf("mismatched types %s and %s", x.typ, y.typ)
		goto Error
	}

	// check if comparison is defined for operands
	code = UndefinedOp
	switch op {
	case syntax.Eql, syntax.Neq:
		// spec: "The equality operators == and != apply to operands that are comparable."
		switch {
		case x.isNil() || y.isNil():
			// Comparison against nil requires that the other operand type has nil.
			typ := x.typ
			if x.isNil() {
				typ = y.typ
			}
			if !hasNil(typ) {
				// This case should only be possible for "nil == nil".
				// Report the error on the 2nd operand since we only
				// know after seeing the 2nd operand whether we have
				// an invalid comparison.
				errOp = y
				goto Error
			}

		case !Comparable(x.typ):
			errOp = x
			cause = checks.incomparableCause(x.typ)
			goto Error

		case !Comparable(y.typ):
			errOp = y
			cause = checks.incomparableCause(y.typ)
			goto Error
		}

	case syntax.Lss, syntax.Leq, syntax.Gtr, syntax.Geq:
		// spec: The ordering operators <, <=, >, and >= apply to operands that are ordered."
		switch {
		case !allOrdered(x.typ):
			errOp = x
			goto Error
		case !allOrdered(y.typ):
			errOp = y
			goto Error
		}

	default:
		panic("unreachable")
	}

	// comparison is ok
	if x.mode == constant_ && y.mode == constant_ {
		x.val = constant.MakeBool(constant.Compare(x.val, op2tok[op], y.val))
		// The operands are never materialized; no need to update
		// their types.
	} else {
		x.mode = value
		// The operands have now their final types, which at run-
		// time will be materialized. Update the expression trees.
		// If the current types are untyped, the materialized type
		// is the respective default type.
		checks.updateExprType(x.expr, Default(x.typ), true)
		checks.updateExprType(y.expr, Default(y.typ), true)
	}

	// spec: "Comparison operators compare two operands and yield
	//        an untyped boolean value."
	x.typ = Typ[UntypedBool]
	return

Error:
	// We have an offending operand errOp and possibly an error cause.
	if cause == "" {
		if isTypeParam(x.typ) || isTypeParam(y.typ) {
			// TODO(gri) should report the specific type causing the problem, if any
			if !isTypeParam(x.typ) {
				errOp = y
			}
			cause = checks.sprintf("type parameter %s cannot use operator %s", errOp.typ, op)
		} else {
			// catch-all: neither x nor y is a type parameter
			what := compositeKind(errOp.typ)
			if what == "" {
				what = checks.sprintf("%s", errOp.typ)
			}
			cause = checks.sprintf("operator %s not defined on %s", op, what)
		}
	}
	if switchCase {
		checks.errorf(x, code, "invalid case %s in switch on %s (%s)", x.expr, y.expr, cause) // error position always at 1st operand
	} else {
		checks.errorf(errOp, code, invalidOp+"%s %s %s (%s)", x.expr, op, y.expr, cause)
	}
	x.mode = invalid
}

// incomparableCause returns a more specific cause why typ is not comparable.
// If there is no more specific cause, the result is "".
func (checks *Checker) incomparableCause(typ Type) string {
	switch under(typ).(type) {
	case *Signature, *Map:
		return compositeKind(typ) + " can only be compared to nil"
	}
	// see if we can extract a more specific error
	return comparableType(typ, true, nil).format(checks)
}

// If e != nil, it must be the shift expression; it may be nil for non-constant shifts.
func (checks *Checker) shift(x, y *operand, e syntax.Expr, op syntax.Operator) {
	// TODO(gri) This function seems overly complex. Revisit.

	var xval constant.Value
	if x.mode == constant_ {
		xval = constant.ToInt(x.val)
	}

	if allInteger(x.typ) || isUntyped(x.typ) && xval != nil && xval.Kind() == constant.Int {
		// The lhs is of integer type or an untyped constant representable
		// as an integer. Nothing to do.
	} else {
		// shift has no chance
		checks.errorf(x, InvalidShiftOperand, invalidOp+"shifted operand %s must be integer", x)
		x.mode = invalid
		return
	}

	// spec: "The right operand in a shift expression must have integer type
	// or be an untyped constant representable by a value of type uint."

	// Check that constants are representable by uint, but do not convert them
	// (see also go.dev/issue/47243).
	var yval constant.Value
	if y.mode == constant_ {
		// Provide a good error message for negative shift counts.
		yval = constant.ToInt(y.val) // consider -1, 1.0, but not -1.1
		if yval.Kind() == constant.Int && constant.Sign(yval) < 0 {
			checks.errorf(y, InvalidShiftCount, invalidOp+"negative shift count %s", y)
			x.mode = invalid
			return
		}

		if isUntyped(y.typ) {
			// Caution: Check for representability here, rather than in the switch
			// below, because isInteger includes untyped integers (was bug go.dev/issue/43697).
			checks.representable(y, Typ[Uint])
			if y.mode == invalid {
				x.mode = invalid
				return
			}
		}
	} else {
		// Check that RHS is otherwise at least of integer type.
		switch {
		case allInteger(y.typ):
			if !allUnsigned(y.typ) && !checks.verifyVersionf(y, go1_13, invalidOp+"signed shift count %s", y) {
				x.mode = invalid
				return
			}
		case isUntyped(y.typ):
			// This is incorrect, but preserves pre-existing behavior.
			// See also go.dev/issue/47410.
			checks.convertUntyped(y, Typ[Uint])
			if y.mode == invalid {
				x.mode = invalid
				return
			}
		default:
			checks.errorf(y, InvalidShiftCount, invalidOp+"shift count %s must be integer", y)
			x.mode = invalid
			return
		}
	}

	if x.mode == constant_ {
		if y.mode == constant_ {
			// if either x or y has an unknown value, the result is unknown
			if x.val.Kind() == constant.Unknown || y.val.Kind() == constant.Unknown {
				x.val = constant.MakeUnknown()
				// ensure the correct type - see comment below
				if !isInteger(x.typ) {
					x.typ = Typ[UntypedInt]
				}
				return
			}
			// rhs must be within reasonable bounds in constant shifts
			const shiftBound = 1023 - 1 + 52 // so we can express smallestFloat64 (see go.dev/issue/44057)
			s, ok := constant.Uint64Val(yval)
			if !ok || s > shiftBound {
				checks.errorf(y, InvalidShiftCount, invalidOp+"invalid shift count %s", y)
				x.mode = invalid
				return
			}
			// The lhs is representable as an integer but may not be an integer
			// (e.g., 2.0, an untyped float) - this can only happen for untyped
			// non-integer numeric constants. Correct the type so that the shift
			// result is of integer type.
			if !isInteger(x.typ) {
				x.typ = Typ[UntypedInt]
			}
			// x is a constant so xval != nil and it must be of Int kind.
			x.val = constant.Shift(xval, op2tok[op], uint(s))
			x.expr = e
			checks.overflow(x, opPos(x.expr))
			return
		}

		// non-constant shift with constant lhs
		if isUntyped(x.typ) {
			// spec: "If the left operand of a non-constant shift
			// expression is an untyped constant, the type of the
			// constant is what it would be if the shift expression
			// were replaced by its left operand alone.".
			//
			// Delay operand checking until we know the final type
			// by marking the lhs expression as lhs shift operand.
			//
			// Usually (in correct programs), the lhs expression
			// is in the untyped map. However, it is possible to
			// create incorrect programs where the same expression
			// is evaluated twice (via a declaration cycle) such
			// that the lhs expression type is determined in the
			// first round and thus deleted from the map, and then
			// not found in the second round (double insertion of
			// the same expr node still just leads to one entry for
			// that node, and it can only be deleted once).
			// Be cautious and check for presence of entry.
			// Example: var e, f = int(1<<""[f]) // go.dev/issue/11347
			if info, found := checks.untyped[x.expr]; found {
				info.isLhs = true
				checks.untyped[x.expr] = info
			}
			// keep x's type
			x.mode = value
			return
		}
	}

	// non-constant shift - lhs must be an integer
	if !allInteger(x.typ) {
		checks.errorf(x, InvalidShiftOperand, invalidOp+"shifted operand %s must be integer", x)
		x.mode = invalid
		return
	}

	x.mode = value
}

var binaryOpPredicates opPredicates

func init() {
	// Setting binaryOpPredicates in init avoids declaration cycles.
	binaryOpPredicates = opPredicates{
		syntax.Add: allNumericOrString,
		syntax.Sub: allNumeric,
		syntax.Mul: allNumeric,
		syntax.Div: allNumeric,
		syntax.Rem: allInteger,

		syntax.And:    allInteger,
		syntax.Or:     allInteger,
		syntax.Xor:    allInteger,
		syntax.AndNot: allInteger,

		syntax.AndAnd: allBoolean,
		syntax.OrOr:   allBoolean,
	}
}

// If e != nil, it must be the binary expression; it may be nil for non-constant expressions
// (when invoked for an assignment operation where the binary expression is implicit).
func (checks *Checker) binary(x *operand, e syntax.Expr, lhs, rhs syntax.Expr, op syntax.Operator) {
	var y operand

	checks.expr(nil, x, lhs)
	checks.expr(nil, &y, rhs)

	if x.mode == invalid {
		return
	}
	if y.mode == invalid {
		x.mode = invalid
		x.expr = y.expr
		return
	}

	if isShift(op) {
		checks.shift(x, &y, e, op)
		return
	}

	checks.matchTypes(x, &y)
	if x.mode == invalid {
		return
	}

	if isComparison(op) {
		checks.comparison(x, &y, op, false)
		return
	}

	// Special handling for string + number/boolean concatenation
	if op == syntax.Add && !Identical(x.typ, y.typ) {
		if allString(x.typ) && allNumeric(y.typ) {
			// String + Number: "text" + 123 -> "text123"
			if x.mode == constant_ && y.mode == constant_ {
				// Convert numeric constant to string representation
				numStr := y.val.String()
				y.val = constant.MakeString(numStr)
				y.typ = x.typ // Use the same string type as left operand
			} else {
				checks.convertUntyped(&y, Typ[String])
			}
		} else if allNumeric(x.typ) && allString(y.typ) {
			// Number + String: 123 + "text" -> "123text"  
			if x.mode == constant_ && y.mode == constant_ {
				// Convert numeric constant to string representation
				numStr := x.val.String()
				x.val = constant.MakeString(numStr)
				x.typ = y.typ // Use the same string type as right operand
			} else {
				checks.convertUntyped(x, Typ[String])
			}
		} else if allString(x.typ) && allBoolean(y.typ) {
			// String + Boolean: "prefix" + true -> "prefix✔️"
			if x.mode == constant_ && y.mode == constant_ {
				boolStr := "✖️" // false
				if constant.BoolVal(y.val) {
					boolStr = "✔️" // true
				}
				y.val = constant.MakeString(boolStr)
				y.typ = x.typ // Use the same string type as left operand
			} else {
				checks.convertUntyped(&y, Typ[String])
			}
		} else if allBoolean(x.typ) && allString(y.typ) {
			// Boolean + String: true + "suffix" -> "✔️suffix"
			if x.mode == constant_ && y.mode == constant_ {
				boolStr := "✖️" // false
				if constant.BoolVal(x.val) {
					boolStr = "✔️" // true
				}
				x.val = constant.MakeString(boolStr)
				x.typ = y.typ // Use the same string type as right operand
			} else {
				checks.convertUntyped(x, Typ[String])
			}
		}
	}

	if !Identical(x.typ, y.typ) {
		// only report an error if we have valid types
		// (otherwise we had an error reported elsewhere already)
		if isValid(x.typ) && isValid(y.typ) {
			if e != nil {
				checks.errorf(x, MismatchedTypes, invalidOp+"%s (mismatched types %s and %s)", e, x.typ, y.typ)
			} else {
				checks.errorf(x, MismatchedTypes, invalidOp+"%s %s= %s (mismatched types %s and %s)", lhs, op, rhs, x.typ, y.typ)
			}
		}
		x.mode = invalid
		return
	}

	if !checks.op(binaryOpPredicates, x, op) {
		x.mode = invalid
		return
	}

	if op == syntax.Div || op == syntax.Rem {
		// check for zero divisor
		if (x.mode == constant_ || allInteger(x.typ)) && y.mode == constant_ && constant.Sign(y.val) == 0 {
			checks.error(&y, DivByZero, invalidOp+"division by zero")
			x.mode = invalid
			return
		}

		// check for divisor underflow in complex division (see go.dev/issue/20227)
		if x.mode == constant_ && y.mode == constant_ && isComplex(x.typ) {
			re, im := constant.Real(y.val), constant.Imag(y.val)
			re2, im2 := constant.BinaryOp(re, token.MUL, re), constant.BinaryOp(im, token.MUL, im)
			if constant.Sign(re2) == 0 && constant.Sign(im2) == 0 {
				checks.error(&y, DivByZero, invalidOp+"division by zero")
				x.mode = invalid
				return
			}
		}
	}

	if x.mode == constant_ && y.mode == constant_ {
		// if either x or y has an unknown value, the result is unknown
		if x.val.Kind() == constant.Unknown || y.val.Kind() == constant.Unknown {
			x.val = constant.MakeUnknown()
			// x.typ is unchanged
			return
		}
		// force integer division for integer operands
		tok := op2tok[op]
		if op == syntax.Div && isInteger(x.typ) {
			tok = token.QUO_ASSIGN
		}
		x.val = constant.BinaryOp(x.val, tok, y.val)
		x.expr = e
		checks.overflow(x, opPos(x.expr))
		return
	}

	x.mode = value
	// x.typ is unchanged
}

// matchTypes attempts to convert any untyped types x and y such that they match.
// If an error occurs, x.mode is set to invalid.
func (checks *Checker) matchTypes(x, y *operand) {
	// mayConvert reports whether the operands x and y may
	// possibly have matching types after converting one
	// untyped operand to the type of the other.
	// If mayConvert returns true, we try to convert the
	// operands to each other's types, and if that fails
	// we report a conversion failure.
	// If mayConvert returns false, we continue without an
	// attempt at conversion, and if the operand types are
	// not compatible, we report a type mismatch error.
	mayConvert := func(x, y *operand) bool {
		// If both operands are typed, there's no need for an implicit conversion.
		if isTyped(x.typ) && isTyped(y.typ) {
			return false
		}
		// An untyped operand may convert to its default type when paired with an empty interface
		// TODO(gri) This should only matter for comparisons (the only binary operation that is
		//           valid with interfaces), but in that case the assignability check should take
		//           care of the conversion. Verify and possibly eliminate this extra test.
		if isNonTypeParamInterface(x.typ) || isNonTypeParamInterface(y.typ) {
			return true
		}
		// A boolean type can only convert to another boolean type.
		if allBoolean(x.typ) != allBoolean(y.typ) {
			return false
		}
		// A string type can only convert to another string type.
		if allString(x.typ) != allString(y.typ) {
			return false
		}
		// Untyped nil can only convert to a type that has a nil.
		if x.isNil() {
			return hasNil(y.typ)
		}
		if y.isNil() {
			return hasNil(x.typ)
		}
		// An untyped operand cannot convert to a pointer.
		// TODO(gri) generalize to type parameters
		if isPointer(x.typ) || isPointer(y.typ) {
			return false
		}
		return true
	}

	if mayConvert(x, y) {
		checks.convertUntyped(x, y.typ)
		if x.mode == invalid {
			return
		}
		checks.convertUntyped(y, x.typ)
		if y.mode == invalid {
			x.mode = invalid
			return
		}
	}
}

// exprKind describes the kind of an expression; the kind
// determines if an expression is valid in 'statement context'.
type exprKind int

const (
	conversion exprKind = iota
	expression
	statement
)

// target represent the (signature) type and description of the LHS
// variable of an assignment, or of a function result variable.
type target struct {
	sig  *Signature
	desc string
}

// newTarget creates a new target for the given type and description.
// The result is nil if typ is not a signature.
func newTarget(typ Type, desc string) *target {
	if typ != nil {
		if sig, _ := under(typ).(*Signature); sig != nil {
			return &target{sig, desc}
		}
	}
	return nil
}

// rawExpr typechecks expression e and initializes x with the expression
// value or type. If an error occurred, x.mode is set to invalid.
// If a non-nil target T is given and e is a generic function,
// T is used to infer the type arguments for e.
// If hint != nil, it is the type of a composite literal element.
// If allowGeneric is set, the operand type may be an uninstantiated
// parameterized type or function value.
func (checks *Checker) rawExpr(T *target, x *operand, e syntax.Expr, hint Type, allowGeneric bool) exprKind {
	if checks.conf.Trace {
		checks.trace(e.Pos(), "-- expr %s", e)
		checks.indent++
		defer func() {
			checks.indent--
			checks.trace(e.Pos(), "=> %s", x)
		}()
	}

	kind := checks.exprInternal(T, x, e, hint)

	if !allowGeneric {
		checks.nonGeneric(T, x)
	}

	checks.record(x)

	return kind
}

// If x is a generic type, or a generic function whose type arguments cannot be inferred
// from a non-nil target T, nonGeneric reports an error and invalidates x.mode and x.typ.
// Otherwise it leaves x alone.
func (checks *Checker) nonGeneric(T *target, x *operand) {
	if x.mode == invalid || x.mode == novalue {
		return
	}
	var what string
	switch t := x.typ.(type) {
	case *Alias, *Named:
		if isGeneric(t) {
			what = "type"
		}
	case *Signature:
		if t.tparams != nil {
			if enableReverseTypeInference && T != nil {
				checks.funcInst(T, x.Pos(), x, nil, true)
				return
			}
			what = "function"
		}
	}
	if what != "" {
		checks.errorf(x.expr, WrongTypeArgCount, "cannot use generic %s %s without instantiation", what, x.expr)
		x.mode = invalid
		x.typ = Typ[Invalid]
	}
}

// exprInternal contains the core of type checking of expressions.
// Must only be called by rawExpr.
// (See rawExpr for an explanation of the parameters.)
func (checks *Checker) exprInternal(T *target, x *operand, e syntax.Expr, hint Type) exprKind {
	// make sure x has a valid state in case of bailout
	// (was go.dev/issue/5770)
	x.mode = invalid
	x.typ = Typ[Invalid]

	switch e := e.(type) {
	case nil:
		panic("unreachable")

	case *syntax.BadExpr:
		goto Error // error was reported before

	case *syntax.Name:
		checks.ident(x, e, nil, false)

	case *syntax.DotsType:
		// dots are handled explicitly where they are valid
		checks.error(e, InvalidSyntaxTree, "invalid use of ...")
		goto Error

	case *syntax.BasicLit:
		if e.Bad {
			goto Error // error reported during parsing
		}
		checks.basicLit(x, e)
		if x.mode == invalid {
			goto Error
		}

	case *syntax.FuncLit:
		checks.funcLit(x, e)
		if x.mode == invalid {
			goto Error
		}

	case *syntax.CompositeLit:
		checks.compositeLit(x, e, hint)
		if x.mode == invalid {
			goto Error
		}

	case *syntax.ParenExpr:
		// type inference doesn't go past parentheses (target type T = nil)
		kind := checks.rawExpr(nil, x, e.X, nil, false)
		x.expr = e
		return kind

	case *syntax.SelectorExpr:
		checks.selector(x, e, nil, false)

	case *syntax.IndexExpr:
		if checks.indexExpr(x, e) {
			if !enableReverseTypeInference {
				T = nil
			}
			checks.funcInst(T, e.Pos(), x, e, true)
		}
		if x.mode == invalid {
			goto Error
		}

	case *syntax.SliceExpr:
		checks.sliceExpr(x, e)
		if x.mode == invalid {
			goto Error
		}

	case *syntax.AssertExpr:
		checks.expr(nil, x, e.X)
		if x.mode == invalid {
			goto Error
		}
		// x.(type) expressions are encoded via TypeSwitchGuards
		if e.Type == nil {
			checks.error(e, InvalidSyntaxTree, "invalid use of AssertExpr")
			goto Error
		}
		if isTypeParam(x.typ) {
			checks.errorf(x, InvalidAssert, invalidOp+"cannot use type assertion on type parameter value %s", x)
			goto Error
		}
		if _, ok := under(x.typ).(*Interface); !ok {
			checks.errorf(x, InvalidAssert, invalidOp+"%s is not an interface", x)
			goto Error
		}
		T := checks.varType(e.Type)
		if !isValid(T) {
			goto Error
		}
		checks.typeAssertion(e, x, T, false)
		x.mode = commaok
		x.typ = T

	case *syntax.TypeSwitchGuard:
		// x.(type) expressions are handled explicitly in type switches
		checks.error(e, InvalidSyntaxTree, "use of .(type) outside type switch")
		checks.use(e.X)
		goto Error

	case *syntax.CallExpr:
		return checks.callExpr(x, e)

	case *syntax.ListExpr:
		// catch-all for unexpected expression lists
		checks.error(e, InvalidSyntaxTree, "unexpected list of expressions")
		goto Error

	// case *syntax.UnaryExpr:
	// 	check.expr(x, e.X)
	// 	if x.mode == invalid {
	// 		goto Error
	// 	}
	// 	check.unary(x, e, e.Op)
	// 	if x.mode == invalid {
	// 		goto Error
	// 	}
	// 	if e.Op == token.ARROW {
	// 		x.expr = e
	// 		return statement // receive operations may appear in statement context
	// 	}

	// case *syntax.BinaryExpr:
	// 	check.binary(x, e, e.X, e.Y, e.Op)
	// 	if x.mode == invalid {
	// 		goto Error
	// 	}

	case *syntax.Operation:
		if e.Y == nil {
			// unary expression
			if e.Op == syntax.Mul {
				// pointer indirection
				checks.exprOrType(x, e.X, false)
				switch x.mode {
				case invalid:
					goto Error
				case typexpr:
					checks.validVarType(e.X, x.typ)
					x.typ = &Pointer{base: x.typ}
				default:
					var base Type
					if !underIs(x.typ, func(u Type) bool {
						p, _ := u.(*Pointer)
						if p == nil {
							checks.errorf(x, InvalidIndirection, invalidOp+"cannot indirect %s", x)
							return false
						}
						if base != nil && !Identical(p.base, base) {
							checks.errorf(x, InvalidIndirection, invalidOp+"pointers of %s must have identical base types", x)
							return false
						}
						base = p.base
						return true
					}) {
						goto Error
					}
					x.mode = variable
					x.typ = base
				}
				break
			}

			checks.unary(x, e)
			if x.mode == invalid {
				goto Error
			}
			if e.Op == syntax.Recv {
				x.expr = e
				return statement // receive operations may appear in statement context
			}
			break
		}

		// binary expression
		checks.binary(x, e, e.X, e.Y, e.Op)
		if x.mode == invalid {
			goto Error
		}

	case *syntax.KeyValueExpr:
		// key:value expressions are handled in composite literals
		checks.error(e, InvalidSyntaxTree, "no key:value expected")
		goto Error

	case *syntax.ArrayType, *syntax.SliceType, *syntax.StructType, *syntax.FuncType,
		*syntax.InterfaceType, *syntax.MapType, *syntax.ChanType:
		x.mode = typexpr
		x.typ = checks.typ(e)
		// Note: rawExpr (caller of exprInternal) will call check.recordTypeAndValue
		// even though check.typ has already called it. This is fine as both
		// times the same expression and type are recorded. It is also not a
		// performance issue because we only reach here for composite literal
		// types, which are comparatively rare.

	default:
		panic(fmt.Sprintf("%s: unknown expression type %T", atPos(e), e))
	}

	// everything went well
	x.expr = e
	return expression

Error:
	x.mode = invalid
	x.expr = e
	return statement // avoid follow-up errors
}

// keyVal maps a complex, float, integer, string or boolean constant value
// to the corresponding complex128, float64, int64, uint64, string, or bool
// Go value if possible; otherwise it returns x.
// A complex constant that can be represented as a float (such as 1.2 + 0i)
// is returned as a floating point value; if a floating point value can be
// represented as an integer (such as 1.0) it is returned as an integer value.
// This ensures that constants of different kind but equal value (such as
// 1.0 + 0i, 1.0, 1) result in the same value.
func keyVal(x constant.Value) interface{} {
	switch x.Kind() {
	case constant.Complex:
		f := constant.ToFloat(x)
		if f.Kind() != constant.Float {
			r, _ := constant.Float64Val(constant.Real(x))
			i, _ := constant.Float64Val(constant.Imag(x))
			return complex(r, i)
		}
		x = f
		fallthrough
	case constant.Float:
		i := constant.ToInt(x)
		if i.Kind() != constant.Int {
			v, _ := constant.Float64Val(x)
			return v
		}
		x = i
		fallthrough
	case constant.Int:
		if v, ok := constant.Int64Val(x); ok {
			return v
		}
		if v, ok := constant.Uint64Val(x); ok {
			return v
		}
	case constant.String:
		return constant.StringVal(x)
	case constant.Bool:
		return constant.BoolVal(x)
	}
	return x
}

// typeAssertion checks x.(T). The type of x must be an interface.
func (checks *Checker) typeAssertion(e syntax.Expr, x *operand, T Type, typeSwitch bool) {
	var cause string
	if checks.assertableTo(x.typ, T, &cause) {
		return // success
	}

	if typeSwitch {
		checks.errorf(e, ImpossibleAssert, "impossible type switch case: %s\n\t%s cannot have dynamic type %s %s", e, x, T, cause)
		return
	}

	checks.errorf(e, ImpossibleAssert, "impossible type assertion: %s\n\t%s does not implement %s %s", e, T, x.typ, cause)
}

// expr typechecks expression e and initializes x with the expression value.
// If a non-nil target T is given and e is a generic function or
// a function call, T is used to infer the type arguments for e.
// The result must be a single value.
// If an error occurred, x.mode is set to invalid.
func (checks *Checker) expr(T *target, x *operand, e syntax.Expr) {
	checks.rawExpr(T, x, e, nil, false)
	checks.exclude(x, 1<<novalue|1<<builtin|1<<typexpr)
	checks.singleValue(x)
}

// genericExpr is like expr but the result may also be generic.
func (checks *Checker) genericExpr(x *operand, e syntax.Expr) {
	checks.rawExpr(nil, x, e, nil, true)
	checks.exclude(x, 1<<novalue|1<<builtin|1<<typexpr)
	checks.singleValue(x)
}

// multiExpr typechecks e and returns its value (or values) in list.
// If allowCommaOk is set and e is a map index, comma-ok, or comma-err
// expression, the result is a two-element list containing the value
// of e, and an untyped bool value or an error value, respectively.
// If an error occurred, list[0] is not valid.
func (checks *Checker) multiExpr(e syntax.Expr, allowCommaOk bool) (list []*operand, commaOk bool) {
	var x operand
	checks.rawExpr(nil, &x, e, nil, false)
	checks.exclude(&x, 1<<novalue|1<<builtin|1<<typexpr)

	if t, ok := x.typ.(*Tuple); ok && x.mode != invalid {
		// multiple values
		list = make([]*operand, t.Len())
		for i, v := range t.vars {
			list[i] = &operand{mode: value, expr: e, typ: v.typ}
		}
		return
	}

	// exactly one (possibly invalid or comma-ok) value
	list = []*operand{&x}
	if allowCommaOk && (x.mode == mapindex || x.mode == commaok || x.mode == commaerr) {
		x2 := &operand{mode: value, expr: e, typ: Typ[UntypedBool]}
		if x.mode == commaerr {
			x2.typ = universeError
		}
		list = append(list, x2)
		commaOk = true
	}

	return
}

// exprWithHint typechecks expression e and initializes x with the expression value;
// hint is the type of a composite literal element.
// If an error occurred, x.mode is set to invalid.
func (checks *Checker) exprWithHint(x *operand, e syntax.Expr, hint Type) {
	assert(hint != nil)
	checks.rawExpr(nil, x, e, hint, false)
	checks.exclude(x, 1<<novalue|1<<builtin|1<<typexpr)
	checks.singleValue(x)
}

// exprOrType typechecks expression or type e and initializes x with the expression value or type.
// If allowGeneric is set, the operand type may be an uninstantiated parameterized type or function
// value.
// If an error occurred, x.mode is set to invalid.
func (checks *Checker) exprOrType(x *operand, e syntax.Expr, allowGeneric bool) {
	checks.rawExpr(nil, x, e, nil, allowGeneric)
	checks.exclude(x, 1<<novalue)
	checks.singleValue(x)
}

// exclude reports an error if x.mode is in modeset and sets x.mode to invalid.
// The modeset may contain any of 1<<novalue, 1<<builtin, 1<<typexpr.
func (checks *Checker) exclude(x *operand, modeset uint) {
	if modeset&(1<<x.mode) != 0 {
		var msg string
		var code Code
		switch x.mode {
		case novalue:
			if modeset&(1<<typexpr) != 0 {
				msg = "%s used as value"
			} else {
				msg = "%s used as value or type"
			}
			code = TooManyValues
		case builtin:
			msg = "%s must be called"
			code = UncalledBuiltin
		case typexpr:
			msg = "%s is not an expression"
			code = NotAnExpr
		default:
			panic("unreachable")
		}
		checks.errorf(x, code, msg, x)
		x.mode = invalid
	}
}

// singleValue reports an error if x describes a tuple and sets x.mode to invalid.
func (checks *Checker) singleValue(x *operand) {
	if x.mode == value {
		// tuple types are never named - no need for underlying type below
		if t, ok := x.typ.(*Tuple); ok {
			assert(t.Len() != 1)
			checks.errorf(x, TooManyValues, "multiple-value %s in single-value context", x)
			x.mode = invalid
		}
	}
}

// op2tok translates syntax.Operators into token.Tokens.
var op2tok = [...]token.Token{
	syntax.Def:  token.ILLEGAL,
	syntax.Not:  token.NOT,
	syntax.Recv: token.ILLEGAL,

	syntax.OrOr:   token.LOR,
	syntax.AndAnd: token.LAND,

	syntax.Eql: token.EQL,
	syntax.Neq: token.NEQ,
	syntax.Lss: token.LSS,
	syntax.Leq: token.LEQ,
	syntax.Gtr: token.GTR,
	syntax.Geq: token.GEQ,

	syntax.Add: token.ADD,
	syntax.Sub: token.SUB,
	syntax.Or:  token.OR,
	syntax.Xor: token.XOR,

	syntax.Mul:    token.MUL,
	syntax.Div:    token.QUO,
	syntax.Rem:    token.REM,
	syntax.And:    token.AND,
	syntax.AndNot: token.AND_NOT,
	syntax.Shl:    token.SHL,
	syntax.Shr:    token.SHR,
}
