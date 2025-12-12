#!/bin/bash
set -e

# Build complete Goo toolchain with all tools
# This ensures all tools are built with matching versions

echo "Building complete Goo toolchain..."

cd "$(dirname "$0")/.."

# Set GOROOT to our directory
export GOROOT=$(pwd)
export GOROOT_FINAL=$GOROOT

# Build using make.bash (builds everything including all tools)
echo "Running make.bash to build complete toolchain..."
cd src
./make.bash

echo ""
echo "✅ Toolchain build complete!"
echo ""
echo "Built tools:"
ls -lh ../pkg/tool/darwin_arm64/ 2>/dev/null || ls -lh ../pkg/tool/*/
echo ""
echo "To create optimized release package:"
echo "  ./scripts/package-release.sh darwin arm64"
