GOROOT=/opt/other/go
GOTMPDIR=/tmp/go-debug
GOROOT_FINAL=/opt/other/go
GODEBUG=keepwork=1
GOCACHE=/tmp/go-cache
GOOS=darwin
GOARCH=arm64
cd /opt/other/go/src/

# Update version in zbootstrap.go with current git hash
HASH=$(git -C /opt/other/go rev-parse --short HEAD 2>/dev/null || echo "unknown")
if [ "$HASH" != "unknown" ]; then
  # Check if version changed
  CURRENT_VERSION=$(grep "const version = " /opt/other/go/src/internal/buildcfg/zbootstrap.go | cut -d'`' -f2)
  NEW_VERSION="go1.26.1.$HASH"

  if [ "$CURRENT_VERSION" != "$NEW_VERSION" ]; then
    echo "Updating version from $CURRENT_VERSION to $NEW_VERSION"
    sed -i '' "s/const version = \`go1\.[0-9]*\.[0-9]*\.[a-f0-9]*\`/const version = \`$NEW_VERSION\`/" \
      /opt/other/go/src/internal/buildcfg/zbootstrap.go
    # Clear build caches when version changes
    GOCACHE=/tmp/go-cache $GOROOT/bin/go clean -cache
    $GOROOT/bin/go clean -cache  # Also clean default cache
  fi
fi

# Build compiler
$GOROOT/bin/go build -tags=transforms -o ../bin/compile ./cmd/compile
cp ../bin/compile ../pkg/tool/darwin_arm64/compile

# Build go command
$GOROOT/bin/go build -tags=transforms -o ../bin/go ./cmd/go
