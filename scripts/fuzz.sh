#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
#
# Run every native Go fuzz target for a short time (FUZZTIME, default 10s).
# Crashers land in the package's testdata/fuzz directory; commit them as
# regression seeds after fixing the bug.
set -euo pipefail

fuzztime="${FUZZTIME:-10s}"
export GOWORK=off

grep -rl --include='*_test.go' '^func Fuzz' pkg internal cmd 2>/dev/null | sort | while read -r f; do
	dir="./$(dirname "$f")"
	for target in $(grep -o '^func Fuzz[A-Za-z0-9_]*' "$f" | sed 's/^func //'); do
		echo "fuzz: $dir $target ($fuzztime)"
		go test -run='^$' -fuzz="^${target}\$" -fuzztime="$fuzztime" "$dir"
	done
done
