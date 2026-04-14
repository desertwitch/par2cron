#!/usr/bin/env bash
set -euo pipefail

BIN="${PAR2CRON_BIN:-$(pwd)/par2cron}"
CONFIG="${PAR2CRON_CONFIG:-$(pwd)/par2cron.yaml}"

PASS=0
FAIL=0

cleanup_dirs=()
cleanup() {
    for d in "${cleanup_dirs[@]}"; do
        rm -rf "$d"
    done
}
trap cleanup EXIT

mktest() {
    local dir
    dir=$(mktemp -d)
    cleanup_dirs+=("$dir")
    echo "$dir"
}

pass() {
    PASS=$((PASS + 1))
    echo "  PASS: $1"
}

fail() {
    FAIL=$((FAIL + 1))
    echo "  FAIL: $1"
    echo "::error file=e2e_test.sh::FAIL: $1"
}

assert_file_exists() {
    if [ -f "$1" ]; then
        pass "$2"
    else
        fail "$2 (assert_file_exists: file not found: $1)"
    fi
}

assert_glob_not_empty() {
    # shellcheck disable=SC2086
    if ls $1; then
        pass "$2"
    else
        fail "$2 (assert_glob_not_empty: no files matching: $1)"
    fi
}

assert_file_content() {
    local content
    content=$(cat "$1")
    if [ "$content" = "$2" ]; then
        pass "$3"
    else
        fail "$3 (assert_file_content: expected: '$2', got: '$content')"
    fi
}

assert_exit_zero() {
    if "$@"; then
        pass "${TESTNAME:-command succeeded}"
    else
        fail "${TESTNAME:-command should have succeeded} (assert_exit_zero: $*)"
    fi
}

assert_exit_nonzero() {
    if "$@"; then
        fail "${TESTNAME:-command should have failed} (assert_exit_nonzero: $*)"
    else
        pass "${TESTNAME:-command failed as expected}"
    fi
}

# ---------------------------------------------------------------------------
echo "==> Test: check-config (valid)"
TESTNAME="check-config accepts valid config"
assert_exit_zero "$BIN" check-config "$CONFIG"

