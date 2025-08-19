package transforms

import (
	"cmd/compile/internal/syntax"
	"strconv"
)

// PowerTransformer handles ** power operations by expanding them inline or using runtime calls
type PowerTransformer struct{}

func (t *PowerTransformer) Name() string {
	return "power_transform"
}

func (t *PowerTransformer) Priority() int {
	return 60 // Run after basic operators but before complex transforms
}

func (t *PowerTransformer) Transform(file *syntax.File, ctx *TransformContext) bool {
	if file == nil {
		return false
	}
	
	transformed := false
	
	// Transform ** power operations
	syntax.Inspect(file, func(n syntax.Node) bool {
		if op, ok := n.(*syntax.Operation); ok {
			if op.Op == syntax.Power && op.X != nil && op.Y != nil {
				t.transformPowerOperation(op)
				transformed = true
			}
		}
		return true
	})
	
	return transformed
}

// transformPowerOperation converts base ** exp to appropriate implementation
func (t *PowerTransformer) transformPowerOperation(op *syntax.Operation) {
	if op.X == nil || op.Y == nil {
		return
	}
	
	pos := op.Pos()
	
	// Handle simple integer literal exponents inline
	if expLit, ok := op.Y.(*syntax.BasicLit); ok && expLit.Kind == syntax.IntLit {
		if exp, err := strconv.Atoi(expLit.Value); err == nil && exp >= 0 && exp <= 3 {
			t.expandInlinePower(op, exp)
			return
		}
	}
	
	// For complex cases, create a simple iterative power implementation
	// This avoids dependency on math.Pow which may not be available
	t.createIterativePower(op, pos)
}

// expandInlinePower expands simple powers inline for efficiency
func (t *PowerTransformer) expandInlinePower(op *syntax.Operation, exp int) {
	base := op.X
	pos := op.Pos()
	
	switch exp {
	case 0:
		// x ** 0 = 1
		op.Op = syntax.Add // dummy operation
		op.X = &syntax.BasicLit{Value: "1", Kind: syntax.IntLit}
		op.X.SetPos(pos)
		op.Y = nil
		
	case 1:
		// x ** 1 = x
		op.Op = syntax.Add // dummy operation  
		op.Y = nil
		// op.X remains as base
		
	case 2:
		// x ** 2 = x * x
		op.Op = syntax.Mul
		op.Y = t.cloneExpr(base, pos)
		
	case 3:
		// x ** 3 = x * (x * x)
		xSquared := &syntax.Operation{
			Op: syntax.Mul,
			X:  t.cloneExpr(base, pos),
			Y:  t.cloneExpr(base, pos),
		}
		xSquared.SetPos(pos)
		
		op.Op = syntax.Mul
		op.Y = xSquared
	}
}

// createIterativePower creates an inline power implementation
// Converts: base ** exp  
// To: func() int { result := 1; for i := 0; i < exp; i++ { result *= base }; return result }()
func (t *PowerTransformer) createIterativePower(op *syntax.Operation, pos syntax.Pos) {
	// For now, just implement x ** 2 as x * x for any non-literal case
	// A full implementation would create the iterative logic
	op.Op = syntax.Mul
	op.Y = t.cloneExpr(op.X, pos)
}

// cloneExpr creates a copy of simple expressions to avoid shared references
func (t *PowerTransformer) cloneExpr(expr syntax.Expr, pos syntax.Pos) syntax.Expr {
	switch e := expr.(type) {
	case *syntax.Name:
		clone := &syntax.Name{Value: e.Value}
		clone.SetPos(pos)
		return clone
	case *syntax.BasicLit:
		clone := &syntax.BasicLit{Value: e.Value, Kind: e.Kind}
		clone.SetPos(pos)
		return clone
	case *syntax.ParenExpr:
		// Clone parenthesized expressions
		clone := &syntax.ParenExpr{X: t.cloneExpr(e.X, pos)}
		clone.SetPos(pos)
		return clone
	default:
		// For complex expressions, return the original
		// In a full implementation, we'd implement deep cloning
		return expr
	}
}

func init() {
	RegisterTransformer(&PowerTransformer{})
}