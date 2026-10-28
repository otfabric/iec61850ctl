# iec61850ctl interoperability examples

Generated from `e2e/testdata/examples.json`. Do not edit by hand; run `make e2e-docs`.

iec61850-interop: `v0.4.0` · IED fixture: `iec61850-v3` · IED name: `InteropIED`

See also [INTEROP.md](INTEROP.md) and [AUTOMATION.md](AUTOMATION.md).

## Environment

Against **external** adapter servers (libiec61850, IEC61850bean, dscsystems, iec61850-rust):

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
  iec61850-interop-libiec61850:local --port 1102

# iec61850bean
docker run --rm -p 1102:1102 --entrypoint iec61850bean-ied-server \
  iec61850-interop-iec61850bean:local --port 1102

# dscsystems
docker run --rm -p 1102:1102 --entrypoint dscsystems-ied-server \
  iec61850-interop-dscsystems:local --port 1102

# iec61850-rust
docker run --rm -p 1102:1102 --entrypoint iec61850-rust-ied-server \
  iec61850-interop-iec61850-rust:local --port 1102
```

### Digest-pinned images (CI / reproducible)

```bash
LIBIEC61850_IMAGE=ghcr.io/otfabric/iec61850-interop-libiec61850@sha256:966e17e37e7fb5c22b825443ce79565d51067705c8f8485fd63d4001b4bd31ff
IEC61850BEAN_IMAGE=ghcr.io/otfabric/iec61850-interop-iec61850bean@sha256:ab2cda6a77ea054a04a736fdf1c36c5c8f50c766aa41cf5818bd11f3fadfac7d
DSCSYSTEMS_IMAGE=ghcr.io/otfabric/iec61850-interop-dscsystems@sha256:899e4d96605a921647e7c079db0b0a7f69c05d0582830b81607bc1a152a8d60a
IEC61850RUST_IMAGE=ghcr.io/otfabric/iec61850-interop-iec61850-rust@sha256:c46c83c6d7eaa9800bbe48fb6ca9d25a4e9338d0bfc6f31fbba7813d89e4445d
docker run --rm -p 1102:1102 --entrypoint libiec61850-ied-server \
  "$LIBIEC61850_IMAGE" --port 1102
```

### Self-server smoke

```bash
iec61850ctl server start \
  --scl e2e/testdata/interop.icd \
  --values e2e/testdata/self-server-values.json \
  --ied-name InteropIED --fixture-id iec61850-v3 \
  --host 127.0.0.1 --port 1102 --ready-json
