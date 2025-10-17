#!/bin/bash

GOROOT=/opt/other/go
GOTMPDIR=/tmp/go-debug
GOROOT_FINAL=/opt/other/go
GODEBUG=keepwork=1
GOROOT_BOOTSTRAP=/opt/other/go-darwin-arm64-bootstrap
GOCACHE=/tmp/go-cache
GOOS=darwin
GOARCH=arm64
GOO_USE_TRANSFORMERS=1

# Test runner for all tests in ./goo/ directory
# Suppresses output and shows ✅/🔴 summary per test

cd /opt/other/go

echo "Running all tests in ./goo/ directory..."
echo "========================================"

passed=0
failed=0
total=0

for test_file in goo/*.go goo/*.goo probes/*.goo; do
    if [[ -f "$test_file" ]]; then
        total=$((total + 1))
        filename=$(basename "$test_file")
        # Run test and capture exit code, suppress all output
#        gtimeout 30s
        if  ./bin/go run "$test_file" >/dev/null 2>&1; then
            echo "✅ $filename"
            passed=$((passed + 1))
        else
            echo "❌ $filename"
            failed=$((failed + 1))
            # ./bin/go run "$test_file"
        fi
    fi
done

echo "========================================"
echo "Summary: $passed/$total passed, $failed failed"

if [[ $failed -eq 0 ]]; then
    echo "🎉 All tests passed!"
    exit 0
else
    echo "⚠️  Some tests failed"
    if [[ $failed -gt 50 ]]; then
        echo "Consider ./build-compiler.sh or do rollback!"
    fi
    exit 1
fi