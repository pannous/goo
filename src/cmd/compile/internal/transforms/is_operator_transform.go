//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// IsOperatorTransform handles the 'is' type checking operator
// Transforms expressions like "x is int" to type checking function calls
type IsOperatorTransform struct{}

type isOperatorVisitor struct {
	transform *IsOperatorTransform
	ctx       *TransformContext
	changed   bool
}

func (t *IsOperatorTransform) Name() string {
	return "is_operator_transform"
}

func (t *IsOperatorTransform) Priority() int {
	return 30 // Run relatively early, before most other transforms
}

func (t *IsOperatorTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &isOperatorVisitor{transform: t, ctx: ctx}
	syntax.Walk(file, visitor)
	return visitor.changed
}

// Visit implements syntax.Visitor interface
func (v *isOperatorVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}

	// Look for nodes that contain IS operations we can replace
	switch n := node.(type) {
	case *syntax.ExprStmt:
		if isOp, ok := n.X.(*syntax.Operation); ok && isOp.Op == syntax.IS {
			if newExpr := v.transform.convertIsToFunctionCall(isOp); newExpr != isOp {
				n.X = newExpr
				v.changed = true
			}
		}
	case *syntax.AssignStmt:
		if isOp, ok := n.Rhs.(*syntax.Operation); ok && isOp.Op == syntax.IS {
			if newExpr := v.transform.convertIsToFunctionCall(isOp); newExpr != isOp {
				n.Rhs = newExpr
				v.changed = true
			}
		}
	case *syntax.Operation:
		// Handle IS operations inside other operations
		if isOp, ok := n.X.(*syntax.Operation); ok && isOp.Op == syntax.IS {
			if newExpr := v.transform.convertIsToFunctionCall(isOp); newExpr != isOp {
				n.X = newExpr
				v.changed = true
			}
		}
		if isOp, ok := n.Y.(*syntax.Operation); ok && isOp.Op == syntax.IS {
			if newExpr := v.transform.convertIsToFunctionCall(isOp); newExpr != isOp {
				n.Y = newExpr
				v.changed = true
			}
		}
	case *syntax.IfStmt:
		if isOp, ok := n.Cond.(*syntax.Operation); ok && isOp.Op == syntax.IS {
			if newExpr := v.transform.convertIsToFunctionCall(isOp); newExpr != isOp {
				n.Cond = newExpr
				v.changed = true
			}
		}
	case *syntax.CallExpr:
		// Handle IS operations in function call arguments
		for i, arg := range n.ArgList {
			if isOp, ok := arg.(*syntax.Operation); ok && isOp.Op == syntax.IS {
				if newExpr := v.transform.convertIsToFunctionCall(isOp); newExpr != isOp {
					n.ArgList[i] = newExpr
					v.changed = true
				}
			}
		}
	}

	return v
}

// convertIsToFunctionCall transforms "x is Type" to "isTypeOf(x, (*Type)(nil))"
func (t *IsOperatorTransform) convertIsToFunctionCall(isOp *syntax.Operation) syntax.Expr {
	if isOp.Op != syntax.IS {
		return isOp
	}

	pos := isOp.Pos()
	
	// Create isTypeOf function name
	funcName := &syntax.Name{
		Value: "isTypeOf",
	}
	funcName.SetPos(pos)
	
	// Create function call arguments
	args := []syntax.Expr{
		isOp.X,                         // Original left operand (the value)
		t.createTypeString(isOp.Y, pos), // Type name as string from right operand
	}
	
	// Create function call: isTypeOf(x, (*Type)(nil))
	funcCall := &syntax.CallExpr{
		Fun:     funcName,
		ArgList: args,
	}
	funcCall.SetPos(pos)
	
	return funcCall
}

// createTypeString creates a string literal containing the type name
func (t *IsOperatorTransform) createTypeString(typeExpr syntax.Expr, pos syntax.Pos) syntax.Expr {
	// Extract type name from the expression
	typeName := t.extractTypeName(typeExpr)
	
	// Create string literal
	stringLit := &syntax.BasicLit{
		Value: `"` + typeName + `"`,
		Kind:  syntax.StringLit,
		Bad:   false,
	}
	stringLit.SetPos(pos)
	
	return stringLit
}

// extractTypeName extracts the type name from a type expression
func (t *IsOperatorTransform) extractTypeName(typeExpr syntax.Expr) string {
	switch expr := typeExpr.(type) {
	case *syntax.Name:
		return expr.Value
	case *syntax.ArrayType:
		if expr.Len == nil {
			// Slice type: []T
			return "[]" + t.extractTypeName(expr.Elem)
		} else {
			// Array type: [N]T - for simplicity, treat as array type
			return "array"
		}
	case *syntax.Operation:
		if expr.Op == syntax.Mul {
			// Pointer type: *T
			return "*" + t.extractTypeName(expr.X)
		}
	case *syntax.SliceType:
		return "[]" + t.extractTypeName(expr.Elem)
	case *syntax.MapType:
		return "map[" + t.extractTypeName(expr.Key) + "]" + t.extractTypeName(expr.Value)
	}
	return "unknown"
}

func init() {
	RegisterTransformer(&IsOperatorTransform{})
}