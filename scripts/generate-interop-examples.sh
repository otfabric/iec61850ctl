#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SRC="$ROOT/e2e/testdata/examples.json"
LOCK="$ROOT/e2e/testdata/interop.lock.json"
OUT="$ROOT/docs/interop-examples.md"

mkdir -p "$(dirname "$OUT")"

python3 - <<'PY' "$SRC" "$LOCK" "$OUT"
import json, sys
src, lock_path, out = sys.argv[1], sys.argv[2], sys.argv[3]
data = json.load(open(src))
lock = json.load(open(lock_path))
images = lock.get("images", {})
stacks = [
    ("libiec61850", "LIBIEC61850_IMAGE", "libiec61850-ied-server"),
    ("iec61850bean", "IEC61850BEAN_IMAGE", "iec61850bean-ied-server"),
    ("dscsystems", "DSCSYSTEMS_IMAGE", "dscsystems-ied-server"),
    ("iec61850-rust", "IEC61850RUST_IMAGE", "iec61850-rust-ied-server"),
]

lines = []
lines.append(f"# {data['title']}\n")
lines.append("Generated from `e2e/testdata/examples.json`. Do not edit by hand; run `make e2e-docs`.\n")
lines.append(f"iec61850-interop: `{lock.get('source_tag', '?')}` · IED fixture: `{data['fixture']}` · IED name: `{data['ied_name']}`\n")
lines.append("See also [INTEROP.md](INTEROP.md) and [AUTOMATION.md](AUTOMATION.md).\n")

lines.append("## Environment\n")
lines.append("Against **external** adapter servers (libiec61850, IEC61850bean, dscsystems, iec61850-rust):\n")
lines.append("```bash")
lines.append("export IEC61850_HOST=127.0.0.1")
lines.append("export IEC61850_PORT=1102")
lines.append(f"export IEC61850_IED_NAME={data['ied_name']}")
lines.append("```\n")
lines.append("Against **`iec61850ctl server start` (self-server)**, leave `IEC61850_IED_NAME` **unset** — Go domains are bare LD names.\n")

lines.append("## Starting adapters\n")
lines.append("### Local images\n")
lines.append("```bash")
for i, (name, _, entry) in enumerate(stacks):
    if i:
        lines.append("")
    lines.append(f"# {name}")
    lines.append(f"docker run --rm -p 1102:1102 --entrypoint {entry} \\")
    lines.append(f"  iec61850-interop-{name}:local --port 1102")
lines.append("```\n")
lines.append("### Digest-pinned images (CI / reproducible)\n")
lines.append("```bash")
for name, var, _ in stacks:
    lines.append(f"{var}={images.get(name, 'ghcr.io/otfabric/iec61850-interop-' + name + '@sha256:<digest>')}")
lines.append("docker run --rm -p 1102:1102 --entrypoint libiec61850-ied-server \\")
lines.append("  \"$LIBIEC61850_IMAGE\" --port 1102")
lines.append("```\n")
lines.append("### Self-server smoke\n")
lines.append("```bash")
lines.append("iec61850ctl server start \\")
lines.append("  --scl e2e/testdata/interop.icd \\")
lines.append("  --values e2e/testdata/self-server-values.json \\")
lines.append(f"  --ied-name InteropIED --fixture-id {data['fixture']} \\")
lines.append("  --host 127.0.0.1 --port 1102 --ready-json")
lines.append("```\n")
lines.append("For reverse (external client → CLI server) examples, bind `--host 0.0.0.0` and pass `--add-host=host.docker.internal:host-gateway` to the adapter container. See [SERVER.md](SERVER.md).\n")

lines.append("## Examples\n")
for case in data["cases"]:
    lines.append(f"### {case['title']}\n")
    stacks = case.get("stacks") or []
    if stacks:
        lines.append("Stacks: " + ", ".join(f"`{s}`" for s in stacks) + "\n")
    notes = case.get("notes")
    if notes:
        lines.append(f"{notes}\n")
    argv = " ".join(case["argv"])
    lines.append("```bash")
    lines.append(f"iec61850ctl {argv}")
    lines.append("```\n")
    exp = case.get("expected_output")
    if exp:
        lines.append("Example stdout (illustrative):\n")
        lines.append("```")
        lines.append(exp)
        lines.append("```\n")
    if "REPORT_INSTANCE" in argv:
        lines.append("Discover the instance name first:\n")
        lines.append("```bash")
        lines.append("iec61850ctl list reports --ld InteropLD --ln LLN0 --format json")
        lines.append("# then substitute --report <name> from the JSON `name` field")
        lines.append("```\n")

open(out, "w").write("\n".join(lines))
print("wrote", out)
PY
