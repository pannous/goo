echo "full rebuild changes version string"
cd src
./make.bash
GOROOT_BOOTSTRAP=/opt/other/go-darwin-arm64-bootstrap /opt/other/go-darwin-arm64-bootstrap/bin/go install -v cmd/compile