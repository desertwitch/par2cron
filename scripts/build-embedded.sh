#!/bin/bash
set -euo pipefail

REPO="${1:-https://github.com/Parchive/par2cmdline}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
EMBED_DIR="${SCRIPT_DIR}/../cmd/par2cron/embed"
PATCHES_DIR="${SCRIPT_DIR}/patches"
JOBS="$(nproc 2>/dev/null || echo 2)"

BUILDDIR="$(mktemp -d)"
trap 'rm -rf "$BUILDDIR"' EXIT

echo "==> Cloning ${REPO}"
git clone --depth 1 "$REPO" "$BUILDDIR/par2cmdline"

pushd "$BUILDDIR/par2cmdline"
  if [ -d "$PATCHES_DIR" ]; then
    echo "==> Patching"
    for p in "$PATCHES_DIR"/*.patch; do
      [ -f "$p" ] && echo "$p" && patch -p1 < "$p"
    done
  fi

  if [ -f CMakeLists.txt ]; then
    # CMake (par2cmdline 2.x): no OpenMP anymore (std::thread), so nothing to
    # strip for a static build. Flags as in upstream's build/build-static.sh.
    echo "==> Building with CMake (static)"
    cmake -S . -B build-cmake \
      -DCMAKE_BUILD_TYPE=Release \
      -DPAR2_INSTALL=OFF \
      -DCMAKE_EXE_LINKER_FLAGS="-static"
    cmake --build build-cmake -j"$JOBS"

    echo "==> Running par2cmdline's test suite"
    ctest --test-dir build-cmake --output-on-failure -j"$JOBS"

    PAR2_BIN="$BUILDDIR/par2cmdline/build-cmake/par2"
  else
    # Autotools (par2cmdline 1.x): OpenMP (libgomp) does not link statically.
    echo "==> Building with autotools (static)"
    sed -i '/AC_OPENMP/d' configure.ac
    ./automake.sh
    ./configure LDFLAGS="-static"
    make -j"$JOBS"
    make check

    PAR2_BIN="$BUILDDIR/par2cmdline/par2"
  fi
popd

LDD_OUTPUT=$(ldd "$PAR2_BIN" 2>&1) || true
file "$PAR2_BIN" 2>&1 || true
echo "$LDD_OUTPUT"
if ! echo "$LDD_OUTPUT" | grep -qi "not a dynamic"; then
  echo "==> ERROR: par2 is not statically linked" >&2
  exit 1
fi
echo "==> Confirmed: par2 is statically linked"

echo "==> Copying par2 binary to ${EMBED_DIR}"
mkdir -p "$EMBED_DIR"
cp -f "$PAR2_BIN" "$EMBED_DIR/par2"

echo "==> Running make par2cron-embed"
(cd "$SCRIPT_DIR/.." && make par2cron-embed)

echo "==> Done"
