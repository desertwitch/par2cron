#!/usr/bin/env bash
set -uo pipefail

fuzz_name="$1"
pkg_path="$2"
fuzztime="${3:-3m}"

go test -run="^${fuzz_name}\$" -fuzz="^${fuzz_name}\$" -fuzztime="$fuzztime" "$pkg_path" 2>&1
