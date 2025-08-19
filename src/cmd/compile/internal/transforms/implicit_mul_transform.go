package transforms

import (
	"cmd/compile/internal/syntax"
)

func init() {
	RegisterTransformer(&ImplicitMulTransformer{})
}

type ImplicitMulTransformer struct{}

func (t *ImplicitMulTransformer) Name() string {
	return "implicit_mul_transform"
}

func (t *ImplicitMulTransformer) Priority() int {
	return 10 // Run very early, before other transformers
}

func (t *ImplicitMulTransformer) Transform(file *syntax.File, ctx *TransformContext) bool {
	// Temporarily disable this transformer to test postfix operators
	return false
}

func (t *ImplicitMulTransformer) shouldTransform(file *syntax.File) bool {
	return true
}

// The key insight is to handle this at the scanner level
// by modifying how number() works when it encounters letters