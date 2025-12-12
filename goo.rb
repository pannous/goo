class Goo < Formula
  desc "Go with syntactic sugar - truthy values, lambdas, type operators, and more"
  homepage "https://github.com/pannous/goo"
  version "1.0.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/pannous/goo/releases/download/v#{version}/goo-darwin-arm64.tar.gz"
      sha256 "" # Will be filled when release is created
    else
      url "https://github.com/pannous/goo/releases/download/v#{version}/goo-darwin-amd64.tar.gz"
      sha256 "" # Will be filled when release is created
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/pannous/goo/releases/download/v#{version}/goo-linux-arm64.tar.gz"
      sha256 "" # Will be filled when release is created
    else
      url "https://github.com/pannous/goo/releases/download/v#{version}/goo-linux-amd64.tar.gz"
      sha256 "" # Will be filled when release is created
    end
  end

  def install
    bin.install "bin/go" => "goo"
    prefix.install Dir["*"]
  end

  def caveats
    <<~EOS
      Add the following to your shell profile:
        export GOROOT=#{prefix}
        export PATH=#{bin}:$PATH
    EOS
  end

  test do
    system "#{bin}/goo", "version"
  end
end
