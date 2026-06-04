#!/bin/bash
set -euo pipefail

REPO="${1:-https://github.com/Parchive/par2cmdline}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
EMBED_DIR="${SCRIPT_DIR}/../cmd/par2cron/embed"

BUILDDIR="$(mktemp -d)"
trap 'rm -rf "$BUILDDIR"' EXIT

echo "==> Cloning ${REPO}"
git clone --depth 1 "$REPO" "$BUILDDIR/par2cmdline"

pushd "$BUILDDIR/par2cmdline"
  echo "==> Patching and building (static)"
  sed -i '/AC_OPENMP/d' configure.ac
  ./automake.sh
  ./configure LDFLAGS="-static"
  make
  make check
popd

LDD_OUTPUT=$(ldd "$BUILDDIR/par2cmdline/par2" 2>&1) || true
if ! echo "$LDD_OUTPUT" | grep -qi "not a dynamic"; then
  echo "$LDD_OUTPUT"
  echo "==> ERROR: par2 is not statically linked" >&2
  exit 1
fi
echo "==> Confirmed: par2 is statically linked"

echo "==> Copying par2 binary to ${EMBED_DIR}"
mkdir -p "$EMBED_DIR"
cp -f "$BUILDDIR/par2cmdline/par2" "$EMBED_DIR/par2"

echo "==> Running make par2cron-embed"
(cd "$SCRIPT_DIR/.." && make par2cron-embed)

echo "==> Done"
