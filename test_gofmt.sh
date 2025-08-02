#!/bin/bash

GOROOT=/opt/other/go
GOTMPDIR=/tmp/go-debug
GOROOT_FINAL=/opt/other/go
GODEBUG=keepwork=1
GOROOT_BOOTSTRAP=/opt/other/go-darwin-arm64-bootstrap
GOCACHE=/tmp/go-cache
GOOS=darwin
GOARCH=arm64

# Test runner for gofmt feature support
# Tests each Goo syntactic feature to see if gofmt can format it without errors

cd /opt/other/go

echo "Testing gofmt support for Goo syntactic features..."
echo "=================================================="

passed=0
failed=0
total=0

# Create temporary test directory
mkdir -p /tmp/gofmt_tests
test_dir="/tmp/gofmt_tests"

# Function to test a feature
test_feature() {
    local feature_name="$1"
    local test_code="$2"
    local filename="$test_dir/test_$(echo "$feature_name" | tr ' ' '_' | tr -d '()').goo"
    
    total=$((total + 1))
    
    # Write test code to file
    echo "$test_code" > "$filename"
    
    # Test gofmt on the file
    if ./bin/gofmt "$filename" >/dev/null 2>&1; then
        echo "✅ $feature_name"
        passed=$((passed + 1))
        return 0
    else
        echo "❌ $feature_name"
        failed=$((failed + 1))
        return 1
    fi
}

# ADD new features here, don't use slash '/' in feature names!

test_feature "hash comments" '# This is a hash comment'
test_feature "shebang support" '#!/usr/bin/env go'
test_feature "traditional comments" '// Traditional comment\n/* Block comment */'
test_feature "and operator" 'if true and false {}'
test_feature "or operator" 'if true or false {}'
test_feature "not operator" 'if not true {}'
test_feature "in operator" 'if 1 in [1,2,3] {}'
test_feature "as operator" 'x := 1 as string'
test_feature "def keyword" 'def main(){} '
test_feature "function modifier !" 'def modify!(xs []int) '
test_feature "auto return" ' func test() int { 42}'
test_feature "check keyword" 'check 1 == 1'
test_feature "try catch blocks" 'try { panic("test") } catch e { printf("caught") }'
test_feature "array literal syntax" 'z := [1,2,3]'
test_feature "1-indexed array access" 'z := [1,2,3]x := z#1'
test_feature "map literal syntax" 'm := {a: 1, b: 2}'
test_feature "map dot notation" 'm := {a: 1, b: 2}x := m.a'
test_feature "string concatenation mixed types" 's := "a" + 1'
test_feature "string methods" 'b := "abc".contains("a")'
test_feature "unicode string rune equality" 'b := "你" == '\''你'\'''
test_feature "lambda syntax" 'f := x => x * 2'
test_feature "list methods" 'result := [1,2,3].apply(x => x * 2)'
test_feature "typeof function" 't := typeof(42)'
test_feature "class via type struct" 'type Person class {name string } '
test_feature "enum declarations" 'enum Status { OK, BAD }'
test_feature "printf function" 'printf("hello")'
test_feature "put function" 'put({a: 1})'
test_feature "list comparison" 'b := [1,2] == [1,2]'

# Clean up
rm -rf "$test_dir"

echo ""
echo "=================================================="
echo "Summary: $passed/$total desired features supported by gofmt, $failed failed"

echo "Running gofmt tests on ALL goo example files..."
for test_file in goo/*.go goo/*.goo probes/*.goo; do
    if [[ -f "$test_file" ]]; then
        total=$((total + 1))
        filename=$(basename "$test_file")

        # Run test and capture exit code, suppress all output
#        gtimeout 30s
        if  ./bin/gofmt "$test_file" >/dev/null 2>&1; then
            echo "✅ $filename"
            passed=$((passed + 1))
        else
            echo "❌ $filename"
            failed=$((failed + 1))
#             ./bin/gofmt "$test_file"
#             exit 1
        fi
    fi
done


echo ""
echo "=================================================="
echo "Summary: $passed/$total files supported by gofmt, $failed failed"

if [[ $failed -eq 0 ]]; then
    echo "🎉 All gofmt features working!"
    exit 0
else
    echo "⚠️  Some gofmt features need work"
    exit 1
fi