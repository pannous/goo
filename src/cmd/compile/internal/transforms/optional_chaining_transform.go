//go:build transforms

package transforms

import (
	"cmd/compile/internal/syntax"
)

// OptionalChainingTransform handles the optional chaining operator (?.)
//
// CURRENT STATUS: Parser infrastructure complete, transformation TODO
// - Scanner recognizes ?. token
// - Parser sets SelectorExpr.Optional flag
// - Transformation disabled pending type information
//
// IMPLEMENTATION NOTES:
// Proper nil-safe transformation requires type information to generate correct zero values.
// This should be implemented in cmd/compile/internal/noder where type information is available.
//
// Example transformation needed:
//   p?.name  =>  func() T { var result T; if p != nil { result = p.name }; return result }()
//
// For now, the Optional flag passes through unchanged and will cause a panic if dereferenced.
type OptionalChainingTransform struct{}

func (t *OptionalChainingTransform) Name() string {
	return "optional_chaining_transform"
}

func (t *OptionalChainingTransform) Priority() int {
	return 200 // Run fairly late in the pipeline
}

func (t *OptionalChainingTransform) Transform(file *syntax.File, ctx *TransformContext) bool {
	// Transformation disabled - pass through unchanged
	// The Optional flag remains on SelectorExpr nodes for future implementation
	return false
}

func init() {
	RegisterTransformer(&OptionalChainingTransform{})
}
