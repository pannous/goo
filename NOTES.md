Because they serve different goals:

⸻

1. go/parser = Standard Library Parser
	•	Location: src/go/parser/parser.go
	•	Purpose: Used for tools, linters, gofmt, go vet, etc.
	•	Output: go/ast + go/token trees
	•	Focus: Lossless parsing, preserves all syntax (including comments, semicolons, parentheses).
	•	Constraint: Must be backwards-compatible, stable API.
	•	Maintained as part of x/tools, broadly used by editors and external tools.

⸻

2. cmd/compile/internal/syntax
	•	Location: src/cmd/compile/internal/syntax/parser.go
	•	Purpose: Compiler-only, internal use by cmd/compile
	•	Output: Custom minimal AST (syntax package), optimized for fast type checking & codegen
	•	Focus: Performance, not fidelity. Discards comments, formatting details, and some syntactic sugar early.
	•	Often diverges slightly in how it handles edge cases (e.g., error recovery).

⸻

Why not unify them?
	•	Different consumers: Tools need a complete, faithful representation; the compiler needs speed and stripped-down structure.
	•	Backward compatibility: go/parser can’t break users. The compiler parser can evolve aggressively.
	•	Implementation legacy: The compiler had its own parser long before go/parser was generalized for tooling use.

⸻

You’ll also notice:
	•	cmd/compile/internal/syntax uses a more compact AST to reduce GC pressure.
	•	Some experimental syntax may show up here before it reaches the standard parser—since it’s internal.

In short: two parsers exist because tooling and compiling have incompatible requirements.