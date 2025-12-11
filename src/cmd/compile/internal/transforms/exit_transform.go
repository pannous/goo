//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// ExitTransform converts exit() calls to os.Exit(1)
type ExitTransform struct{}

func (t *ExitTransform) Name() string {
	return "exit_transform"
}

func (t *ExitTransform) Priority() int {
	return 60
}

func (t *ExitTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	changed := false
	needsImport := false

	// Transform top-level statements
	if len(file.TopLevelStmts) > 0 {
		for i, stmt := range file.TopLevelStmts {
			newStmt, imported := t.transformStmt(stmt)
			if newStmt != stmt {
				file.TopLevelStmts[i] = newStmt
				changed = true
			}
			if imported {
				needsImport = true
			}
		}
	}

	// Transform function bodies
	for i, decl := range file.DeclList {
		if funcDecl, ok := decl.(*syntax.FuncDecl); ok && funcDecl.Body != nil {
			newBody, imported := t.transformStmt(funcDecl.Body)
			if newBody != funcDecl.Body {
				newFunc := *funcDecl
				newFunc.Body = newBody.(*syntax.BlockStmt)
				file.DeclList[i] = &newFunc
				changed = true
			}
			if imported {
				needsImport = true
			}
		}
	}

	if needsImport {
		addImportIfMissing(file, "os")
	}

	return changed
}

func (t *ExitTransform) transformStmt(stmt syntax.Stmt) (syntax.Stmt, bool) {
	if stmt == nil {
		return nil, false
	}

	switch s := stmt.(type) {
	case *syntax.BlockStmt:
		changed := false
		imported := false
		newList := make([]syntax.Stmt, len(s.List))
		for i, inner := range s.List {
			var imp bool
			newList[i], imp = t.transformStmt(inner)
			if newList[i] != inner {
				changed = true
			}
			if imp {
				imported = true
			}
		}
		if changed {
			newBlock := *s
			newBlock.List = newList
			return &newBlock, imported
		}
		return s, imported

	case *syntax.ExprStmt:
		var imported bool
		var newExpr syntax.Expr
		newExpr, imported = t.transformExpr(s.X)
		if newExpr != s.X {
			newStmt := *s
			newStmt.X = newExpr
			return &newStmt, imported
		}
		return s, imported

	case *syntax.IfStmt:
		var imported, imp1, imp2 bool
		var newCond syntax.Expr
		var newThen, newElse syntax.Stmt
		newCond, imported = t.transformExpr(s.Cond)
		newThen, imp1 = t.transformStmt(s.Then)
		newElse, imp2 = t.transformStmt(s.Else)

		if newCond != s.Cond || newThen != s.Then || newElse != s.Else {
			newIf := *s
			newIf.Cond = newCond
			newIf.Then = newThen.(*syntax.BlockStmt)
			newIf.Else = newElse
			return &newIf, imported || imp1 || imp2
		}
		return s, imported || imp1 || imp2
	}

	return stmt, false
}

func (t *ExitTransform) transformExpr(expr syntax.Expr) (syntax.Expr, bool) {
	if expr == nil {
		return nil, false
	}

	if call, ok := expr.(*syntax.CallExpr); ok {
		if name, ok := call.Fun.(*syntax.Name); ok && name.Value == "exit" {
			// Convert exit() to os.Exit(1)
			osName := &syntax.Name{Value: "os"}
			osName.SetPos(call.Pos())

			exitSel := &syntax.SelectorExpr{
				X:   osName,
				Sel: &syntax.Name{Value: "Exit"},
			}
			exitSel.SetPos(call.Pos())

			oneLit := &syntax.BasicLit{
				Value: "1",
				Kind:  syntax.IntLit,
			}
			oneLit.SetPos(call.Pos())

			newCall := *call
			newCall.Fun = exitSel
			newCall.ArgList = []syntax.Expr{oneLit}

			return &newCall, true
		}
	}

	return expr, false
}

func init() {
	RegisterTransformer(&ExitTransform{})
}
