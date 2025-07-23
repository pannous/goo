This project contains the source code for the go language and it's compiler and its toolchains.
It is very comprehensive and we need to be careful when searching for a code to not use up all our clout code subscription context window and usage.

Do NOT explore the whole codebase structure and architecture

We are trying to achieve small modifications to the compiler only and nothing else thus we only want to recompile the compiler and not the whole toolchain.


# To compile go, use the following command:

cd /opt/other/go/src
./make.bash >/dev/null 2>&1

or maybe
GOOS=darwin GOARCH=arm64 ./bootstrap.bash ?

So far incremental building does not work like this :(

../bin/go tool dist install -v cmd/compile


# Go Compiler Lexer/Scanner Summary

## Main Files: 
- Project root folder `/opt/other/go/`
- Claude root folder `/opt/other/go/src/`
- Core lexical analysis for Go compiler
- Main files:
  - `cmd/compile/internal/syntax/scanner.go`: main scanner implementation
  - `go/scanner/scanner.go`: standard scanner used by tools like `go run`, `go build`
  - `go/scanner/tokens.go`: token definitions
  - `go/scanner/source.go`: source reading and buffering
  - ./cmd/compile/internal/noder folder with important files:
      unified.go
      lex_test.go
      lex.go
      import.go
      posmap.go
      types.go
      export.go
      quirks.go
      codes.go
      writer.go
      linker.go
      doc.go
      reader.go
      irgen.go
      helpers.go
      noder.go
- Token types defined in `tokens.go`, scanner state in scanner struct
- Character reading via `source.go` buffered reader
- Supports UTF-8, tracks line/column positions
- `next()` method: main tokenization loop with large switch statement for each character

- ### Dual Scanner System
  1. Standard Scanner (go/scanner/scanner.go) - Used by go run, go build, and other tools for initial parsing/package analysis
  2. Internal Syntax Scanner (cmd/compile/internal/syntax/scanner.go) - Used by the compiler itself for actual compilation
- Two main scanning modes: normal tokens + comment/directive callbacks

## Key Methods:
- `next()`: main tokenization (big switch ~line 110-355)
- `ident()`: identifier scanning
- `number()`: numeric literal parsing  
- `stdString()`, `rawString()`, `rune()`: string/char literals
- `lineComment()`, `fullComment()`: comment handling
- Error handling via `errorf()`, position tracking

# Testing Guidelines
- recompile bin/go before testing!
- Always create exactly one new go file <git_root>/goo/test_{feature}.go to test the new feature; not src/goo, rather ../goo/ ! Don't try to create a new folder. If the folder does not exist you're trying the wrong folder: it should exist!)
- IF you end up with multiple test / debug files (which you shouldn't) make sure to delete all but one before committing
- Before committing quickly run these new tests with the freshly built ../bin/go 
- After committing, run the following command in src/ to test the compatibility with the whole system:

```bash
GOROOT=$(pwd)/.. ./run.bash --no-rebuild 2>&1 | grep -Ev '^\?|^ok ' | tee /dev/tty | grep -m1 FAIL && exit 1
```
- This command runs the tests, filters out the output to show only failures, and exits with an error code if any tests fail.
- if we encounter a FAIL, ponder whether our changes might be related to it and if so try once to fix it or tell me to look at it.

# Compiler Build Notes
- `go tool dist install` is unreliable and may do nothing
- Recommendation: Use `./make.bash` directly for compiler installation