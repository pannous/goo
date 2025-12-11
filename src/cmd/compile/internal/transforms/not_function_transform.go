//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// NotFunctionTransform converts not(expr) function calls to ! operator
// Treats 'not' as a function: not(x == y) becomes !(x == y)
type NotFunctionTransform struct{}

func (t *NotFunctionTransform) Name() string {
	return "not_function_transform"
}

func (t *NotFunctionTransform) Priority() int {
	return 50 // Run early
}

func (t *NotFunctionTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	changed := false

	// Transform top-level statements
	if len(file.TopLevelStmts) > 0 {
		for i, stmt := range file.TopLevelStmts {
			if newStmt := t.transformStmt(stmt); newStmt != stmt {
				file.TopLevelStmts[i] = newStmt
				changed = true
			}
		}
	}

	// Transform function bodies
	for i, decl := range file.DeclList {
		if funcDecl, ok := decl.(*syntax.FuncDecl); ok && funcDecl.Body != nil {
			if newBody := t.transformStmt(funcDecl.Body); newBody != funcDecl.Body {
				newFunc := *funcDecl
				newFunc.Body = newBody.(*syntax.BlockStmt)
				file.DeclList[i] = &newFunc
				changed = true
			}
		}
	}

	return changed
}

func (t *NotFunctionTransform) transformStmt(stmt syntax.Stmt) syntax.Stmt {
	if stmt == nil {
		return nil
	}

	switch s := stmt.(type) {
	case *syntax.BlockStmt:
		changed := false
		newList := make([]syntax.Stmt, len(s.List))
		for i, inner := range s.List {
			newList[i] = t.transformStmt(inner)
			if newList[i] != inner {
				changed = true
			}
		}
		if changed {
			newBlock := *s
			newBlock.List = newList
			return &newBlock
		}

	case *syntax.IfStmt:
		newCond := t.transformExpr(s.Cond)
		newInit := t.transformStmt(s.Init)
		newThen := t.transformStmt(s.Then)
		newElse := t.transformStmt(s.Else)

		if newCond != s.Cond || newInit != s.Init || newThen != s.Then || newElse != s.Else {
			newIf := *s
			newIf.Cond = newCond
			if newInit != nil {
				newIf.Init = newInit.(syntax.SimpleStmt)
			}
			newIf.Then = newThen.(*syntax.BlockStmt)
			newIf.Else = newElse
			return &newIf
		}

	case *syntax.ExprStmt:
		newExpr := t.transformExpr(s.X)
		if newExpr != s.X {
			newStmt := *s
			newStmt.X = newExpr
			return &newStmt
		}

	case *syntax.AssignStmt:
		changed := false
		newLhs := make([]syntax.Expr, len(s.Lhs))
		newRhs := make([]syntax.Expr, len(s.Rhs))
		for i, expr := range s.Lhs {
			newLhs[i] = t.transformExpr(expr)
			if newLhs[i] != expr {
				changed = true
			}
		}
		for i, expr := range s.Rhs {
			newRhs[i] = t.transformExpr(expr)
			if newRhs[i] != expr {
				changed = true
			}
		}
		if changed {
			newStmt := *s
			newStmt.Lhs = newLhs
			newStmt.Rhs = newRhs
			return &newStmt
		}

	case *syntax.ReturnStmt:
		if s.Results != nil {
			changed := false
			newResults := make([]syntax.Expr, len(s.Results))
			for i, expr := range s.Results {
				newResults[i] = t.transformExpr(expr)
				if newResults[i] != expr {
					changed = true
				}
			}
			if changed {
				newStmt := *s
				newStmt.Results = newResults
				return &newStmt
			}
		}
	}

	return stmt
}

func (t *NotFunctionTransform) transformExpr(expr syntax.Expr) syntax.Expr {
	if expr == nil {
		return nil
	}

	// Transform not(expr) to !expr
	if call, ok := expr.(*syntax.CallExpr); ok {
		if name, ok := call.Fun.(*syntax.Name); ok && name.Value == "not" {
			if len(call.ArgList) == 1 {
				notOp := new(syntax.Operation)
				notOp.Op = syntax.Not
				notOp.X = t.transformExpr(call.ArgList[0])
				notOp.SetPos(call.Pos())
				return notOp
			}
		}
		// Transform arguments of other calls
		newArgs := make([]syntax.Expr, len(call.ArgList))
		changed := false
		for i, arg := range call.ArgList {
			newArgs[i] = t.transformExpr(arg)
			if newArgs[i] != arg {
				changed = true
			}
		}
		if changed {
			newCall := *call
			newCall.ArgList = newArgs
			return &newCall
		}
	}

	// Transform operation operands
	if op, ok := expr.(*syntax.Operation); ok {
		newX := t.transformExpr(op.X)
		newY := t.transformExpr(op.Y)
		if newX != op.X || newY != op.Y {
			newOp := *op
			newOp.X = newX
			newOp.Y = newY
			return &newOp
		}
	}

	return expr
}

func init() {
	RegisterTransformer(&NotFunctionTransform{})
}
