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

Black-box e2e exercises the built CLI against all four independent stacks of [mms-interop](https://github.com/otfabric/mms-interop): [libiec61850](https://libiec61850.com), [IEC61850bean](https://www.beanit.com/iec-61850/), [dscsystems/go-iec61850](https://github.com/dscsystems/go-iec61850) and [iec61850-rust](https://github.com/csp0924/iec61850-rust). It covers their IED servers, their controller/reporter/client adapters against `server start`, and GOOSE in both directions against their publishers and subscribers. See [docs/INTEROP.md](docs/INTEROP.md).

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
- **GOOSE** — list GoCBs from SCL, sniff, supervised subscribe, guarded publish (test frames by default), GoCB read/write over MMS
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
├── discover / scan     Scan subnet (at most /16) or IP range for MMS devices
├── list
│   ├── lds             List logical devices (alias: domains)
│   ├── lns             List logical nodes (--ld)
│   ├── dos             List data objects (--ld --ln)
│   ├── das             List data attributes (--ld --ln --do)
│   ├── dss             List data sets (--ld --ln)
│   ├── reports         List report control blocks
│   ├── journals        List journals (--ld or --domain)
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
│   └── bulk            Bulk path resolve from a mapping file (--mapping --output)
├── subscribe
│   └── report          Subscribe to URCB/BRCB (--type RP|BR; BRCB --purge-buf/--entry-id; --keepalive)
├── control
│   ├── inspect         Read ctlModel / controllability
│   └── operate         Atomic select/operate journey (one association)
├── set
│   └── object          Write a scalar attribute (--fc; rejects CO)
├── scl
│   ├── parse           Parse SCL to tree or flat paths
│   └── convert         Convert SCL to CSV
├── server
│   ├── start           Start MMS server from --scl (--values, --files-dir, --port 0)
│   └── generate-config Export libiec61850 .cfg (export-only)
├── goose
│   ├── list            GOOSE control blocks from SCL (--scl)
│   ├── sniff           Decode GOOSE frames on an interface (--iface)
│   ├── subscribe       Validated, supervised subscription to one GoCB (--iface --scl --gocb)
│   ├── publish         Publish a GoCB (--iface --yes; test frames unless --test=false --live)
│   └── gocb get|set    Read/write a GoCB over MMS (--ld --gocb)
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

`--interrogation` triggers GI after enable. `--keepalive` (default `5s`, `0` disables) re-reads the RCB periodically so a dropped association ends the subscription with exit code `3` instead of waiting silently for reports. `--sync` reads the RCB dataset once for a baseline snapshot. Use `--type RP` for unbuffered (URCB). `--purge-buf` / `--entry-id` / `--resv-tms` apply only to `--type BR`.

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

```sh
# Resolve many semantic properties in one pass (example: docs/examples/mapping.json)
iec61850ctl find bulk --mapping docs/examples/mapping.json --output results.json
```

### Tree serialize (for server value seeding)

```sh
iec61850ctl tree --serialize --include all --output device.json
iec61850ctl server start --scl device.cid --values device.json --port 102
```

Details: [docs/SERVER.md](docs/SERVER.md).

### GOOSE

```sh
iec61850ctl goose list --scl station.scd
iec61850ctl goose sniff --iface eth0 --appid 0x1001 --count 10 --format jsonl
iec61850ctl goose subscribe --iface eth0 --scl station.scd --gocb IED1/LD0/gcb1 --duration 30s

# Publishing injects traffic: explicit --iface and --yes; test=true unless --test=false --live
iec61850ctl goose publish --iface eth0 --scl device.cid --gocb gcb1 --value 0=true --changes 3 --interval 1s --yes

iec61850ctl goose gocb get --host 192.0.2.10 --ld LD0 --gocb gcb1
iec61850ctl goose gocb set --host 192.0.2.10 --ld LD0 --gocb gcb1 --disable
```

Raw sockets need `CAP_NET_RAW` on Linux or access to `/dev/bpf*` on macOS/BSD. Details: [docs/GOOSE.md](docs/GOOSE.md).

### HTTP API

```sh
iec61850ctl http --iec-host 192.0.2.10 --iec-port 102 --listen :8080
# GET /api/logical-devices
# GET /api/logical-nodes?ld=MNSREF615LD0
# GET /api/reports/all
```

Errors are JSON `{"error": "...", "code": "..."}` with a status derived from the error class: `400` invalid input, `404` not found, `403` rejected/denied, `501` unsupported, `504` timeout, `502`/`503` device connection problems, `500` otherwise. The global `--timeout`, `--connect-timeout` and `--retry` flags apply to the device connection.

## Global Flags and Environment

| Flag | Env | Description |
|------|-----|-------------|
| `--host` | `IEC61850_HOST` | IED address (required for most client commands) |
| `--port` | `IEC61850_PORT` | MMS port (default `102`) |
| `--ied-name` | `IEC61850_IED_NAME` | IED name for dial (domain prefix) and `server start` SCL selection |
| `--connect-timeout` | — | Bound on each connect attempt: TCP + COTP + MMS association (default `10s`) |
| `--timeout` | — | Bound on each MMS request (default `10s`) |
| `--retry` | — | Connect retries with exponential backoff and jitter (default `0`) |
| `--retry-max-delay` | — | Cap on the delay between connect retries (default `5s`) |
| `--log-level` | — | Structured logs on stderr: `debug`, `info`, `warn` (default), `error`, `off` |
| `--log-format` | — | `text` (default) or `json` |
| `--debug` | — | Shorthand for `--log-level debug` (ignored when `--log-level` is set) |

Flag values override environment variables. `discover` / `scan` uses `--host` as a CIDR (at most `/16`) or IP range (e.g. `192.0.2.0/24` or `192.0.2.10-20`), not a single client target. Optional `--resolve-mac` needs root to read the ARP cache.

Retries apply only to establishing the connection; invalid input, authentication failures and interrupts are never retried. Logs never go to stdout.

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Failure (unclassified) |
| `2` | Usage error or invalid input / reference / configuration |
| `3` | Connection failed or lost |
| `4` | Timeout (connect or request) |
| `5` | Not found (object, file, IED, RCB) |
| `6` | Rejected or access denied by the server (including failed controls) |
| `7` | Write verification or control confirmation mismatch |
| `8` | Service not supported by the server |
| `130` | Interrupted (SIGINT / SIGTERM) |

## Output Formats

| Commands | `--format` |
|----------|------------|
| `list lds`, `list lns`, `list dos`, `list das` | `text`, `json`, `csv`, `table`, `yaml` |
| `list dss`, `list reports`, `get object`, `get ds`, `get report` | `text`, `json` |
| `control inspect`, `control operate`, `set object` | `text`, `json` |
| `subscribe report`, `goose sniff`, `goose subscribe`, `goose publish` | `text`, `jsonl` |
| `goose list`, `goose gocb get`, `goose gocb set` | `text`, `json` |

Default is `text`. Automation contracts (stdout/stderr, JSON shapes, JSON-on-failure, JSONL events): [docs/AUTOMATION.md](docs/AUTOMATION.md).

## Development

```sh
make check                 # fmt, staticcheck, golangci-lint, vet, race tests, coverage gate
make build-nocheck         # fast binary → bin/iec61850ctl
make test                  # unit tests with -race
make coverage-gate         # every package >= 90% (COVERAGE_MIN=..), cgo-free
make fuzz                  # every fuzz target for FUZZTIME (default 10s)
make vet

make e2e                   # black-box e2e (default: the four client stacks + self)
make e2e-self              # self-server smoke only
IEC61850CTL_E2E_STACKS=all make e2e   # all CI directions (adds reverse control/report/reader and GOOSE)
make e2e-verify-fixtures   # check e2e/testdata hashes vs interop.lock.json
make e2e-update-fixtures   # refresh fixtures from MMS_INTEROP_DIR
make e2e-docs              # regenerate docs/interop-examples.md
```

`CGO_ENABLED=0` for builds and the coverage gate; only `make test` enables CGO, because Go's race detector needs it on Linux (the code itself is cgo-free). The `cmd/` tests drive the CLI in-process against an in-repo MMS server and compare output with golden files under `cmd/testdata/golden` (refresh with `go test ./cmd -update`); `cmd/chaos_test.go` puts a fault-injecting proxy (stalls, resets, slow peers) between CLI and server. External/reverse e2e needs Docker. See `Makefile` and [docs/INTEROP.md](docs/INTEROP.md).

## Built With

| Library | Role |
|---------|------|
| [go-iec61850](https://github.com/otfabric/go-iec61850) | IEC 61850 client/server, SCL model, reports, GOOSE |
| [go-mms](https://github.com/otfabric/go-mms) | MMS / ISO-on-TCP transport |
| [cobra](https://github.com/spf13/cobra) | CLI |

## Documentation

| Doc | Contents |
|-----|----------|
| [docs/PROTOCOL.md](docs/PROTOCOL.md) | IEC 61850 hierarchy, object refs, FCs |
| [docs/SERVER.md](docs/SERVER.md) | SCL server, readiness JSON, value seeding, `.cfg` export |
| [docs/GOOSE.md](docs/GOOSE.md) | GOOSE list/sniff/subscribe/publish, GoCB access, safety rules |
| [docs/AUTOMATION.md](docs/AUTOMATION.md) | Machine-facing stdout/stderr and JSON/JSONL contract |
| [docs/INTEROP.md](docs/INTEROP.md) | Client/reverse matrices, fixtures, CI stacks |
| [docs/interop-examples.md](docs/interop-examples.md) | Generated operator examples against the interop fixture |
| [RELEASE.md](RELEASE.md) | Release notes |

## License

This project is licensed under the MIT License. See [LICENSE](./LICENSE).
