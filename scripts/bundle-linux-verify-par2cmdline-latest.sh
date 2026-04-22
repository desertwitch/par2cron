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
TMP_SUBDIR="_verify"
REPO_ROOT=$(git rev-parse --show-toplevel)
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

# --- Generate and verify each bundle ---
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
    if "$PAR2" v -q "./$PAR2_FILE"; then
        echo "OK: $NAME verified successfully (exit code 0)."
    else
        echo "::error::Verification of $NAME failed with exit code $?!"
        FAILED=1
    fi
    popd > /dev/null

    echo ""
done

cleanup_verify

if [ "$FAILED" -ne 0 ]; then
    echo "::error::One or more bundles failed verification."
    exit 1
fi

echo "All bundles verified successfully."
