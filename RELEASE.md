# iec61850ctl Releases

## v0.3.0

**Date:** 2026-10-28

### Summary

GOOSE support on [go-iec61850 v1.1.0](https://github.com/otfabric/go-iec61850/releases/tag/v1.1.0), plus a quality pass:
- typed errors with stable exit codes
- structured logging
- connect retries and timeouts
- in-process CLI tests with golden output
- chaos and fuzz tests
- a 90% per-package coverage gate

### Highlights

#### GOOSE (`goose` subtree)

- **`goose list --scl`**: the GOOSE control blocks in an SCL file, each with its GSE address (MAC, APPID, VLAN, Min/MaxTime) and dataset members. Output as text or JSON.
- **`goose sniff --iface`**: decodes every GOOSE frame on an interface. Filters: `--appid`, `--dst`, `--gocb-ref`. Output as text or JSONL.
- **`goose subscribe --iface --scl --gocb`**: a subscription built from SCL that validates confRev, datSet, goID and member types, and supervises the stream like LGOS (timeouts, stNum/sqNum gaps, duplicates, restarts). It ends with a per-stream summary. With `--count`, it exits `4` if the count is not reached within `--duration`.
- **`goose publish`**: publishes a GoCB from SCL with `--value` overrides and `--changes`/`--interval` state changes.
  - It needs an explicit `--iface` and `--yes`.
  - Frames carry `test=true` unless `--test=false` is combined with `--live`.
- **`goose gocb get|set`**: reads and writes a GoCB over MMS: GoEna, GoID and DatSet. GoEna is written last.
- **`--bare-domain`** matches publishers whose MMS domain is the bare LD instance (such as go-iec61850 and the bundled server) instead of IED name + LD.
- The raw transports are AF_PACKET on Linux (needs `CAP_NET_RAW`) and BPF on macOS/BSD (needs access to `/dev/bpf*`).

#### Errors and exit codes

- Stable exit codes:

  | Code | Meaning |
  |------|---------|
  | `2` | usage or invalid input |
  | `3` | connection failed or lost |
  | `4` | timeout |
  | `5` | not found |
  | `6` | rejected or denied |
  | `7` | verification or confirmation mismatch |
  | `8` | unsupported |
  | `130` | interrupted |

- GOOSE transport errors map onto the same codes: a missing interface gives `5`, missing permission gives `6`, an unsupported platform gives `8`.
- The HTTP API returns typed status codes and an error `code` field.

#### Global flags

- `--log-level` and `--log-format` (slog, on stderr).
- `--connect-timeout` and `--timeout`.
- `--retry` and `--retry-max-delay`. Retries use go-mms `RetryPolicy`/`Retry` with backoff and jitter; invalid input, authentication failures and interrupts are never retried.

#### Reliability

- `subscribe report --keepalive` (default `5s`) detects a lost association and exits with code `3`.
- Fixed:
  - Journal paging could loop forever.
  - `list das` output order is now stable and names are no longer doubled.
  - Control mode and confirmation failures return the right exit codes.
  - A float32 overflow in control values is now an error.
  - GOOSE `--value` rejects text that is not printable ASCII for VisibleString members.
- `server start --files-dir` (read-only MMS file service) and `--port 0` (ephemeral port).

#### Testing

- `cmd/` is driven in-process against the bundled server, with golden output for every format.
- A chaos proxy injects stalls, resets and slow peers, with goroutine-leak checks.
- GOOSE e2e tests run over the go-iec61850 in-memory hub.
- Fuzz targets cover references, SCL, value formatters and parsers, HTTP handlers, and GOOSE APPID/value parsing.
- New make targets: `make coverage-gate` (every package at 90% or more) and `make fuzz`. A CI `quality` job runs both.

### Dependencies

- `github.com/otfabric/go-iec61850` **v1.0.9** → **v1.1.0**
- `github.com/otfabric/go-mms` **v1.1.0** → **v1.2.1**
- `github.com/otfabric/go-cotp` **v1.0.4** → **v1.1.0** (indirect)
- `github.com/otfabric/go-tpkt` → **v1.0.1** (indirect)

### Removed

- The `results.json` and `test_rcb.sh` artefacts. `mapping.json` moved to `docs/examples/`.

---

## v0.2.1

**Date:** 2026-10-28

### Summary

Bump the MMS stack to [go-mms v1.1.0](https://github.com/otfabric/go-mms/releases/tag/v1.1.0)
and [go-iec61850 v1.0.9](https://github.com/otfabric/go-iec61850/releases/tag/v1.0.9).
No CLI flags or output schema change.

### Dependencies

- `github.com/otfabric/go-iec61850` **v1.0.6** → **v1.0.9**
- `github.com/otfabric/go-mms` **v1.0.5** → **v1.1.0**

### Changed

- Seeded `BinaryTime` values outside 1984-01-01…2163-06-05 are clamped
  to that range so the local server can still encode them (go-mms v1.1.0
  rejects Unix 0 and other out-of-range instants).
- `list files --detailed` already omitted a missing `lastModified`; that
  matches the optional field on the v1.1.0 wire.
- Unused BRCB `TimeOfEntry` on the bundled server is 1984-01-01 (from
  go-iec61850 v1.0.9), so a read of an empty buffer no longer fails.

---

## v0.2.0

**Date:** 2026-07-27

### Summary

Control and scalar write CLI, buffered-report purge/resume, reverse reader interoperability, and stronger failure/automation contracts. Requires [mms-interop v0.1.4](https://github.com/otfabric/mms-interop/releases/tag/v0.1.4) for the ING `SetInt1` write fixture and digest-pinned CI images.

### Highlights

#### Control

- **`control inspect` / `control operate`** — ctlModel-aware atomic journeys (`auto` / `direct` / `sbo` / `sbow`) on one association; no cross-process select
- **Confirmation** — only via `--confirm-ref`; statuses: `planned`, `operated`, `confirmed`, `operated-unconfirmed`, `confirmation-mismatch`, `failed`
- **JSON-on-failure** — with `--format json`, journeys that reach execution emit exactly one result document even on non-zero exit
- **Cancel cleanup** — best-effort after select-without-complete-operate (not a rollback); `LastApplError` only when the control-object reference matches
- **IEC61850bean SBOw** — forward client → Bean server is a registered expected failure (not a skip)

#### Scalar write

- **`set object`** — explicit scalar MMS write with `--verify`; types never inferred
- **`FC=CO` rejected** — use `control operate` for controllable objects
- **Writable fixture** — `InteropLD/GGIO1.SetInt1.setVal[SP]` (ING CDC)

#### Buffered reports

- `get report` exposes `purge_buf` / `entry_id_value` / `resv_tms` when present
- `subscribe report --type BR` supports `--purge-buf`, `--entry-id`, `--resv-tms` written **before** enable
- Isolated e2e: purge + GI, then resume with EntryID (libiec61850 required; Bean EntryID best-effort)

#### Reverse server interoperability

- External controller/reporter coverage retained (direct, SBO, SBOw; URCB enable → report → disable)
- New reader stacks: `server-reader-libiec61850` / `server-reader-iec61850bean`
- External `*-ied-client` adapters: associate, discovery, ST/MX/CF/DC reads, ST write, dataset, conclude
- Self-server seeds for `Mod.ctlModel` and `Mod.d`

#### Reliability and testing

- Unit coverage for connection fault after select → Cancel cleanup, concurrent direct operate, short endurance
- E2E negatives: mode mismatch, confirmation mismatch, `FC=CO` reject

### Interop pin

Lock file `e2e/testdata/interop.lock.json`:

| Field | Value |
|-------|--------|
| `source_tag` | `v0.1.4` |
| `source_commit` | `803e783616510396b6a7e38d79eb6c140428032f` |
| libiec61850 image | `ghcr.io/otfabric/mms-interop-libiec61850@sha256:9c29847dbd0523002b113a2fd1806cfb523bdd1610ae2e47f53b6156ec15fde7` |
| iec61850bean image | `ghcr.io/otfabric/mms-interop-iec61850bean@sha256:d95a64e2cb801db24ce9836e9dacc1035bf5131056802e241e9de6546861c611` |

Release provenance: [mms-interop v0.1.4](https://github.com/otfabric/mms-interop/releases/tag/v0.1.4) / [actions run](https://github.com/otfabric/mms-interop/actions/runs/30224434315).

### Scope notes

- HTTP API remains **read/browse/find only** (no control or write over HTTP).
- Standalone `control select` / cross-process SBO, time-activated controls, CDC-aware auto confirm-ref, and complex MMS writes remain out of scope.
- Complex JSON encodings (timestamps, bit/octet strings, arrays, structures) remain deferred — see [docs/AUTOMATION.md](docs/AUTOMATION.md).

### Dependencies

- `github.com/otfabric/go-iec61850` **v1.0.6**
- `github.com/otfabric/go-mms` **v1.0.5**
- Go **1.24+**
- mms-interop adapters **[v0.1.4](https://github.com/otfabric/mms-interop/releases/tag/v0.1.4)**

---

## v0.1.0

**Date:** 2026-07-26

### Summary

Automation-safe CLI contracts, black-box e2e against libiec61850 and IEC61850bean, and reverse controller/reporter interoperability against `server start`.

### Highlights

- **Stdout/stderr split** — command results on stdout; connection status and diagnostics on stderr
- **JSON / JSONL contracts** — browse formats remain broad; get/dataset/report use `text|json`; `subscribe report` supports `jsonl` (see [docs/AUTOMATION.md](docs/AUTOMATION.md))
- **IED name** — `--ied-name` / `IEC61850_IED_NAME` for client dial and server SCL selection
- **Black-box e2e** — client → libIEC / Bean, self-smoke, reverse controller/reporter ([docs/INTEROP.md](docs/INTEROP.md))
- **Fixture/image locking** — `e2e/testdata/interop.lock.json` with hashes and digest-pinned adapter images
- **Structured server readiness** — `--ready-json` / `--fixture-id` for automation and Docker-to-host tests

### Scope notes

- Control/write CLI, reverse reader coverage, and BRCB purge/resume follow in **v0.2.0**.
- Complex JSON encodings (timestamps, bit/octet strings, arrays, structures) remain deferred.

### Dependencies

- `github.com/otfabric/go-iec61850` **v1.0.5**
- `github.com/otfabric/go-mms` **v1.0.5**
- Go **1.24+**

---

## v0.0.1

**Date:** 2026-07-26

### Summary

Initial public OT Fabric release of **iec61850ctl**: a pure-Go IEC 61850 MMS CLI and local SCL-driven server on [go-iec61850](https://github.com/otfabric/go-iec61850) v1.0.5 and [go-mms](https://github.com/otfabric/go-mms) v1.0.5.

### Highlights

- Client browse/read for LDs, LNs, DOs, DAs, datasets, reports, files, and journals
- Report subscribe with GI and optional dataset baseline sync
- `tree` (including `--serialize`) and `find` / `find bulk`
- Offline `scl parse` / `scl convert`
- `server start --scl` with optional `--values` seeding; `generate-config` for external `.cfg` export only
- `http` REST front-end for selected list/get/find operations
- MIT license, GitHub Releases, `CGO_ENABLED=0` cross-builds

### Dependencies

- `github.com/otfabric/go-iec61850` **v1.0.5**
- `github.com/otfabric/go-mms` **v1.0.5** (armv7 / 32-bit build fix)
- Go **1.24+**

### Scope

- GOOSE / Sampled Values publishing and sniffing are **out of scope** for this release.
- Home: [github.com/otfabric/iec61850ctl](https://github.com/otfabric/iec61850ctl)

---
