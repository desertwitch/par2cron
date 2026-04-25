#!/usr/bin/env bash
set -uo pipefail

fuzz_name="$1"
pkg_path="$2"
fuzztime="${3:-3m}"

tmplog=$(mktemp)
trap 'rm -f $tmplog' EXIT

go test -run='^$' -fuzz="$fuzz_name" -fuzztime="$fuzztime" "$pkg_path" 2>&1 | tee "$tmplog"
if [ "${PIPESTATUS[0]}" -ne 0 ]; then
    if grep -q "context deadline exceeded" "$tmplog"; then
        echo "::warning::ignoring spurious 'context deadline exceeded' (Go issue #75804)"
        exit 0
    fi
    exit 1
fi
