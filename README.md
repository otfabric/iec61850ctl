# iec61850ctl

[![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Go Reference](https://pkg.go.dev/badge/github.com/otfabric/iec61850ctl.svg)](https://pkg.go.dev/github.com/otfabric/iec61850ctl)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI](https://github.com/otfabric/iec61850ctl/actions/workflows/ci.yml/badge.svg)](https://github.com/otfabric/iec61850ctl/actions/workflows/ci.yml)
[![E2E](https://github.com/otfabric/iec61850ctl/actions/workflows/e2e.yml/badge.svg)](https://github.com/otfabric/iec61850ctl/actions/workflows/e2e.yml)
[![Codecov](https://codecov.io/gh/otfabric/iec61850ctl/graph/badge.svg)](https://codecov.io/gh/otfabric/iec61850ctl)
[![Release](https://img.shields.io/github/v/release/otfabric/iec61850ctl?label=release)](https://github.com/otfabric/iec61850ctl/releases)

A command-line tool for IEC 61850 MMS, built in pure Go on [otfabric/go-iec61850](https://github.com/otfabric/go-iec61850) and [otfabric/go-mms](https://github.com/otfabric/go-mms).

`iec61850ctl` gives operators and developers terminal access to IEC 61850 IEDs: browse the data model, read and write scalar objects, operate controllable objects, subscribe to reports (including BRCB purge/resume), transfer files, query journals, parse SCL offline, run a local MMS server from SCL, and optionally expose a small read-only HTTP/JSON API.

**No CGO.** Binaries are static Go builds — no libiec61850 native library at runtime.

Black-box e2e exercises the built CLI independently against [libiec61850](https://libiec61850.com) and [IEC61850bean](https://www.beanit.com/iec-61850/) adapter servers from [mms-interop](https://github.com/otfabric/mms-interop), plus reverse controller/reporter/client journeys against `server start`. See [docs/INTEROP.md](docs/INTEROP.md).

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Command Reference](#command-reference)
- [Usage Examples](#usage-examples)
- [Global Flags and Environment](#global-flags-and-environment)
- [Output Formats](#output-formats)
- [Development](#development)
- [Built With](#built-with)
- [Documentation](#documentation)
- [License](#license)

## Features

- **Discover** — TCP scan of a subnet/range for MMS (port 102) responders
- **Browse** — list LDs, LNs, DOs, DAs, datasets, reports, journals, and files
- **Read** — objects (with FC), datasets, report configuration, journal entries, file download
- **Tree** — hierarchical or flat model walk; `--serialize` JSON for value seeding
- **Find** — path search by LN pattern + DO/DA path; bulk mapping file support
- **Subscribe** — report control blocks (BR/RP) with GI, optional dataset sync, BRCB purge/resume EntryID, clean disable on exit
- **Control** — atomic `control inspect` / `control operate` (auto/direct/SBO/SBOw on one association)
- **Write** — `set object` for scalar MMS attributes (rejects `FC=CO`; use control for CO)
- **SCL** — offline parse/flatten and CSV convert (no device connection)
- **Server** — run an MMS server from ICD/CID/SCD (`--scl`); optional value seed from serialized JSON
- **Export** — generate libiec61850 `.cfg` from serialized JSON (**export-only**, not used by this tool’s runtime)
- **HTTP** — REST JSON API for browse/read/find (**no** control or write over HTTP)

See [RELEASE.md](RELEASE.md) for the latest release notes. Protocol model primer: [docs/PROTOCOL.md](docs/PROTOCOL.md).

## Installation

### Pre-built binaries

Download from [GitHub Releases](https://github.com/otfabric/iec61850ctl/releases).

### From source

Requires Go 1.24+.

```sh
git clone https://github.com/otfabric/iec61850ctl.git
cd iec61850ctl
make build-nocheck
# binary: ./bin/iec61850ctl
```

Or:

```sh
go install github.com/otfabric/iec61850ctl@latest
```

### Cross-platform builds

```sh
make build-all
```

Produces linux/darwin/windows binaries under `release/` for amd64/arm64 (and linux arm/v7). See `Makefile`.

## Quick Start

```sh
# List logical devices
iec61850ctl list lds --host 192.0.2.10

# List LNs in a device
iec61850ctl list lns --host 192.0.2.10 --ld MNSREF615LD0

# Read a measurement (FC MX)
iec61850ctl get object --host 192.0.2.10 \
  --object MNSREF615LD0/FMMXU1.Hz.mag.f --fc MX

# Walk the model (flat)
iec61850ctl tree --host 192.0.2.10 --flatten --path MNSREF615LD0/FMMXU1

# Subscribe to a buffered report (Ctrl+C to stop)
iec61850ctl subscribe report --host 192.0.2.10 \
  --ld MNSREF615LD0 --ln LLN0 --report rcbMeasFlt01 --type BR \
  --show-values --interrogation

# Parse SCL offline
iec61850ctl scl parse --input device.cid --flatten

# Run a local MMS server from SCL
iec61850ctl server start --scl device.cid --host 0.0.0.0 --port 102
```

Defaults: `--port 102`. Host/port/IED name can also come from `IEC61850_HOST` / `IEC61850_PORT` / `IEC61850_IED_NAME` (flags win).

## Command Reference

```
iec61850ctl
├── discover / scan     Scan subnet or IP range for MMS devices
├── list
│   ├── lds             List logical devices (alias: domains)
│   ├── lns             List logical nodes (--ld)
│   ├── dos             List data objects (--ld --ln)
│   ├── das             List data attributes (--ld --ln --do)
│   ├── dss             List data sets (--ld --ln)
│   ├── reports         List report control blocks
│   ├── journals        List journals (--ld)
│   └── files           List remote files
├── get
│   ├── object          Read an object reference (--object --fc)
│   ├── ds              Get data set details / values
│   ├── report          Get RCB configuration
│   ├── journal         Read journal entries
│   └── file            Download a file
├── tree                Full or scoped model tree (--flatten / --serialize)
├── find
│   ├── path            Find LN/DO paths by pattern
│   └── bulk            Bulk path resolve from a mapping file
├── subscribe
│   └── report          Subscribe to URCB/BRCB (--type RP|BR; BRCB --purge-buf/--entry-id)
├── control
│   ├── inspect         Read ctlModel / controllability
│   └── operate         Atomic select/operate journey (one association)
├── set
│   └── object          Write a scalar attribute (--fc; rejects CO)
├── scl
│   ├── parse           Parse SCL to tree or flat paths
│   └── convert         Convert SCL to CSV
├── server
│   ├── start           Start MMS server from --scl
│   └── generate-config Export libiec61850 .cfg (export-only)
├── http                REST API fronting one IEC 61850 device
├── version             Print version / build metadata
└── completion          Shell completion (bash|zsh|fish|powershell)
```

## Usage Examples

### Browse and read

```sh
export IEC61850_HOST=192.0.2.10

iec61850ctl list lds --detailed --format table
iec61850ctl list lns --ld MNSREF615LD0 --detailed
iec61850ctl list dos --ld MNSREF615LD0 --ln FMMXU1
iec61850ctl list das --ld MNSREF615LD0 --ln FMMXU1 --do Hz --detailed

iec61850ctl get object --object MNSREF615LD0/FMMXU1.Hz.mag.f --fc MX --detailed
iec61850ctl get ds --ld MNSREF615LD0 --ln LLN0 --name MeasFlt --detailed
```

### Reports

```sh
iec61850ctl list reports --ld MNSREF615LD0 --ln LLN0
iec61850ctl list reports --all --detailed
iec61850ctl get report --ld MNSREF615LD0 --ln LLN0 --report rcbMeasFlt01

iec61850ctl subscribe report \
  --ld MNSREF615LD0 --ln LLN0 --report rcbMeasFlt01 --type BR \
  --show-values --interrogation --sync --duration 30s

# BRCB: purge buffer or resume from EntryID (hex) before enable
iec61850ctl subscribe report \
  --ld MNSREF615LD0 --ln LLN0 --report rcbMeasFlt01 --type BR \
  --purge-buf --interrogation --max-reports 1 --format jsonl
```

`--interrogation` triggers GI after enable. `--sync` reads the RCB dataset once for a baseline snapshot. Use `--type RP` for unbuffered (URCB). `--purge-buf` / `--entry-id` / `--resv-tms` apply only to `--type BR`.

### Control and scalar write

```sh
# Inspect ctlModel
iec61850ctl control inspect --object InteropLD/GGIO1.SPCSO2 --format json

# Atomic operate (select when required; one association; optional confirm-ref)
iec61850ctl control operate \
  --object InteropLD/GGIO1.SPCSO2 \
  --value true --type bool --mode auto \
  --confirm-ref 'InteropLD/GGIO1.SPCSO2.stVal[ST]' \
  --format json

# Scalar write (not a control bypass; FC=CO rejected)
iec61850ctl set object \
  --object InteropLD/GGIO1.SetInt1.setVal --fc SP \
  --value 5 --type int --verify --format json
```

Selection cannot survive across CLI processes. Prefer `--mode auto`. Confirmation is off unless `--confirm-ref` is set.

### Files and journals

```sh
iec61850ctl list files --detailed
iec61850ctl get file --name conf.xml.gz --output ./conf.xml.gz --detailed

iec61850ctl list journals --ld MNSREF615LD0
iec61850ctl get journal --domain MNSREF615LD0 --journal LLN0\$EventLog \
  --from 2024-10-30T12:00:00Z --to 2024-10-30T13:00:00Z
```

`--from` / `--to` accept RFC3339, space-separated UTC, or Unix milliseconds. Omit `--to` to read after `--from`.

### Find paths

```sh
iec61850ctl find path --ln MMXU --path Hz
iec61850ctl find path --ln MMXU --path A.phsA --include-das --detailed
```

`--ln` is a regex (e.g. `MMXU` matches `FMMXU1`). `--path` is an exact DO or DO.DA match.

### Tree serialize (for server value seeding)

```sh
iec61850ctl tree --serialize --include all --output device.json
iec61850ctl server start --scl device.cid --values device.json --port 102
```

Details: [docs/SERVER.md](docs/SERVER.md).

### HTTP API

```sh
iec61850ctl http --iec-host 192.0.2.10 --iec-port 102 --listen :8080
# GET /api/logical-devices
# GET /api/logical-nodes?ld=MNSREF615LD0
# GET /api/reports/all
```

## Global Flags and Environment

| Flag | Env | Description |
|------|-----|-------------|
| `--host` | `IEC61850_HOST` | IED address (required for most client commands) |
| `--port` | `IEC61850_PORT` | MMS port (default `102`) |
| `--ied-name` | `IEC61850_IED_NAME` | IED name for dial (domain prefix) and `server start` SCL selection |
| `--debug` | — | Log underlying IEC 61850 / MMS calls |

Flag values override environment variables. `discover` / `scan` uses `--host` as a CIDR or IP range (e.g. `192.0.2.0/24` or `192.0.2.10-20`), not a single client target. Optional `--resolve-mac` needs root to read the ARP cache.

## Output Formats

| Commands | `--format` |
|----------|------------|
| `list lds`, `list lns`, `list dos`, `list das` | `text`, `json`, `csv`, `table`, `yaml` |
| `list dss`, `list reports`, `get object`, `get ds`, `get report` | `text`, `json` |
| `control inspect`, `control operate`, `set object` | `text`, `json` |
| `subscribe report` | `text`, `jsonl` |

Default is `text`. Automation contracts (stdout/stderr, JSON shapes, JSON-on-failure, JSONL events): [docs/AUTOMATION.md](docs/AUTOMATION.md).

## Development

```sh
make check                 # fmt, staticcheck, golangci-lint, vet, test, coverage
make build-nocheck         # fast binary → bin/iec61850ctl
make test
make vet

make e2e                   # black-box e2e (default: libiec61850, iec61850bean, self)
make e2e-self              # self-server smoke only
IEC61850CTL_E2E_STACKS=all make e2e   # all CI directions (adds reverse control/report/reader)
make e2e-verify-fixtures   # check e2e/testdata hashes vs interop.lock.json
make e2e-update-fixtures   # refresh fixtures from MMS_INTEROP_DIR
make e2e-docs              # regenerate docs/interop-examples.md
```

`CGO_ENABLED=0` for builds; race tests enable CGO temporarily. External/reverse e2e needs Docker. See `Makefile` and [docs/INTEROP.md](docs/INTEROP.md).

## Built With

| Library | Role |
|---------|------|
| [go-iec61850](https://github.com/otfabric/go-iec61850) | IEC 61850 client/server, SCL model, reports |
| [go-mms](https://github.com/otfabric/go-mms) | MMS / ISO-on-TCP transport |
| [cobra](https://github.com/spf13/cobra) | CLI |

## Documentation

| Doc | Contents |
|-----|----------|
| [docs/PROTOCOL.md](docs/PROTOCOL.md) | IEC 61850 hierarchy, object refs, FCs |
| [docs/SERVER.md](docs/SERVER.md) | SCL server, readiness JSON, value seeding, `.cfg` export |
| [docs/AUTOMATION.md](docs/AUTOMATION.md) | Machine-facing stdout/stderr and JSON/JSONL contract |
| [docs/INTEROP.md](docs/INTEROP.md) | Client/reverse matrices, fixtures, CI stacks |
| [docs/interop-examples.md](docs/interop-examples.md) | Generated operator examples against the interop fixture |
| [RELEASE.md](RELEASE.md) | Release notes |

## License

This project is licensed under the MIT License. See [LICENSE](./LICENSE).
