# iec61850ctl Releases

## v0.6.1

**Date:** 2026-10-31
**Previous release:** v0.6.0

### Changed

- Tests, CI and documents only; `docs/json-output.md` added. `make coverage-gate` is now `make coverage-pkg-check`, and `COVERAGE_MIN` is now `COVER_PKG_MIN`.
- Built on go-iec61850 v1.4.3, go-mms v1.2.3, go-cotp v1.1.1, go-tpkt v1.0.2.

---

## v0.6.0

**Date:** 2026-10-30
**Previous release:** v0.5.0

### Dependencies

- `go-iec61850` v1.2.2 → v1.3.0, `golang.org/x/sys` → v0.44.0 (govulncheck). Requires Go 1.25.

---

## v0.5.0

**Date:** 2026-10-30
**Previous release:** v0.4.1

### Summary

Exit codes converge on the organisation table (ADR-0006). **Scripts that test for these codes must change.**

| Case | Old | New |
|---|---|---|
| Unclassified failure | 1 | 10 |
| Invalid reference, configuration or SCL | 2 | 3 |
| Connection failed or lost | 3 | 4 |
| Timeout, `--count` not reached | 4 | 5 |
| Not found | 5 | 11 |
| Authentication failed, service not permitted, raw-socket permission | 6 | 9 |
| Write or enable verification mismatch | 7 | 12 |

Unchanged: 0, 2 for invalid flags and values and refused publishes, 6 for a server's rejection, 8, 130. The HTTP API's statuses and `code` values do not change.

### Added

- `IEC61850CTL_FORMAT` (canonical) and `IEC61850CTL_OUTPUT_FORMAT` set the default of `--format` on commands that have it.

---

## v0.4.1

**Date:** 2026-10-28

### Summary

