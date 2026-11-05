#!/usr/bin/env bash
# Installs the pinned Dagger CLI into the directory given as $1 and verifies
# its checksum. The engine version follows the CLI: the CLI starts (or reuses)
# registry.dagger.io/engine:<same version> on the host's Docker daemon.
#
# Bump DAGGER_VERSION here, in .github/workflows/go.yml and in
# .dagger/modules/echopoint/dagger-module.toml together, with the checksums from
# https://dl.dagger.io/dagger/releases/<version>/checksums.txt.
set -euo pipefail

version="${DAGGER_VERSION:-v1.0.0-beta.24}"
dest="${1:?usage: install-dagger.sh <bin-dir>}"

case "$(uname -s)/$(uname -m)" in
  Linux/x86_64) platform=linux_amd64 sum=648a5ffcf7d6d724526d20942066fd023cdf2b8284204d0ca3e1fcf6f0793f6e ;;
  Linux/aarch64) platform=linux_arm64 sum=c6fd1d9fa53ac4880a21373411a923720cd65e6b7a05a617379278a524ee078d ;;
  Darwin/arm64) platform=darwin_arm64 sum=f0b5bfe032f8d245633ee03c350fcc4ad7089bf46a1f86db0a8dddf3011ca5fc ;;
  *) echo "No pinned Dagger checksum for $(uname -s)/$(uname -m)" >&2; exit 1 ;;
esac
if [ "$version" != v1.0.0-beta.24 ]; then
  echo "The checksums in this script are for v1.0.0-beta.24, not $version." >&2
  exit 1
fi

if [ -x "$dest/dagger" ] && "$dest/dagger" version 2>/dev/null | grep -q "$version"; then
  exit 0
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
archive="dagger_${version}_${platform}.tar.gz"
curl -fsSL --retry 3 -o "$tmp/$archive" "https://dl.dagger.io/dagger/releases/${version#v}/$archive"
echo "$sum  $tmp/$archive" | sha256sum -c - >/dev/null 2>&1 \
  || echo "$sum  $tmp/$archive" | shasum -a 256 -c - >/dev/null
mkdir -p "$dest"
tar -xzf "$tmp/$archive" -C "$dest" dagger
"$dest/dagger" version
