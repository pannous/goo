# Claude Memory File

## Working Guidelines

never unstage or undo my changes (unless explicitly asked), understand and incorporate them!

This project contains the source code for the go language and it's compiler and its toolchains.
It is very comprehensive and we need to be careful when searching for a code to not use up all our clout code subscription context window and usage.

Do NOT explore the whole codebase structure and architecture

We are trying to achieve small modifications to the compiler only and nothing else thus we only want to recompile the compiler and not the whole toolchain.

Always start simple and when the simple case works add more complexity. 

Stick to one suggested approach first, NO "Actually, let me take a different approach." until we have tried out the first one.
If one approach fails, note it in ./probes/TRIED.md and try another approach.
After one feature is done you can delete this file.

• Quick compiler rebuild:
/opt/other/go/src/build-compiler.sh !!

• Full rebuild (when is this needed??)
GOROOT=/opt/other/go ./make.bash 2>&1 | head -10

!! Important! Never do destructive commands like remove, rm, git clean, etc even in YOLO mode: get explicit user confirmation !!

### Style Guidelines

Always replace interface{} with any

# Compile
To compile go, use the following command:

```
cd /opt/other/go/src
```

# Debugging

GOROOT=/opt/other/go ../bin/go build -work
retains logs in 
WORK=/tmp/go-debug/…

# Go Compiler
Project folders:
root  /opt/other/go/
source ${GOROOT}/src
tests ${GOROOT}/goo
debug ${GOROOT}/probe

Main source files:
go/types/universe.go 
cmd/compile/internal/syntax/tokens.go
cmd/compile/internal/noder folder with important files: types.go irgen.go noder.go 

If you're extending Go itself:
•	cmd/compile/internal/syntax/parser.go AST construction!
•	make things pass in types2 but  
•	implement the actual (desugaring) AST transformation in the noder !
•	Or patch the irgen logic in cmd/compile/internal/noder/irgen.go
•	by the time things reach writer.go they should be incorrect format, no more hacking here!

`next()` method: main tokenization loop with large switch statement for each character

### Dual Scanner System
  1. Standard Scanner (go/scanner/scanner.go) Used by go run, go build, and other tools for initial parsing/package analysis
  2. Internal Syntax Scanner (cmd/compile/internal/syntax/scanner.go) Used by the compiler itself for actual compilation
Two main scanning modes: normal tokens + comment/directive callbacks

## Key Methods:
`next()`: main tokenization (big switch ~line 110-355)
`ident()`: identifier scanning
`number()`: numeric literal parsing  
`stdString()`, `rawString()`, `rune()`: string/char literals
`lineComment()`, `fullComment()`: comment handling
Error handling via `errorf()`, position tracking


## Implementation Order for builtins
tokens.so
lexer.go
parser.go
transform.go  custom desugaring stage, best place to start
typecheck.go in types2 should only be touched in the rarest cases, e.g. when adding new builtins that require type checking

1. types2 declarations (universe.go)
2. types2 handling (builtins.go)
3. IR declarations (node.go)
4. IR registration (typecheck/universe.go)
5. IR handling (func.go, typecheck.go)

Register runtime functions at /opt/other/go/src/cmd/compile/internal/typecheck/_builtin/runtime.go Which will code generate declaration lists
If you change this file you must run "go generate" in cmd/compile/internal/typecheck to update builtin.go.
Full rebuild needed: src/make.bash

# Testing Guidelines
ALWAYS RUN tests before doing any changes!
To check the current status and that everthing is (should be) OK.

NEVER modify existing tests!
INSTEAD of touching tests in goo, create new tests in the probes folder
Do not remove tests that are not passing instead leave them there for later fixing.


Do not re-create tests for what is already working, if this is already tested elsewhere.

after performing changes recompile bin/go before testing!

• Never modify my tests in goo. Only add one new test after I've approved the fully functional major feature otherwise leave your tests in probes

# Consolidation
When failing to make progress don't celebrate (prior progress), 
Do NOT repeat What Works well from previous progress,
just be candid and leave the failing tests in the probe directory and summarize the difficulties that have been encountered and possible (alternative) ways forward.

If all newly created tests in ./probes/ succed:
Create exactly one new file <git_root>/goo/test_{feature}.goo to test cases for the new feature; not src/goo, rather ../goo/ ! Don't try to create a new folder. If the folder does not exist you're trying the wrong folder: it should exist!)
Only create ONE new test per feature and reuse existing tests for very similar features.
Before committing quickly run these new tests with the freshly built ../bin/go 

Whenever the number of passing tests in `run_all_tests.sh | tee test_report.md` increases, commit to git!

# Test Writing
Usually when you create one test there's no more need to modify it unless you really missed something
USE existing tests instead of writing new ones
## Editing Guidelines
never edit token_string.go
always use ../bin/go 
NEVER manually touch generated files like op_string.go  token_string.go 

## Enum Stability Practices
 - Never insert in middle of enums
 - Always append new operations at end
 - Use external bootstrap for enum changes (?)
 - Add validation tests for critical constants

To avoid code duplication do a quick git history search (grep?) to see if there have been related changes

Ignore TODO.md, it's only for myself   add to claudeignore and remove this line

## Build Optimization
• no need to ./make.bash twice with no changes

## Transformers
• Add Position information SetPos(pos) In Transformers
• Never modify tests in the goo folder only in probes.

## System Health
• do ./run_all_tests.sh before and after work to see if the general system is ok

## Code File Guidelines
• omit package main and func main in .goo files

## Debugging
• don't Temporarily disable To test functionality that worked before instead push forward to resolve the issue


