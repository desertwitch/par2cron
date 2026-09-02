#!/usr/bin/env bash
set -euo pipefail

# --- Configuration ---
# Override via environment if you need a fork, a tag, or a specific commit.
PAR2_REPO="${PAR2_REPO:-https://github.com/Parchive/par2cmdline.git}"
PAR2_REF="${PAR2_REF:-master}"
# Set to 1 to also run par2cmdline's own test suite (slow).
RUN_PAR2_SELFTEST="${RUN_PAR2_SELFTEST:-0}"

# Resolve the repo root before we cd anywhere else.
REPO_ROOT=$(git rev-parse --show-toplevel)

# --- Check build prerequisites ---
MISSING=""
for tool in git autoconf automake make g++ unzip; do
    command -v "$tool" > /dev/null 2>&1 || MISSING="$MISSING $tool"
done
if [ -n "$MISSING" ]; then
    echo "::error::Missing required build tools:$MISSING"
    echo "On Debian/Ubuntu: sudo apt-get install -y build-essential autoconf automake"
    exit 1
fi

WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

# --- Clone and build par2cmdline from source ---
SRC_DIR="$WORK_DIR/par2cmdline"

echo "Cloning $PAR2_REPO (ref: $PAR2_REF)..."
if ! git clone --depth 1 --branch "$PAR2_REF" "$PAR2_REPO" "$SRC_DIR"; then
    # --branch only accepts branches and tags; fall back to a full clone for a raw SHA.
    echo "Shallow clone of '$PAR2_REF' failed, retrying with a full clone..."
    rm -rf "$SRC_DIR"
    git clone "$PAR2_REPO" "$SRC_DIR"
    git -C "$SRC_DIR" checkout "$PAR2_REF"
fi

PAR2_COMMIT=$(git -C "$SRC_DIR" rev-parse --short HEAD)
echo "Building par2cmdline at commit $PAR2_COMMIT..."

pushd "$SRC_DIR" > /dev/null
if [ -x ./automake.sh ]; then
    ./automake.sh
else
    autoreconf -fiv
fi
./configure
make -j"$(nproc 2>/dev/null || echo 2)"

if [ "$RUN_PAR2_SELFTEST" = "1" ]; then
    echo "Running par2cmdline's own test suite..."
    if ! make check; then
        echo "::error::par2cmdline 'make check' failed."
        popd > /dev/null
        exit 1
    fi
fi
popd > /dev/null

PAR2="$SRC_DIR/par2"
if [ ! -x "$PAR2" ]; then
    echo "::error::Build finished but no par2 binary at $PAR2"
    exit 1
fi
echo "Built par2 at: $PAR2 (commit $PAR2_COMMIT)"

# --- Bundle definitions ---
TMP_SUBDIR="_verify"
BUNDLE_DIR="$REPO_ROOT/internal/bundle"
TESTDATA_DIR="$BUNDLE_DIR/testdata"
SOURCES_DIR="$TESTDATA_DIR/sources"
VERIFY_DIR="$TESTDATA_DIR/$TMP_SUBDIR"

if [ ! -d "$SOURCES_DIR" ]; then
    echo "::error::Sources directory not found: $SOURCES_DIR"
    exit 1
fi

declare -a BUNDLE_NAMES=(
    "multipar"
    "par2cmdline"
    "par2cmdline-turbo"
    "parpar"
    "quickpar"
)

declare -a BUNDLE_ARGS=(
    "-dir testdata -out $TMP_SUBDIR/multipar.p2c.par2 -parse multipar/files.par2 multipar/files.par2 multipar/files.vol00+7.par2 multipar/files.vol07+6.par2 multipar/files.vol13+6.par2"
    "-dir testdata -out $TMP_SUBDIR/par2cmdline.p2c.par2 -parse par2cmdline/files.par2 par2cmdline/files.par2 par2cmdline/files.vol0+1.par2 par2cmdline/files.vol1+1.par2 par2cmdline/files.vol2+1.par2"
    "-dir testdata -out $TMP_SUBDIR/par2cmdline-turbo.p2c.par2 -parse par2cmdline-turbo/files.par2 par2cmdline-turbo/files.par2 par2cmdline-turbo/files.vol0+1.par2 par2cmdline-turbo/files.vol1+1.par2 par2cmdline-turbo/files.vol2+1.par2"
    "-dir testdata -out $TMP_SUBDIR/parpar.p2c.par2 -parse parpar/files.par2 parpar/files.par2 parpar/files.vol00+05.par2 parpar/files.vol05+05.par2 parpar/files.vol10+03.par2"
    "-dir testdata -out $TMP_SUBDIR/quickpar.p2c.par2 -parse quickpar/files.par2 quickpar/files.par2 quickpar/files.vol0+1.PAR2 quickpar/files.vol1+1.PAR2 quickpar/files.vol2+2.PAR2"
)

