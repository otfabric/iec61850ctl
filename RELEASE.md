# iec61850ctl Releases

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
