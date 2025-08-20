#!/bin/bash

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: scripts/run_one.sh <path-to-test.goo>" >&2
  exit 1
fi

TEST_FILE="$1"

export GOROOT=/opt/other/go
export GOTMPDIR=/tmp/go-debug
export GOROOT_FINAL=/opt/other/go
export GODEBUG=keepwork=1
export GOROOT_BOOTSTRAP=/opt/other/go-darwin-arm64-bootstrap
export GOCACHE=/tmp/go-cache
export GOOS=darwin
export GOARCH=arm64
export GOO_USE_TRANSFORMERS=1

echo "Running: $TEST_FILE"
set -x
./bin/go run "$TEST_FILE"