# --- Generate, verify, damage, repair each bundle ---
FAILED=0

cleanup_verify() {
    rm -rf "$VERIFY_DIR"
}

for i in "${!BUNDLE_NAMES[@]}"; do
    NAME="${BUNDLE_NAMES[$i]}"
    ARGS="${BUNDLE_ARGS[$i]}"

    echo "============================================"
    echo "Processing: $NAME"
    echo "============================================"

    # Clean the verify dir so only this bundle is present
    cleanup_verify
    mkdir -p "$VERIFY_DIR"

    # Generate the bundle
    echo "Running: go run ../../tool/generate-bundle $ARGS"
    pushd "$BUNDLE_DIR" > /dev/null
    # shellcheck disable=SC2086
    if ! go run ../../tool/generate-bundle $ARGS; then
        echo "::error::generate-bundle failed for $NAME"
        FAILED=1
        popd > /dev/null
        continue
    fi
    popd > /dev/null

    # Copy source files into the verify dir
    cp -r "$SOURCES_DIR"/* "$VERIFY_DIR/"
    echo "Copied source files into $VERIFY_DIR"

    # Verify with par2
    PAR2_FILE="$NAME.p2c.par2"
    echo "Verifying: $PAR2_FILE"
    pushd "$VERIFY_DIR" > /dev/null
    if ! "$PAR2" v -q "./$PAR2_FILE"; then
        echo "::error::Verification of $NAME failed!"
        FAILED=1
        popd > /dev/null
        continue
    fi
    echo "OK: $NAME verified successfully."
    popd > /dev/null

    # Record bundle file MD5 before damaging anything
    BUNDLE_PATH="$VERIFY_DIR/$PAR2_FILE"
    MD5_BEFORE=$(md5sum "$BUNDLE_PATH" | awk '{print $1}')
    echo "Bundle MD5 before repair: $MD5_BEFORE"

    # Clip one byte from each source file
    echo "Damaging source files for repair test..."
    for sf in "$VERIFY_DIR"/*; do
        [ "$(basename "$sf")" = "$PAR2_FILE" ] && continue
        [ ! -f "$sf" ] && continue
        SIZE=$(stat -c%s "$sf")
        if [ "$SIZE" -gt 0 ]; then
            truncate -s -1 "$sf"
            NEW_SIZE=$(stat -c%s "$sf")
            echo "  Clipped 1 byte from $(basename "$sf") ($SIZE -> $NEW_SIZE)"
        fi
    done

    # Repair with par2
    echo "Repairing: $PAR2_FILE"
    pushd "$VERIFY_DIR" > /dev/null
    if "$PAR2" r -q "./$PAR2_FILE"; then
        echo "OK: $NAME repaired successfully (exit code 0)."
    else
        echo "::error::Repair of $NAME failed with exit code $?!"
        FAILED=1
        popd > /dev/null
        continue
    fi
    popd > /dev/null

    # Verify bundle file was not modified during repair
    MD5_AFTER=$(md5sum "$BUNDLE_PATH" | awk '{print $1}')
    echo "Bundle MD5 after repair:  $MD5_AFTER"
    if [ "$MD5_BEFORE" != "$MD5_AFTER" ]; then
        echo "::error::Bundle file $PAR2_FILE was modified during repair! ($MD5_BEFORE -> $MD5_AFTER)"
        FAILED=1
    else
        echo "OK: Bundle file unchanged after repair."
    fi

    echo ""
done

cleanup_verify

if [ "$FAILED" -ne 0 ]; then
    echo "::error::One or more bundles failed verification/repair."
    exit 1
fi

echo "All bundles verified and repaired successfully (par2cmdline @ $PAR2_COMMIT)."
