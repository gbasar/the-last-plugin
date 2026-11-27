#!/bin/sh
# Build lplug from source with Go's own build system (go build + go.mod).
#   ./build.sh            all platforms, into agents/claude-code/bin/
#   ./build.sh --local    this machine only
#   ./build.sh --install  this machine, then link lplug into $XDG_BIN_HOME (~/.local/bin)
#
# Which Go: $LPLUG_GO, else `go` on PATH, else ~/.local/share/go/bin/go.
# Where libraries come from: Go's own settings (`go env GOPROXY`, kept in
# ~/.config/go/env). Point that at your Nexus Go proxy once; see README.
# Exits 3 when no Go is found.
set -eu
cd "$(dirname "$0")"

go=${LPLUG_GO:-}
if [ -z "$go" ]; then
  if command -v go >/dev/null 2>&1; then go=go
  elif [ -x "${XDG_DATA_HOME:-$HOME/.local/share}/go/bin/go" ]; then go="${XDG_DATA_HOME:-$HOME/.local/share}/go/bin/go"
  else echo "build.sh: no Go found (set LPLUG_GO=/path/to/go)" >&2; exit 3
  fi
fi
echo "using $("$go" version) from $(command -v "$go" || echo "$go")"
echo "libraries from GOPROXY=$("$go" env GOPROXY)"

version=${VERSION:-$(git describe --tags --always 2>/dev/null || date +%Y.%m.%d)}
out=agents/claude-code/bin
targets="darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64"
mode=${1:-all}
[ "$mode" = all ] || targets="$("$go" env GOOS)/$("$go" env GOARCH)"

for t in $targets; do
  os=${t%/*} arch=${t#*/} ext=
  [ "$os" = windows ] && ext=.exe
  echo "building $os/$arch"
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch "$go" build -trimpath \
    -ldflags "-s -w -X lplug/internal/lplug.Version=$version" \
    -o "$out/lplug-$os-$arch$ext" ./cmd/lplug
done

if [ "$mode" = --install ]; then
  bindir=${XDG_BIN_HOME:-$HOME/.local/bin}
  mkdir -p "$bindir"
  ln -sf "$PWD/$out/lplug" "$bindir/lplug"
  echo "linked $bindir/lplug (make sure $bindir is on your PATH)"
fi
