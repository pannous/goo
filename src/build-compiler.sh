GOROOT=/opt/other/go
GOTMPDIR=/tmp/go-debug
GOROOT_FINAL=/opt/other/go
GODEBUG=keepwork=1
GOCACHE=/tmp/go-cache
GOOS=darwin
GOARCH=arm64
cd /opt/other/go/src/

# Build compiler
$GOROOT/bin/go build -tags=transforms -o ../bin/compile ./cmd/compile
cp ../bin/compile ../pkg/tool/darwin_arm64/compile

# Build go command
$GOROOT/bin/go build -tags=transforms -o ../bin/go ./cmd/go
