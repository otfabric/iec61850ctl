#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LOCK="$ROOT/e2e/testdata/interop.lock.json"
DIR="$ROOT/e2e/testdata"

if [[ ! -f "$LOCK" ]]; then
  echo "missing lock file: $LOCK" >&2
  exit 1
fi

verify_file() {
  local name="$1"
  local expected="$2"
  local path="$DIR/$name"
  if [[ ! -f "$path" ]]; then
    echo "missing fixture file: $path" >&2
    exit 1
  fi
  local got
  got="sha256:$(shasum -a 256 "$path" | awk '{print $1}')"
  if [[ "$got" != "$expected" ]]; then
    echo "hash mismatch for $name" >&2
    echo "  expected: $expected" >&2
    echo "  got:      $got" >&2
    exit 1
  fi
}

while IFS=$'\t' read -r name hash; do
  [[ -z "$name" ]] && continue
  verify_file "$name" "$hash"
done < <(python3 - <<'PY' "$LOCK"
import json, sys
lock = json.load(open(sys.argv[1]))
for name, h in lock.get("files", {}).items():
    print(f"{name}\t{h}")
PY
)

echo "e2e fixtures verified against interop.lock.json"
