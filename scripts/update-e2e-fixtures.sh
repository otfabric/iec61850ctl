#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIR="$ROOT/e2e/testdata"
LOCK="$DIR/interop.lock.json"
MMS_INTEROP_DIR="${MMS_INTEROP_DIR:-}"

if [[ -z "$MMS_INTEROP_DIR" ]]; then
  echo "usage: MMS_INTEROP_DIR=/path/to/mms-interop $0" >&2
  echo "optional: LIBIEC61850_IMAGE=... IEC61850BEAN_IMAGE=..." >&2
  exit 1
fi

ICD_SRC="$MMS_INTEROP_DIR/fixtures/iec61850/interop.icd"
VAL_SRC="$MMS_INTEROP_DIR/fixtures/iec61850/values.json"
if [[ ! -f "$ICD_SRC" || ! -f "$VAL_SRC" ]]; then
  echo "mms-interop fixtures not found under $MMS_INTEROP_DIR" >&2
  exit 1
fi

mkdir -p "$DIR"
cp "$ICD_SRC" "$DIR/interop.icd"
cp "$VAL_SRC" "$DIR/expected-values.json"

if [[ ! -f "$DIR/self-server-values.json" ]]; then
  echo "missing self-server-values.json — create it before updating the lock" >&2
  exit 1
fi

SOURCE_COMMIT="$(git -C "$MMS_INTEROP_DIR" rev-parse HEAD)"
SOURCE_TAG="$(git -C "$MMS_INTEROP_DIR" describe --tags --exact-match 2>/dev/null || git -C "$MMS_INTEROP_DIR" describe --tags 2>/dev/null || true)"

ICD_HASH="sha256:$(shasum -a 256 "$DIR/interop.icd" | awk '{print $1}')"
VAL_HASH="sha256:$(shasum -a 256 "$DIR/expected-values.json" | awk '{print $1}')"
SELF_HASH="sha256:$(shasum -a 256 "$DIR/self-server-values.json" | awk '{print $1}')"

LIB_IMAGE="${LIBIEC61850_IMAGE:-}"
BEAN_IMAGE="${IEC61850BEAN_IMAGE:-}"
if [[ -f "$LOCK" ]]; then
  if [[ -z "$LIB_IMAGE" ]]; then
    LIB_IMAGE="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["images"]["libiec61850"])' "$LOCK")"
  fi
  if [[ -z "$BEAN_IMAGE" ]]; then
    BEAN_IMAGE="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["images"]["iec61850bean"])' "$LOCK")"
  fi
fi
LIB_IMAGE="${LIB_IMAGE:-mms-interop-libiec61850:local}"
BEAN_IMAGE="${BEAN_IMAGE:-mms-interop-iec61850bean:local}"

python3 - <<PY
import json
lock = {
  "source_repository": "otfabric/mms-interop",
  "source_commit": "$SOURCE_COMMIT",
  "source_tag": "$SOURCE_TAG",
  "fixture_revision": "iec61850-v2",
  "files": {
    "interop.icd": "$ICD_HASH",
    "expected-values.json": "$VAL_HASH",
    "self-server-values.json": "$SELF_HASH",
  },
  "images": {
    "libiec61850": "$LIB_IMAGE",
    "iec61850bean": "$BEAN_IMAGE",
  },
  "required_commands": {
    "libiec61850-ied-server": True,
    "iec61850bean-ied-server": True,
  },
}
with open("$LOCK", "w") as f:
  json.dump(lock, f, indent=2)
  f.write("\n")
print("updated", "$LOCK")
print("source_commit", "$SOURCE_COMMIT")
print("source_tag", "$SOURCE_TAG")
PY
