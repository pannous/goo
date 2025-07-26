<!-- remove GOROOT_FINAL for release! -->
 remove from .bashpaths : 
export GOROOT_FINAL=/opt/other/go 


put(1,2,3)
printf(1,2,3)
printf("%v %v %v", 1,2,3)

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

haha 
       ../goo/test_printf.go:7:2: printf("OK") (no value) is not used
   ../goo/test_printf_discard.go:4:6: printf("OK") (no value) used as value
  yeah Let me sleep about that     



errors.UnusedImport

  ⏺ I found it! Looking at line 421, the handleBailout function sets *err = checks.firstErr. /* And in the Files 
  method at line 447, the defer checks.handleBailout(&err) statement at line 459 will set the return error to
  checks.firstErr.

  So even if I handle unused imports as warnings instead of errors, if they are the first error encountered, they
   will still cause the Check method to return an error.

  The solution is to modify the types2 system to not set checks.firstErr for soft errors like unused imports. Let
   me check where firstErr is set:


Should we completely remove the old buildin mechanism for printf,put or would that make it fail?
       682 +    case _Put:
       683 +      // put is handled by AST transformation in callExpr, not as a builtin
       684 +      // This case should never be reached due to early transformation
       685 +      checks.errorf(call, InvalidSyntaxTree, "put should have been transformed to fmt.Printf")
       686 +  

parser.go
sep == SPACE? OK
// list = [ f { sep f } [sep] ] close .
func (p *parser) list(context string, sep, close token, f func() bool) Pos {
	if debug && (sep != _Comma && sep != _Semi || close != _Rparen && close != _Rbrace && close != _Rbrack) {
		panic("invalid sep or close argument for list")
	}


<--
Currently ignore the system test because it fails unrelated to our changes:
After committing, run the following command in src/ to test the compatibility with the whole system:
```bash
GOROOT=/opt/other/go ./run.bash --no-rebuild 2>&1 | grep -Ev '^\?|^ok ' | grep -m1 FAIL && exit 1
```
This command runs the tests, filters out the output to show only failures, and exits with an error code if any tests fail.
if we encounter a FAIL, ponder whether our changes might be related to it and if so try once to fix it or tell me to look at it.
-->



tools requires Go 1.25.5, but system go is 1.24.5 => 
ignore the go.mod by GOTOOLCHAIN=local  
or use GOROOT=/opt/other/go /opt/other/go/bin/go instead of global go!


 Write(test_runtime.go)
  ⎿  Wrote 11 lines to test_runtime.go                
     package main
     import "runtime"
     //go:linkname stringfromint runtime.stringfromint
     func stringfromint(x int64) string
     func main() {
         result := stringfromint(42)
         println("stringfromint(42):", result)