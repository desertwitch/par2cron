#!/usr/bin/env bash
set -euo pipefail

# --- Download latest par2cmdline release ---
echo "Fetching latest par2cmdline release..."
DOWNLOAD_URL=$(curl -s -H "Authorization: Bearer ${GITHUB_TOKEN}" \
    "https://api.github.com/repos/Parchive/par2cmdline/releases/latest" \
    | jq -r '.assets[] | select(.name | endswith("-linux-amd64.zip")) | .browser_download_url')

if [ -z "$DOWNLOAD_URL" ]; then
    echo "::error::No *-linux-amd64.zip asset found in the latest par2cmdline release."
    exit 1
fi

WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

echo "Downloading $DOWNLOAD_URL"
curl -sL "$DOWNLOAD_URL" -o "$WORK_DIR/par2cmdline.zip"
unzip -q "$WORK_DIR/par2cmdline.zip" -d "$WORK_DIR/par2cmdline_tool"

PAR2=$(find "$WORK_DIR/par2cmdline_tool" -name "par2" -type f | head -1)
if [ -z "$PAR2" ]; then
    echo "::error::Could not find par2 in the par2cmdline release."
    exit 1
fi
chmod +x "$PAR2"
echo "Found par2 at: $PAR2"

# --- Bundle definitions ---
# The bundles are copied from testdata/generated rather than regenerated:
# CI runs `make generate` and `make is-clean` first, so the committed files
# are guaranteed byte-identical to what the packer currently produces.
REPO_ROOT=$(git rev-parse --show-toplevel)
TESTDATA_DIR="$REPO_ROOT/internal/bundle/testdata"
SOURCES_DIR="$TESTDATA_DIR/sources"
GENERATED_DIR="$TESTDATA_DIR/generated"
VERIFY_DIR="$WORK_DIR/verify"

for d in "$SOURCES_DIR" "$GENERATED_DIR"; do
    if [ ! -d "$d" ]; then
        echo "::error::Directory not found: $d"
        exit 1
    fi
done

declare -a BUNDLE_NAMES=(
    "multipar"
    "par2cmdline"
    "par2cmdline-turbo"
    "parpar"
    "quickpar"
)

# --- Verify, damage, repair each committed bundle ---
FAILED=0

for NAME in "${BUNDLE_NAMES[@]}"; do
    PAR2_FILE="$NAME.p2c.par2"

    echo "============================================"
    echo "Processing: $NAME"
    echo "============================================"

    # Fresh verify dir so only this bundle is present
    rm -rf "$VERIFY_DIR"
    mkdir -p "$VERIFY_DIR"

    if [ ! -f "$GENERATED_DIR/$PAR2_FILE" ]; then
        echo "::error::Committed bundle not found: $GENERATED_DIR/$PAR2_FILE"
        FAILED=1
        continue
    fi

    # Copy the committed bundle and the source files into the verify dir
    cp "$GENERATED_DIR/$PAR2_FILE" "$VERIFY_DIR/"
    cp -r "$SOURCES_DIR"/* "$VERIFY_DIR/"
    echo "Copied $PAR2_FILE and source files into $VERIFY_DIR"

    # Verify with par2
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

if [ "$FAILED" -ne 0 ]; then
    echo "::error::One or more bundles failed verification/repair."
    exit 1
fi

echo "All bundles verified and repaired successfully."
