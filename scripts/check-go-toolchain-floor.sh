#!/usr/bin/env bash
# check-go-toolchain-floor.sh <binary> [go.mod-path]
#
# Fails unless <binary> was built by a Go toolchain at least as new as the
# `toolchain` directive in go.mod. setup-go (go-version-file: go.mod) installs
# that directive's version, so this proves the binary we ship was actually
# built with it rather than with an older toolchain on the runner. The image
# COPYs a prebuilt gc, so its Dockerfile pin alone does not decide this.
#
# Fails closed: a missing directive, an unreadable binary, or an unparseable
# version is an error, never a pass.
#
# Exit codes: 0 = at or above the floor; 1 = below it or unverifiable;
# 2 = usage.
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: $0 <binary> [go.mod-path]" >&2
  exit 2
fi

binary=$1
gomod=${2:-go.mod}

if [[ ! -f "$binary" ]]; then
  echo "ERROR: binary not found: $binary" >&2
  exit 1
fi
if [[ ! -f "$gomod" ]]; then
  echo "ERROR: go.mod not found: $gomod" >&2
  exit 1
fi

floor=$(sed -nE 's/^toolchain go([0-9]+\.[0-9]+\.[0-9]+)[[:space:]]*$/\1/p' "$gomod" | head -n 1)
if [[ -z "$floor" ]]; then
  echo "ERROR: $gomod has no 'toolchain goX.Y.Z' directive to check against" >&2
  exit 1
fi

if ! version_line=$(go version "$binary" 2>&1); then
  echo "ERROR: go version $binary failed: $version_line" >&2
  exit 1
fi
built=$(sed -nE 's/^.*: go([0-9]+\.[0-9]+(\.[0-9]+)?)([^0-9.].*)?$/\1/p' <<<"$version_line" | head -n 1)
if [[ -z "$built" ]]; then
  echo "ERROR: could not read the Go version from: $version_line" >&2
  exit 1
fi
# go1.N (no patch) is the .0 release.
[[ "$built" == *.*.* ]] || built="$built.0"

lowest=$(printf '%s\n%s\n' "$floor" "$built" | sort -t. -k1,1n -k2,2n -k3,3n | head -n 1)
if [[ "$lowest" != "$floor" ]]; then
  echo "ERROR: $binary was built with go$built, below the go.mod toolchain floor go$floor" >&2
  exit 1
fi

echo "go toolchain floor: OK ($binary built with go$built, floor go$floor)"