```

For reverse (external client → CLI server) examples, bind `--host 0.0.0.0` and pass `--add-host=host.docker.internal:host-gateway` to the adapter container. See [SERVER.md](SERVER.md).

## Examples

### List logical devices

Stacks: `libiec61850`, `iec61850bean`, `dscsystems`, `iec61850-rust`, `self`

Association + LD discovery. Against external adapters set IEC61850_IED_NAME=InteropIED so domains appear as InteropLD; against self-server leave IEC61850_IED_NAME unset.

```bash
iec61850ctl list lds --format json
```

Example stdout (illustrative):

```
[{"name":"InteropLD"}]
```

### List logical nodes

Stacks: `libiec61850`, `iec61850bean`, `dscsystems`, `iec61850-rust`, `self`

LN names under InteropLD.

```bash
iec61850ctl list lns --ld InteropLD --format json
```

Example stdout (illustrative):

```
[{"name":"LLN0"},{"name":"GGIO1"},…]
```

### Read MX float

Stacks: `libiec61850`, `iec61850bean`, `dscsystems`, `iec61850-rust`, `self`

Scalar float encoding (JSON number).

```bash
iec61850ctl get object --object InteropLD/MMXU1.TotW.mag.f --fc MX --format json
```

Example stdout (illustrative):

```
{"object":"InteropLD/MMXU1.TotW.mag.f","fc":"MX","type":"float","value":1234.5}
```

### Read ST boolean

Stacks: `libiec61850`, `iec61850bean`, `dscsystems`, `iec61850-rust`, `self`

SPS1.stVal seeded false in expected-values / self-server-values.

```bash
iec61850ctl get object --object InteropLD/GGIO1.SPS1.stVal --fc ST --format json
```

Example stdout (illustrative):

```
{"object":"InteropLD/GGIO1.SPS1.stVal","fc":"ST","type":"boolean","value":false}
```

### Get dataset

Stacks: `libiec61850`, `iec61850bean`, `dscsystems`, `iec61850-rust`, `self`

dsInterop has two members (SPS1.stVal, Mod.stVal).

```bash
iec61850ctl get ds --ld InteropLD --ln LLN0 --name dsInterop --format json
```

### Discover report instance names

Stacks: `libiec61850`, `iec61850bean`, `dscsystems`, `iec61850-rust`, `self`

SCL defines urcb01; some servers expand the instance to urcb0101. Always discover before subscribe — neither name is universally valid.

```bash
iec61850ctl list reports --ld InteropLD --ln LLN0 --format json
```

Example stdout (illustrative):

```
[{"ld":"InteropLD","ln":"LLN0","name":"urcb01…","buffered":false,"ref":"…"},…]
```

### URCB GI subscription (JSONL)

Stacks: `libiec61850`, `iec61850bean`, `dscsystems`, `iec61850-rust`

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

### Control inspect

Stacks: `libiec61850`, `iec61850bean`, `dscsystems`, `iec61850-rust`

Atomic control journeys require a single association; inspect ctlModel before operate.

```bash
iec61850ctl control inspect --object InteropLD/GGIO1.SPCSO2 --format json
```

### Control operate (atomic SBO)

Stacks: `libiec61850`, `iec61850bean`, `dscsystems`, `iec61850-rust`

One association: select → operate → optional confirm-ref. No cross-process select.

```bash
iec61850ctl control operate --object InteropLD/GGIO1.SPCSO2 --value true --type bool --mode auto --confirm-ref InteropLD/GGIO1.SPCSO2.stVal[ST] --format json
```

### Set scalar object (ING)

Stacks: `libiec61850`, `iec61850bean`, `dscsystems`, `iec61850-rust`

Writable fixture InteropLD/GGIO1.SetInt1.setVal[SP]. FC=CO is rejected — use control operate.

```bash
iec61850ctl set object --object InteropLD/GGIO1.SetInt1.setVal --fc SP --value 5 --type int --verify --format json
```

### BRCB subscribe with purge

Stacks: `libiec61850`, `iec61850bean`, `dscsystems`, `iec61850-rust`

Replace REPORT_INSTANCE with discovered brcb01*. --purge-buf/--entry-id require --type BR and run before enable.

```bash
iec61850ctl subscribe report --ld InteropLD --ln LLN0 --report REPORT_INSTANCE --type BR --purge-buf --interrogation --max-reports 1 --duration 15s --format jsonl
```

Discover the instance name first:

```bash
iec61850ctl list reports --ld InteropLD --ln LLN0 --format json
# then substitute --report <name> from the JSON `name` field
```

### GOOSE control blocks from SCL

Stacks: `offline`

Offline: lists the GSEControl blocks of the iec61850-interop GOOSE fixture (goose-v1).

```bash
iec61850ctl goose list --scl e2e/testdata/interop-goose.cid
```

### Sniff GOOSE from a peer publisher

Stacks: `libiec61850`, `dscsystems`, `iec61850-rust`

Raw Ethernet: run on a host or container attached to the same Docker bridge or LAN as the adapter's GOOSE publisher. Needs CAP_NET_RAW.

```bash
iec61850ctl goose sniff --iface eth0 --count 5 --format jsonl
```

### Supervised SV subscription

Stacks: `libiec61850`, `dscsystems`, `iec61850-rust`

Subscribes to the 9-2LE stream of the sv-v1 fixture and prints every 4000th sample, a stats record every second and a final summary. Needs CAP_NET_RAW.

```bash
iec61850ctl sv subscribe --iface eth0 --scl e2e/testdata/interop-sv.cid --svcb MSVCB01 --sample-every 4000 --duration 10s --format jsonl
```

### Read an MSVCB over MMS

Stacks: `libiec61850`, `dscsystems`

MSVCB02 of the IED fixture. iec61850bean and iec61850-rust do not model MSVCBs over MMS and fail this read.

```bash
iec61850ctl sv svcb get --ld InteropLD --svcb MSVCB02 --format json
```
