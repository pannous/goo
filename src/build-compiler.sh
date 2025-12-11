#!/bin/bash
# Incremental build script for development
# Note: Does NOT update version string - use ./make.bash for version updates

GOROOT=/opt/other/go
GOTMPDIR=/tmp/go-debug
GOROOT_FINAL=/opt/other/goo
GODEBUG=keepwork=1
GOCACHE=/tmp/go-cache
GOOS=darwin
GOARCH=arm64
cd /opt/other/goo/src/

# echo "Incremental build (version will not update - use make.bash for full rebuild)"

# Build compiler
$GOROOT/bin/go build -tags=transforms -o ../bin/compile ./cmd/compile
cp ../bin/compile ../pkg/tool/darwin_arm64/compile

# Build go command
# $GOROOT/bin/go build -tags=transforms -o ../bin/go ./cmd/go

# echo "Done! Run './bin/go version' to see current version"
