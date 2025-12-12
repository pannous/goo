# Creating a Goo Release

## Darwin arm64 (macOS Apple Silicon) ✅
Already built: `goo-darwin-arm64.tar.gz` (125MB)
SHA256: `99a3b954418e8a235f00c050484733626bf19787f71ec86f041d9945eaaf65a8`

## Building Other Platforms

### Using Multipass for Linux Builds

```bash
# Start Ubuntu VM
multipass launch --memory 4G --name goo-build || echo "VM may already exist"
multipass mount /opt/other/goo goo-build:goo

# Build for Linux
multipass exec goo-build -- bash -c "
  cd goo/src &&
  sudo apt-get update &&
  sudo apt-get install -y golang-go git build-essential &&
  GOTOOLCHAIN=local go build -tags=transforms -o ../bin/compile ./cmd/compile
"

# Package Linux arm64
multipass exec goo-build -- bash -c "
  mkdir -p /tmp/goo-linux-arm64 &&
  cd goo &&
  cp -r bin lib pkg src api misc /tmp/goo-linux-arm64/ &&
  cd /tmp &&
  tar -czf goo-linux-arm64.tar.gz goo-linux-arm64 &&
  shasum -a 256 goo-linux-arm64.tar.gz
"

# Copy out
multipass exec goo-build -- cat /tmp/goo-linux-arm64.tar.gz > goo-linux-arm64.tar.gz
```

### Darwin amd64 (macOS Intel)
Cross-compile or build on Intel Mac

### Linux amd64
Use multipass with GOARCH=amd64

## Creating GitHub Release

1. Tag the release:
```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

2. Create release on GitHub:
```bash
gh release create v1.0.0 \
  goo-darwin-arm64.tar.gz \
  goo-darwin-amd64.tar.gz \
  goo-linux-arm64.tar.gz \
  goo-linux-amd64.tar.gz \
  --title "Goo v1.0.0" \
  --notes "First stable release of Goo"
```

3. Update SHA256 hashes in `goo.rb` after creating release

4. Test installation:
```bash
brew tap pannous/goo
brew install goo
goo version
```

Note: The tap repository is now at https://github.com/pannous/homebrew-goo (separate from source repo)
