import "slices"
package slices
func Filter[S ~[]E, E any](s S, f func(E) bool) S { return s }
