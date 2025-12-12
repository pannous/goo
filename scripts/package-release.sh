#!/bin/bash
set -e

# Package Goo for release with optimized size
# Usage: ./scripts/package-release.sh [platform] [arch]
# Example: ./scripts/package-release.sh darwin arm64

PLATFORM=${1:-darwin}
ARCH=${2:-arm64}
VERSION=${3:-1.0.0}

OUTPUT_DIR="/tmp/goo-${PLATFORM}-${ARCH}"
TARBALL="goo-${PLATFORM}-${ARCH}.tar.gz"

echo "Creating optimized Goo release for ${PLATFORM}-${ARCH}..."

# Clean up any existing output
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# Copy binaries (only for target platform)
echo "Copying binaries..."
mkdir -p "$OUTPUT_DIR/bin"
cp bin/go "$OUTPUT_DIR/bin/"
cp bin/gofmt "$OUTPUT_DIR/bin/"
cp bin/compile "$OUTPUT_DIR/bin/"

# Build missing tools if they don't exist
echo "Building complete toolchain..."
if [ ! -f "pkg/tool/${PLATFORM}_${ARCH}/asm" ]; then
    echo "Building missing tools (asm, link, etc)..."
    cd src
    GOTOOLCHAIN=local go build -o "../pkg/tool/${PLATFORM}_${ARCH}/asm" cmd/asm
    GOTOOLCHAIN=local go build -o "../pkg/tool/${PLATFORM}_${ARCH}/link" cmd/link
    GOTOOLCHAIN=local go build -o "../pkg/tool/${PLATFORM}_${ARCH}/cgo" cmd/cgo
    cd ..
fi

# Copy pkg directory (includes tools)
echo "Copying pkg..."
cp -r pkg "$OUTPUT_DIR/"

# Copy lib directory
echo "Copying lib..."
cp -r lib "$OUTPUT_DIR/"

# Copy essential source files (needed for stdlib)
echo "Copying essential source files..."
mkdir -p "$OUTPUT_DIR/src"
# Copy only stdlib source (not cmd tools source)
for dir in src/*; do
    basename=$(basename "$dir")
    # Skip cmd directory and build scripts - not needed at runtime
    if [[ "$basename" != "cmd" && "$basename" != *.bash && "$basename" != *.sh ]]; then
        cp -r "$dir" "$OUTPUT_DIR/src/"
    fi
done
# Copy essential files from src root
cp src/go.mod "$OUTPUT_DIR/src/" 2>/dev/null || true
cp src/README.vendor "$OUTPUT_DIR/src/" 2>/dev/null || true

# Copy api directory (smaller, useful for compatibility)
echo "Copying api..."
cp -r api "$OUTPUT_DIR/"

# Copy misc directory (very small)
echo "Copying misc..."
cp -r misc "$OUTPUT_DIR/"

# Create tarball
echo "Creating tarball..."
cd /tmp
tar -czf "$TARBALL" "goo-${PLATFORM}-${ARCH}"

# Calculate size and hash
SIZE=$(du -h "$TARBALL" | cut -f1)
SHA256=$(shasum -a 256 "$TARBALL" | cut -d' ' -f1)

echo ""
echo "✅ Release package created: $TARBALL"
echo "   Size: $SIZE"
echo "   SHA256: $SHA256"
echo ""
echo "To upload to GitHub:"
echo "  gh release create v${VERSION} /tmp/$TARBALL --title \"Goo v${VERSION}\" --notes \"Optimized release\""
echo ""
echo "Update goo.rb with this SHA256:"
echo "  sha256 \"$SHA256\""
