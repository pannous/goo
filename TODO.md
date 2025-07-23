  - assert(condition) panics at runtime if condition is false
  - For compile-time boolean constants, checking occurs at compile time
  - Generates if !condition { panic("assertion failed") } in the backend
  - Follows the same pattern as panic() builtin

  Current Status:
  The implementation compiles successfully but encounters a runtime type system error during execution. The error
   suggests there's a type signature mismatch in the types2 system where a *types2.Basic type is being treated as
   a *types2.Signature. This appears to be a deeper integration issue with Go's type checking system that would
  require additional investigation to resolve completely.


/opt/other/go/bin/go list -modfile=/opt/other/go/pkg/bootstrap/src/bootstrap/cmd/compile/internal/ssa/_gen/go.mod -m -json -mod=mod all #gosetup
malformed go.sum:
/opt/other/go/pkg/bootstrap/src/bootstrap/cmd/compile/internal/ssa/_gen/go.sum:1: wrong number of fields 10


⏺ Bash(git revert --no-edit a93845c9d8b01fc269c30500d4f139b8d0fd08aa)
  ⎿  Error: error: could not revert a93845c9d8... check OK!
     hint: After resolving the conflicts, mark them with
     hint: "git add/rm <pathspec>", then run
     hint: "git revert --continue".
     hint: You can instead skip this commit with "git revert --skip".
     hint: To abort and get back to the state before "git revert",
     hint: run "git revert --abort".
     hint: Disable this message with "git config set advice.mergeConflict false"
     Recorded preimage for 'src/CLAUDE.md'