#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIR="$ROOT/e2e/testdata"
LOCK="$DIR/interop.lock.json"
IEC61850_INTEROP_DIR="${IEC61850_INTEROP_DIR:-}"

if [[ -z "$IEC61850_INTEROP_DIR" ]]; then
  echo "usage: IEC61850_INTEROP_DIR=/path/to/iec61850-interop $0" >&2
  echo "optional: LIBIEC61850_IMAGE=... IEC61850BEAN_IMAGE=... DSCSYSTEMS_IMAGE=... IEC61850RUST_IMAGE=..." >&2
  exit 1
fi

ICD_SRC="$IEC61850_INTEROP_DIR/fixtures/iec61850/interop.icd"
VAL_SRC="$IEC61850_INTEROP_DIR/fixtures/iec61850/values.json"
GOOSE_CID_SRC="$IEC61850_INTEROP_DIR/fixtures/goose/interop-goose.cid"
GOOSE_JSON_SRC="$IEC61850_INTEROP_DIR/fixtures/goose/goose.json"
SV_CID_SRC="$IEC61850_INTEROP_DIR/fixtures/sv/interop-sv.cid"
SV_JSON_SRC="$IEC61850_INTEROP_DIR/fixtures/sv/sv.json"
CAPS_SRC="$IEC61850_INTEROP_DIR/fixtures/capabilities.json"
for f in "$ICD_SRC" "$VAL_SRC" "$GOOSE_CID_SRC" "$GOOSE_JSON_SRC" "$SV_CID_SRC" "$SV_JSON_SRC" "$CAPS_SRC"; do
  if [[ ! -f "$f" ]]; then
    echo "iec61850-interop fixture not found: $f" >&2
    exit 1
  fi
done

mkdir -p "$DIR"
cp "$ICD_SRC" "$DIR/interop.icd"
cp "$VAL_SRC" "$DIR/expected-values.json"
cp "$GOOSE_CID_SRC" "$DIR/interop-goose.cid"
cp "$GOOSE_JSON_SRC" "$DIR/goose.json"
cp "$SV_CID_SRC" "$DIR/interop-sv.cid"
cp "$SV_JSON_SRC" "$DIR/sv.json"
cp "$SV_CID_SRC" "$ROOT/pkg/svops/testdata/interop-sv.cid"

if [[ ! -f "$DIR/self-server-values.json" ]]; then
  echo "missing self-server-values.json — create it before updating the lock" >&2
  exit 1
fi

SOURCE_COMMIT="$(git -C "$IEC61850_INTEROP_DIR" rev-parse HEAD)"
SOURCE_TAG="$(git -C "$IEC61850_INTEROP_DIR" describe --tags --exact-match 2>/dev/null || git -C "$IEC61850_INTEROP_DIR" describe --tags 2>/dev/null || true)"

# Images: <STACK>_IMAGE overrides, else the digest already in the lock, else
# the local build tag. The image digests of a release are in the
# iec61850-interop release manifest.json.
SOURCE_COMMIT="$SOURCE_COMMIT" SOURCE_TAG="$SOURCE_TAG" python3 - "$DIR" "$LOCK" "$CAPS_SRC" <<'PY2'
import hashlib, json, os, sys
d, lock_path, caps_path = sys.argv[1:]
caps = json.load(open(caps_path))
old = {}
if os.path.exists(lock_path):
    old = json.load(open(lock_path)).get("images", {})
stacks = {
    "libiec61850": "LIBIEC61850_IMAGE",
    "iec61850bean": "IEC61850BEAN_IMAGE",
    "dscsystems": "DSCSYSTEMS_IMAGE",
    "iec61850-rust": "IEC61850RUST_IMAGE",
}
goose = ["libiec61850", "dscsystems", "iec61850-rust"]
sv = [s for s in stacks if caps["commands"].get(f"{s}-sv-publisher")]
files = ["interop.icd", "expected-values.json", "self-server-values.json", "interop-goose.cid", "goose.json",
         "interop-sv.cid", "sv.json"]
lock = {
    "source_repository": "otfabric/iec61850-interop",
    "source_commit": os.environ["SOURCE_COMMIT"],
    "source_tag": os.environ["SOURCE_TAG"],
    "fixture_revision": caps["fixtureRevision"],
    "goose_fixture_revision": json.load(open(os.path.join(d, "goose.json")))["revision"],
    "sv_fixture_revision": json.load(open(os.path.join(d, "sv.json")))["revision"],
    "svcb_over_mms": {s: bool(caps.get("svcbOverMms", {}).get(s)) for s in stacks},
    "files": {f: "sha256:" + hashlib.sha256(open(os.path.join(d, f), "rb").read()).hexdigest() for f in files},
    "images": {s: os.environ.get(env) or old.get(s) or f"iec61850-interop-{s}:local" for s, env in stacks.items()},
    "required_commands": {},
}
for s in stacks:
    for c in ("ied-server", "ied-client", "ied-controller", "ied-reporter"):
        lock["required_commands"][f"{s}-{c}"] = True
for s in goose:
    for c in ("goose-publisher", "goose-subscriber"):
        lock["required_commands"][f"{s}-{c}"] = True
for s in sv:
    for c in ("sv-publisher", "sv-subscriber"):
        lock["required_commands"][f"{s}-{c}"] = True
with open(lock_path, "w") as f:
    json.dump(lock, f, indent=2)
    f.write("\n")
print("updated", lock_path)
print("source_commit", lock["source_commit"])
print("source_tag", lock["source_tag"])
PY2
