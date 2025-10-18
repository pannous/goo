# Compiler Build Guide

- Always rebuild the compiler with the local `GOROOT/bin/go`, not the Homebrew system toolchain, otherwise `go1.25.3` is used and the build fails with `go.mod requires go >= 1.26`.
- The helper script `src/build-compiler.sh` now invokes `$GOROOT/bin/go build -tags=transforms -o ../bin/compile ./cmd/compile`, so running it from the repo root is sufficient.
- Rebuilding with `-tags=transforms` is required to enable the full transformer set (string methods, try/catch, auto-return, etc.) when compiling `.goo` sources.
- After rebuilding, rerun `./run_all_tests.sh` to confirm the transformer-enabled compiler passes the goo tests.
