package transforms

import (
	"cmd/compile/internal/syntax"
)

type ApproxEqualityTransform struct{}

type approxVisitor struct {
	transform *ApproxEqualityTransform
	ctx       *TransformContext
	file      *syntax.File
	changed   bool
	needsMathImport bool
}

func (t *ApproxEqualityTransform) Name() string {
	return "approx_equality_transform"
}

func (t *ApproxEqualityTransform) Priority() int {
	return 100 // Run after basic transforms
}

func (t *ApproxEqualityTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	visitor := &approxVisitor{transform: t, ctx: ctx, file: file}
	
	// Use syntax.Walk to traverse the entire AST
	syntax.Walk(file, visitor)
	
	return visitor.changed
}

func (v *approxVisitor) Visit(node syntax.Node) syntax.Visitor {
	switch n := node.(type) {
	case *syntax.Operation:
		if n.Op == syntax.Approx && n.Y != nil {
			v.changed = true
			
			// Transform "a ≈ b" into "((a - b) < 1e-9) && ((b - a) < 1e-9)"
			// This avoids needing math.Abs by checking both directions
			pos := n.Pos()
			
			// Create a - b
			diff1 := &syntax.Operation{
				Op: syntax.Sub,
				X:  n.X,
				Y:  n.Y,
			}
			diff1.SetPos(pos)
			
			// Create b - a
			diff2 := &syntax.Operation{
				Op: syntax.Sub,
				X:  n.Y,
				Y:  n.X,
			}
			diff2.SetPos(pos)
			
			// Create epsilon (1e-9)
			epsilon1 := &syntax.BasicLit{
				Value: "1e-9",
				Kind:  syntax.FloatLit,
			}
			epsilon1.SetPos(pos)
			
			epsilon2 := &syntax.BasicLit{
				Value: "1e-9",
				Kind:  syntax.FloatLit,
			}
			epsilon2.SetPos(pos)
			
			// Create (a - b) < 1e-9
			cmp1 := &syntax.Operation{
				Op: syntax.Lss,
				X:  diff1,
				Y:  epsilon1,
			}
			cmp1.SetPos(pos)
			
			// Create (b - a) < 1e-9
			cmp2 := &syntax.Operation{
				Op: syntax.Lss,
				X:  diff2,
				Y:  epsilon2,
			}
			cmp2.SetPos(pos)
			
			// Replace the ≈ operation with &&
			n.Op = syntax.AndAnd
			n.X = cmp1
			n.Y = cmp2
		}
	}
	return v
}


func init() {
	RegisterTransformer(&ApproxEqualityTransform{})
}