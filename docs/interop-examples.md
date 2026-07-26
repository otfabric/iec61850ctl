# iec61850ctl interoperability examples

Generated from `e2e/testdata/examples.json`. Do not edit by hand; run `make e2e-docs`.

Fixture: `iec61850-v2` · IED name: `InteropIED`

See also [INTEROP.md](INTEROP.md) and [AUTOMATION.md](AUTOMATION.md).

## Environment

Against **external** adapter servers (libiec61850 / Bean):

```bash
export IEC61850_HOST=127.0.0.1
export IEC61850_PORT=1102
export IEC61850_IED_NAME=InteropIED
```

Against **`iec61850ctl server start` (self-server)**, leave `IEC61850_IED_NAME` **unset** — Go domains are bare LD names.

## Starting adapters

### Local images

```bash
# libiec61850
docker run --rm -p 1102:1102 --entrypoint libiec61850-ied-server \
  mms-interop-libiec61850:local --port 1102

# IEC61850bean
docker run --rm -p 1102:1102 --entrypoint iec61850bean-ied-server \
  mms-interop-iec61850bean:local --port 1102
```

### Digest-pinned images (CI / reproducible)

```bash
LIBIEC61850_IMAGE=ghcr.io/otfabric/mms-interop-libiec61850@sha256:bbe651eea580acc3335de0d3587ccff1f4161126474234e446fb4f8bedea1e89
IEC61850BEAN_IMAGE=ghcr.io/otfabric/mms-interop-iec61850bean@sha256:708f559e0151e6a6580bf1ca95fea112b6c32eebbe4834a111198d00a5940f78
docker run --rm -p 1102:1102 --entrypoint libiec61850-ied-server \
  "$LIBIEC61850_IMAGE" --port 1102
```

### Self-server smoke

```bash
iec61850ctl server start \
  --scl e2e/testdata/interop.icd \
  --values e2e/testdata/self-server-values.json \
  --ied-name InteropIED --fixture-id iec61850-v2 \
  --host 127.0.0.1 --port 1102 --ready-json
```

For reverse (external client → CLI server) examples, bind `--host 0.0.0.0` and pass `--add-host=host.docker.internal:host-gateway` to the adapter container. See [SERVER.md](SERVER.md).

## Examples

### List logical devices

Stacks: `libiec61850`, `iec61850bean`, `self`

Association + LD discovery. Against external adapters set IEC61850_IED_NAME=InteropIED so domains appear as InteropLD; against self-server leave IEC61850_IED_NAME unset.

```bash
iec61850ctl list lds --format json
```

Example stdout (illustrative):

```
[{"name":"InteropLD"}]
```

### List logical nodes

Stacks: `libiec61850`, `iec61850bean`, `self`

LN names under InteropLD.

```bash
iec61850ctl list lns --ld InteropLD --format json
```

Example stdout (illustrative):

```
[{"name":"LLN0"},{"name":"GGIO1"},…]
```

### Read MX float

Stacks: `libiec61850`, `iec61850bean`, `self`

Scalar float encoding (JSON number).

```bash
iec61850ctl get object --object InteropLD/MMXU1.TotW.mag.f --fc MX --format json
```

Example stdout (illustrative):

```
{"object":"InteropLD/MMXU1.TotW.mag.f","fc":"MX","type":"float","value":1234.5}
```

### Read ST boolean

Stacks: `libiec61850`, `iec61850bean`, `self`

SPS1.stVal seeded false in expected-values / self-server-values.

```bash
iec61850ctl get object --object InteropLD/GGIO1.SPS1.stVal --fc ST --format json
```

Example stdout (illustrative):

```
{"object":"InteropLD/GGIO1.SPS1.stVal","fc":"ST","type":"boolean","value":false}
```

### Get dataset

Stacks: `libiec61850`, `iec61850bean`, `self`

dsInterop has two members (SPS1.stVal, Mod.stVal).

```bash
iec61850ctl get ds --ld InteropLD --ln LLN0 --name dsInterop --format json
```

### Discover report instance names

Stacks: `libiec61850`, `iec61850bean`, `self`

SCL defines urcb01; some servers expand the instance to urcb0101. Always discover before subscribe — neither name is universally valid.

```bash
iec61850ctl list reports --ld InteropLD --ln LLN0 --format json
```

Example stdout (illustrative):

```
[{"ld":"InteropLD","ln":"LLN0","name":"urcb01…","buffered":false,"ref":"…"},…]
```

### URCB GI subscription (JSONL)

Stacks: `libiec61850`, `iec61850bean`

Replace REPORT_INSTANCE with the name returned by list reports (often urcb01 or urcb0101). Do not hard-code a single instance across stacks.

```bash
iec61850ctl subscribe report --ld InteropLD --ln LLN0 --report REPORT_INSTANCE --type RP --interrogation --max-reports 1 --duration 15s --show-values --format jsonl
```

Example stdout (illustrative):

```
{"event":"report",…}
{"event":"summary","reports_received":1,"clean_disable":true,…}
```

Discover the instance name first:

```bash
iec61850ctl list reports --ld InteropLD --ln LLN0 --format json
# then substitute --report <name> from the JSON `name` field
```
