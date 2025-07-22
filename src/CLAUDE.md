This project contains the source code for the go language and it's compiler and its toolchains.
It is very comprehensive and we need to be careful when searching for a code to not use up all our clout code subscription context window and usage.

Do NOT explore the whole codebase structure and architecture

We are trying to achieve small modifications to the compiler only and nothing else thus we only want to recompile the compiler and not the whole toolchain.


# To compile the go compiler only, use the following command:

go tool dist install -v cmd/compile
../bin/go tool dist install -v cmd/compile

After we are satisfied with the compiler of from time to time we may try to compile the whole thing:

# To compile the go toolchain, use the following command:

cd /opt/other/go/src
./make.bash

or maybe
GOOS=darwin GOARCH=arm64 ./bootstrap.bash

# Go Compiler Lexer/Scanner Summary

## Main Scanner File: `/opt/other/go/src/cmd/compile/internal/syntax/scanner.go`
- Core lexical analysis for Go compiler
- `next()` method: main tokenization loop with large switch statement for each character
- Token types defined in `tokens.go`, scanner state in scanner struct
- Character reading via `source.go` buffered reader
- Supports UTF-8, tracks line/column positions
- Two main scanning modes: normal tokens + comment/directive callbacks

## Key Methods:
- `next()`: main tokenization (big switch ~line 110-355)
- `ident()`: identifier scanning
- `number()`: numeric literal parsing  
- `stdString()`, `rawString()`, `rune()`: string/char literals
- `lineComment()`, `fullComment()`: comment handling
- Error handling via `errorf()`, position tracking

# Testing Guidelines
- Always create a new go file to test the new feature and store it in the ./goo/ folder
- Before committing quickly run these new tests
- After committing, run the following command in src/ to test the compatibility with the whole system:

```bash
GOROOT=$(pwd)/.. ./run.bash --no-rebuild 2>&1 | grep -Ev '^\?|^ok ' | tee /dev/tty | grep -m1 FAIL && exit 1
```
- This command runs the tests, filters out the output to show only failures, and exits with an error code if any tests fail.
- if we encounter a FAIL, ponder whether our changes might be related to it and if so try once to fix it or tell me to look at it.