Interop e2e moves to the renamed [iec61850-interop v1.0.1](https://github.com/otfabric/iec61850-interop/releases/tag/v1.0.1) adapters on [go-iec61850 v1.2.2](https://github.com/otfabric/go-iec61850/releases/tag/v1.2.2) and [go-mms v1.2.2](https://github.com/otfabric/go-mms/releases/tag/v1.2.2). The release also fixes four CLI inconsistencies in exit codes and output, and brings the README and reference docs up to date for MMS, GOOSE, Sampled Values, SCL and the HTTP API.

### Fixes

- **Missing objects exit `5`.** A server answer of MMS *object-non-existent* (the data-access result, or the access-class service error) now exits `5` (not found) instead of `6`. This covers `get object`, `get report` and the other reads of a nonexistent object. Other data-access errors (access denied, temporarily unavailable, …) still exit `6`. The HTTP API returns `404 not_found` for the same errors, where it returned `403 rejected`. `control` on a DO without a `ctlModel` (status-only) still exits `6`, now reported as not controllable.
- **`get report` ref.** `report.ref` in `get report --format json` is now `LD/LN.RP.name` / `LD/LN.BR.name`, the same form as `list reports`. `list reports --all` and `tree` use it too.
- **`goose gocb set` / `sv svcb set` verify enable.** With `--enable` or `--disable`, a GoEna/SvEna that reads back the other value now exits `7` (verification mismatch) instead of `0`. The read-back is still printed. This happens with `server start`, which accepts the write but does not publish.

### HTTP API

- **`POST /api/find/path` body keys are snake_case**: `ln_pattern`, `do_name`, `da_name`, `include_das`, `detailed`. The v0.4.0 PascalCase keys (`LNPattern`, `DOName`, `DAName`, `IncludeDAs`, `Detailed`) are still accepted but deprecated, and will be removed in a later release. When a body has both forms of a key, the snake_case one wins.

### Documentation

- The README describes the whole tool (MMS client and server, GOOSE, Sampled Values, SCL, HTTP API) instead of "IEC 61850 MMS". New sections: platform support and raw-socket permissions, release download with checksum verification, a safety table, and complete exit codes. The root `--help` matches.
- [docs/HTTP.md](docs/HTTP.md) (new): every endpoint with real responses, error codes, and deployment guidance. The API has no authentication or TLS, and `--listen` defaults to all interfaces.
- AUTOMATION, SERVER, PROTOCOL, INTEROP and SECURITY checked against the CLI: GOOSE/SV formats and events, refreshed JSON examples, GoCB/MSVCB behaviour of `server start`, and the GOOSE/SV security notes.

### Interop

- **iec61850-interop v1.0.1.** The adapter repository otfabric/mms-interop is now [otfabric/iec61850-interop](https://github.com/otfabric/iec61850-interop). e2e pins its v1.0.1 images by digest:

  | Stack | Image |
  |-------|-------|
  | libiec61850 | `ghcr.io/otfabric/iec61850-interop-libiec61850@sha256:c90b97fb41905983ad8e3db258aee03621348c53cbc50ac9d44dafb72df601e4` |
  | iec61850bean | `ghcr.io/otfabric/iec61850-interop-iec61850bean@sha256:5545f059848ac19178fa934a5018c815978081af002ca7db6479d4bfd527e864` |
  | dscsystems | `ghcr.io/otfabric/iec61850-interop-dscsystems@sha256:9c9a48eb3608c75e45a359b7c4614eb1cd7c3da9197dfbc18710b87647d7cf67` |
  | iec61850-rust | `ghcr.io/otfabric/iec61850-interop-iec61850-rust@sha256:77dc11a2580f784dcdb9f06f3420c0fe7cfa264e73bc881d6f2fe26bb79f6795` |

  The fixtures are unchanged (`iec61850-v3`, `goose-v1`, `sv-v1`). Local images default to `iec61850-interop-<stack>:local`, and `make e2e-update-fixtures` reads `IEC61850_INTEROP_DIR` (was `MMS_INTEROP_DIR`). The adapter environment variables and command names are unchanged.
- **DPS and BCR values.** `TestRead_ST_DPS1AndTotVAh` asserts `GGIO1.DPS1.stVal` = 2 (on) and `MMTR1.TotVAh.actVal` = 42 on all four stacks, with no exemptions.
- `TestInteropSuiteNames` keeps the old suite name out of everything but the historical release notes, and checks the lock's repository and image names.

### Dependencies

- `github.com/otfabric/go-iec61850` **v1.2.1** → **v1.2.2**
- `github.com/otfabric/go-mms` **v1.2.1** → **v1.2.2**

---

## v0.4.0

**Date:** 2026-10-28

### Summary

Sampled Values support on [go-iec61850 v1.2.1](https://github.com/otfabric/go-iec61850/releases/tag/v1.2.1), interop e2e against the SV adapters of [mms-interop v0.4.0](https://github.com/otfabric/iec61850-interop/releases/tag/v0.4.0), and GOOSE/SV reference pages with real example sessions.

### Highlights

#### Sampled Values (`sv` subtree)

- **`sv list --scl`**: the SampledValueControl blocks of an SCL file with their SMV address, svID, rate, noASDU, OptFlds, profile (`9-2LE` or `generic`) and the sample layout of the dataset. Output as text or JSON.
- **`sv sniff --iface`**: decodes SV ASDUs on an interface without supervision. Filters: `--appid`, `--dst`, `--sv-id`. Samples are decoded with `--layout` (`9-2LE` or a list of leaf encodings) or the layout of `--scl`/`--svcb`, else shown as hex. `--sample-every` thins the output.
- **`sv subscribe --iface --scl --svcb`**: a subscription built from SCL that validates confRev, datSet, smpRate/smpMod and the sample size, and supervises smpCnt per stream (start, gap, duplicate, out-of-order, timeout, restored, synch, confrev, invalid). A statistics table (rate, ASDUs, lost, duplicates, out-of-order, invalid, timeouts, smpCnt/wrap, smpSynch, confRev) prints every `--stats-interval` and at the end; `--sample-every` prints every Nth sample, with 9-2LE channels in amperes and volts. JSON Lines with `--format jsonl`.
- **`sv publish`**: publishes an SVCB from SCL at its configured rate.
  - The guard rails are those of `goose publish`: an explicit `--iface`, `--yes`, and frames with the simulation bit set unless `--simulation=false` is combined with `--live`.
  - `--value` sets sample leaves by index, member reference, `REF#N`, or 9-2LE channel (`IA`…`VN`, `qIA`…`qVN`). Qualities accept names (`invalid+test`), a 32-bit word or 13 binary digits.
  - `--waveform CHANNEL=PEAK[,HZ[,PHASE]]` injects a sine on a 9-2LE channel (peak A or V), computed from the sample index so it is exact under scheduling jitter.
  - `--smp-synch`, `--frequency`, `--src`, `--bare-domain`. The summary reports samples, frames, skipped and late samples, and send errors.
- **`sv svcb get|set`**: reads and writes a multicast SV control block over MMS: SvEna, MsvID, DatSet, SmpRate, OptFlds, SmpMod. SvEna is written last.
- SV transport and validation errors map onto the usual exit codes (`2`, `5`, `6`, `8`).

#### Documentation

- [docs/SV.md](docs/SV.md) (new) and [docs/GOOSE.md](docs/GOOSE.md) (rewritten) document every subcommand and flag with example sessions captured from real runs (`TestCaptureDocExamples`). The README command overview covers `sv`.
- `TestDocsCoverCommands` fails when a goose/sv subcommand or flag is missing from its page or the README. `TestDocExamplesParse` parses every documented command line and every cobra `Example`.

#### Interop

- e2e now pins mms-interop v0.4.0: IED fixture `iec61850-v3` (adds `MSVCB02`), GOOSE fixture `goose-v1`, SV fixture `sv-v1`.
- New CI jobs `e2e / sv-libiec61850`, `sv-dscsystems` and `sv-iec61850-rust`. They cover `sv subscribe`/`sv sniff` against each stack's SV publisher and `sv publish` against each stack's SV subscriber, for both the 9-2LE and the generic stream, with value, header and loss assertions.
- The forward jobs read `MSVCB02` with `sv svcb get` from the libiec61850 and dscsystems IED servers, and assert the failure on the stacks that do not model it.
- Registered limitation: libiec61850's SV subscriber drops ASDUs with a two-octet smpMod, which go-iec61850 sends.

#### Fixed

- go-iec61850 v1.2.1: `get ds` of a dataset that does not exist exits `5` on every stack; the first report of a member without a previous value no longer carries a spurious `quality-change` reason.
- `goose publish` and `sv publish` share one implementation of the publish guard rails.
- Three test flakes (GOOSE initial state, report keep-alive cancellation, parallel golangci-lint runs).

---

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
  - `list lns` exits `5` for an unknown LD even when the server answers with an empty name list (dscsystems).
  - `get report` reports the BRCB read error when a BRCB exists but cannot be read, instead of the "no URCB" error of the RP fallback.
- `server start --files-dir` (read-only MMS file service) and `--port 0` (ephemeral port).

#### Testing

- `cmd/` is driven in-process against the bundled server, with golden output for every format.
- A chaos proxy injects stalls, resets and slow peers, with goroutine-leak checks.
- GOOSE e2e tests run over the go-iec61850 in-memory hub.
- Interop e2e now covers all four [mms-interop v0.3.0](https://github.com/otfabric/iec61850-interop/releases/tag/v0.3.0) stacks, with images pinned by digest. 20 CI jobs in total:
  - **Peers.** libiec61850 and iec61850bean as before, plus [dscsystems/go-iec61850](https://github.com/dscsystems/go-iec61850) and [iec61850-rust](https://github.com/csp0924/iec61850-rust).
  - **Client and reverse journeys.** Run against all four stacks: client, reverse control/report and reverse reader.
  - **GOOSE.** `goose publish`, `subscribe` and `sniff` against the GOOSE adapters of libiec61850, dscsystems and iec61850-rust, in both directions, on a Docker bridge.
  - **Known limitation.** iec61850-rust cannot read a BRCB (`TimeOfEntry` is object-access-unsupported). This is asserted as an expected failure.
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

Control and scalar write CLI, buffered-report purge/resume, reverse reader interoperability, and stronger failure/automation contracts. Requires [mms-interop v0.1.4](https://github.com/otfabric/iec61850-interop/releases/tag/v0.1.4) for the ING `SetInt1` write fixture and digest-pinned CI images.

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

Release provenance: [mms-interop v0.1.4](https://github.com/otfabric/iec61850-interop/releases/tag/v0.1.4) / [actions run](https://github.com/otfabric/iec61850-interop/actions/runs/30224434315).

### Scope notes

- HTTP API remains **read/browse/find only** (no control or write over HTTP).
- Standalone `control select` / cross-process SBO, time-activated controls, CDC-aware auto confirm-ref, and complex MMS writes remain out of scope.
- Complex JSON encodings (timestamps, bit/octet strings, arrays, structures) remain deferred — see [docs/AUTOMATION.md](docs/AUTOMATION.md).

### Dependencies

- `github.com/otfabric/go-iec61850` **v1.0.6**
- `github.com/otfabric/go-mms` **v1.0.5**
- Go **1.24+**
- mms-interop adapters **[v0.1.4](https://github.com/otfabric/iec61850-interop/releases/tag/v0.1.4)**

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
