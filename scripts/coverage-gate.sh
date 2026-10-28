#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
#
# Run the unit tests with coverage and fail when any package with statements
# is below the threshold (default 90%). Writes coverage.out (all packages).
# Runs without -race so it needs no cgo; races are checked by `make test`
# and the CI Race jobs.
#
#   COVERAGE_MIN=90 scripts/coverage-gate.sh
set -euo pipefail

min="${COVERAGE_MIN:-90}"
export GOWORK=off

out="$(CGO_ENABLED=0 go test -count=1 -covermode=atomic \
	-coverprofile=coverage.out ./... 2>&1)" || { echo "$out"; exit 1; }
echo "$out"

fail=0
while read -r pkg pct; do
	if awk -v p="$pct" -v m="$min" 'BEGIN { exit !(p < m) }'; then
		echo "coverage gate: $pkg at $pct% is below $min%" >&2
		fail=1
	fi
done < <(echo "$out" | awk '$1 == "ok" && /coverage:/ && !/\[no statements\]/ {
	for (i = 1; i <= NF; i++) if ($i == "coverage:") { sub("%", "", $(i+1)); print $2, $(i+1) }
}')

total="$(go tool cover -func=coverage.out | awk '/^total:/ { print $3 }')"
echo "total coverage: $total (gate: every package >= $min%)"
exit "$fail"
