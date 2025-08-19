package transforms

import "cmd/compile/internal/syntax"

// Helper function to create Name nodes with position information
func createNameWithPos(value string, pos syntax.Pos) *syntax.Name {
	name := &syntax.Name{Value: value}
	name.SetPos(pos)
	return name
}

// Helper function to create SelectorExpr with position information
func createSelectorExprWithPos(x syntax.Expr, sel string, pos syntax.Pos) *syntax.SelectorExpr {
	selName := createNameWithPos(sel, pos)
	selectorExpr := &syntax.SelectorExpr{
		X:   x,
		Sel: selName,
	}
	selectorExpr.SetPos(pos)
	return selectorExpr
}

// Helper function to create CallExpr with position information
func createCallExprWithPos(fun syntax.Expr, args []syntax.Expr, pos syntax.Pos) *syntax.CallExpr {
	callExpr := &syntax.CallExpr{
		Fun:     fun,
		ArgList: args,
	}
	callExpr.SetPos(pos)
	return callExpr
}

// Helper function to create BasicLit with position information
func createBasicLitWithPos(value string, kind syntax.LitKind, pos syntax.Pos) *syntax.BasicLit {
	lit := &syntax.BasicLit{
		Value: value,
		Kind:  kind,
	}
	lit.SetPos(pos)
	return lit
}

// Helper function to create Operation with position information
func createOperationWithPos(op syntax.Operator, x, y syntax.Expr, pos syntax.Pos) *syntax.Operation {
	operation := &syntax.Operation{
		Op: op,
		X:  x,
		Y:  y,
	}
	operation.SetPos(pos)
	return operation
}