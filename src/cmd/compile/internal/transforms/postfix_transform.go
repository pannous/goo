package transforms

import (
	"cmd/compile/internal/syntax"
)

func init() {
	RegisterTransformer(&PostfixTransformer{})
}

type PostfixTransformer struct{}

func (t *PostfixTransformer) Name() string {
	return "postfix_transform"
}

func (t *PostfixTransformer) Priority() int {
	return 1 // Run VERY early, before any other transforms
}

func (t *PostfixTransformer) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &postfixVisitor{transform: t, ctx: ctx}
	syntax.Walk(file, visitor)
	return visitor.changed
}

type postfixVisitor struct {
	transform *PostfixTransformer
	ctx       *TransformContext
	changed   bool
}

func (v *postfixVisitor) Visit(node syntax.Node) syntax.Visitor {
	if node == nil {
		return nil
	}

	// Handle specific parent nodes that might contain PostfixExpr
	switch n := node.(type) {
	case *syntax.ExprStmt:
		if postfix, ok := n.X.(*syntax.PostfixExpr); ok {
			replacement := v.transform.convertPostfixToMultiplication(postfix)
			if replacement != nil {
				n.X = replacement
				v.changed = true
			}
		}
	case *syntax.Operation:
		if postfix, ok := n.X.(*syntax.PostfixExpr); ok {
			replacement := v.transform.convertPostfixToMultiplication(postfix)
			if replacement != nil {
				n.X = replacement
				v.changed = true
			}
		}
		if postfix, ok := n.Y.(*syntax.PostfixExpr); ok {
			replacement := v.transform.convertPostfixToMultiplication(postfix)
			if replacement != nil {
				n.Y = replacement
				v.changed = true
			}
		}
	case *syntax.CallExpr:
		for i, arg := range n.ArgList {
			if postfix, ok := arg.(*syntax.PostfixExpr); ok {
				replacement := v.transform.convertPostfixToMultiplication(postfix)
				if replacement != nil {
					n.ArgList[i] = replacement
					v.changed = true
				}
			}
		}
	}
	
	return v
}

func (t *PostfixTransformer) convertPostfixToMultiplication(postfix *syntax.PostfixExpr) syntax.Expr {
	switch postfix.Op {
	case "²":
		// x² becomes x * x
		// Create separate copies to avoid shared node issues
		mult := &syntax.Operation{
			Op: syntax.Mul,
			X:  postfix.X,
			Y:  t.copyExpr(postfix.X),
		}
		mult.SetPos(postfix.Pos())
		return mult
		
	case "³":
		// x³ becomes x * x * x  
		// First create x * x
		innerMult := &syntax.Operation{
			Op: syntax.Mul,
			X:  postfix.X,
			Y:  t.copyExpr(postfix.X),
		}
		innerMult.SetPos(postfix.Pos())
		
		// Then multiply that by x again
		outerMult := &syntax.Operation{
			Op: syntax.Mul,
			X:  innerMult,
			Y:  t.copyExpr(postfix.X),
		}
		outerMult.SetPos(postfix.Pos())
		return outerMult
		
	default:
		return nil
	}
}

// copyExpr creates a copy of an expression to avoid shared node issues
func (t *PostfixTransformer) copyExpr(expr syntax.Expr) syntax.Expr {
	switch e := expr.(type) {
	case *syntax.Name:
		copy := &syntax.Name{Value: e.Value}
		copy.SetPos(e.Pos())
		return copy
	case *syntax.BasicLit:
		copy := &syntax.BasicLit{Value: e.Value, Kind: e.Kind, Bad: e.Bad}
		copy.SetPos(e.Pos())
		return copy
	default:
		// For other expression types, return the original
		// This is a simplified approach - in a full implementation we'd handle all types
		return expr
	}
}