# ---------------------------------------------------------------------------
echo "==> Test: check-config (invalid)"
TMPDIR_CFG=$(mktest)
cat > "$TMPDIR_CFG/bad.yaml" <<'EOF'
create:
  mode: "nested"
  glob: **/*.mp4"
EOF
TESTNAME="check-config rejects invalid config"
assert_exit_nonzero "$BIN" check-config "$TMPDIR_CFG/bad.yaml"

# ---------------------------------------------------------------------------
echo "==> Test: create"
DIR=$(mktest)
touch "$DIR/_par2cron"
echo "hello world" > "$DIR/testfile.txt"

TESTNAME="create succeeds"
assert_exit_zero "$BIN" create "$DIR" -- -vv
assert_glob_not_empty "$DIR/*.par2" "par2 files created"

# ---------------------------------------------------------------------------
echo "==> Test: create with custom args"
DIR=$(mktest)
touch "$DIR/_par2cron"
echo "hello world" > "$DIR/testfile.txt"

TESTNAME="create with custom args succeeds"
assert_exit_zero "$BIN" create "$DIR" -- -r15 -n1 -vv
assert_glob_not_empty "$DIR/*.par2" "par2 files created with custom args"

# ---------------------------------------------------------------------------
echo "==> Test: verify (clean)"
DIR=$(mktest)
touch "$DIR/_par2cron"
echo "hello world" > "$DIR/testfile.txt"
"$BIN" create "$DIR" -- -vv

TESTNAME="verify clean directory"
assert_exit_zero "$BIN" verify "$DIR" -- -vv

# ---------------------------------------------------------------------------
echo "==> Test: verify (corrupted, repairable)"
DIR=$(mktest)
touch "$DIR/_par2cron"
echo "hello world" > "$DIR/testfile.txt"
"$BIN" create "$DIR" -- -vv
echo "yello world" > "$DIR/testfile.txt"

TESTNAME="verify detects corruption"
assert_exit_nonzero "$BIN" verify "$DIR" -- -vv

# ---------------------------------------------------------------------------
echo "==> Test: repair"
DIR=$(mktest)
touch "$DIR/_par2cron"
echo "hello world" > "$DIR/testfile.txt"
"$BIN" create "$DIR" -- -vv
echo "yello world" > "$DIR/testfile.txt"

TESTNAME="verify detects corruption before repair"
assert_exit_nonzero "$BIN" verify "$DIR" -- -vv

TESTNAME="repair succeeds"
assert_exit_zero "$BIN" repair "$DIR" -- -vv
assert_file_content "$DIR/testfile.txt" "hello world" "repair restores original content"

# ---------------------------------------------------------------------------
echo "==> Test: repair with custom args"
DIR=$(mktest)
touch "$DIR/_par2cron"
echo "hello world" > "$DIR/testfile.txt"
"$BIN" create "$DIR" -- -vv
echo "yello world" > "$DIR/testfile.txt"

TESTNAME="verify detects corruption before repair (custom args)"
assert_exit_nonzero "$BIN" verify "$DIR" -- -vv

TESTNAME="repair with custom args succeeds"
assert_exit_zero "$BIN" repair "$DIR" -- -vv
assert_file_content "$DIR/testfile.txt" "hello world" "repair restores original content (custom args)"

# ---------------------------------------------------------------------------
echo "==> Test: info"
DIR=$(mktest)
touch "$DIR/_par2cron"
echo "hello world" > "$DIR/testfile.txt"
"$BIN" create "$DIR" -- -vv

TESTNAME="info succeeds"
assert_exit_zero "$BIN" info "$DIR"

# ---------------------------------------------------------------------------
echo "==> Test: pprof"
DIR=$(mktest)
touch "$DIR/_par2cron"
echo "hello world" > "$DIR/testfile.txt"
PROFDIR=$(mktest)

TESTNAME="pprof writes cpu profile"
assert_exit_zero "$BIN" --pprof "$PROFDIR/cpu.prof" create "$DIR" -- -vv
assert_file_exists "$PROFDIR/cpu.prof" "cpu.prof file created"

# ---------------------------------------------------------------------------
echo "==> Test: complex cron scenario (multi-directory)"
DIR1=$(mktest)
DIR2=$(mktest)
touch "$DIR1/_par2cron" "$DIR2/_par2cron"
echo "hello world" > "$DIR1/testfile1.txt"
echo "hello world" > "$DIR2/testfile2.txt"

TESTNAME="create multiple directories"
assert_exit_zero "$BIN" create "$DIR1" "$DIR2" -- -r15 -n1 -vv
assert_glob_not_empty "$DIR1/*.par2" "par2 files in dir1"
assert_glob_not_empty "$DIR2/*.par2" "par2 files in dir2"

echo "yello world" > "$DIR1/testfile1.txt"
echo "UNREPAIRABLE" > "$DIR2/testfile2.txt"

TESTNAME="verify detects corruption across directories"
assert_exit_nonzero "$BIN" verify "$DIR1" "$DIR2" -- -vv

TESTNAME="repair across directories"
assert_exit_zero "$BIN" repair "$DIR1" "$DIR2" -- -vv
assert_file_content "$DIR1/testfile1.txt" "hello world" "dir1 file restored"
assert_file_content "$DIR2/testfile2.txt" "UNREPAIRABLE" "dir2 file unchanged (unrepairable)"

TESTNAME="verify dir1 clean after repair"
assert_exit_zero "$BIN" verify "$DIR1" -- -vv

# ---------------------------------------------------------------------------
echo ""
echo "==========================================="
echo "  Results: $PASS passed, $FAIL failed"
echo "==========================================="

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
