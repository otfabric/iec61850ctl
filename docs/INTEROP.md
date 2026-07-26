# Interoperability

Black-box CLI interoperability for `iec61850ctl`. This complements the protocol-depth matrix in [go-iec61850/INTEROP.md](https://github.com/otfabric/go-iec61850/blob/main/INTEROP.md).

## Ownership

| Repository | Owns |
|------------|------|
| [mms-interop](https://github.com/otfabric/mms-interop) | Adapter containers, fixtures, readiness/capability contracts |
| [go-iec61850](https://github.com/otfabric/go-iec61850) | Exhaustive Go API interoperability matrix |
| **iec61850ctl** | Operator journeys through the built CLI binary (`e2e/`) |

Assertions live in consumer repos. Adapters and fixtures never embed pass/fail criteria.

The `e2e` package treats `iec61850ctl` as an **external executable**. It does not import `cmd`, `internal`, or `pkg`.

## Matrices

### Client direction (independent interop)

| Client | Server | CI job |
|--------|--------|--------|
| `iec61850ctl` | libiec61850-ied-server | `e2e / libiec61850` |
| `iec61850ctl` | iec61850bean-ied-server | `e2e / iec61850bean` |

Journeys include association/browse, ST/MX reads against `expected-values.json`, datasets, RCB discovery/config, URCB GI subscribe (`--format jsonl`), atomic control operate (SPCSO1/2/3; Bean SBOw = expected failure), scalar write to `GGIO1.SetInt1.setVal[SP]`, BRCB purge/resume, and negative control/write cases.

### Reverse server direction (Phase 3A + 3B)

| External adapter | Server | CI job |
|------------------|--------|--------|
| libiec61850-ied-controller / -reporter | `iec61850ctl server start` | `e2e / server-libiec61850` |
| iec61850bean-ied-controller / -reporter | `iec61850ctl server start` | `e2e / server-iec61850bean` |
| libiec61850-ied-client | `iec61850ctl server start` | `e2e / server-reader-libiec61850` |
| iec61850bean-ied-client | `iec61850ctl server start` | `e2e / server-reader-iec61850bean` |

**Phase 3A** claims:

- external **control** interoperability (direct, SBO, SBOw against SPCSO1/2/3);
- external **URCB** interoperability (enable → write → report → disable → conclude).

**Phase 3B** claims general external browse/read/write/dataset via capability-declared `*-ied-client` adapters:

- association and conclude;
- LD / LN / DO discovery;
- ST, MX, CF, DC reads against seeded values;
- ST write (`Mod.stVal=5`) and dataset read reflecting the write.

Requires mms-interop adapters that declare `libiec61850-ied-client` / `iec61850bean-ied-client` in `fixtures/capabilities.json` (adapter version ≥ `0.1.4` for ING `SetInt1` write fixture). Lock file pins [mms-interop `v0.1.4`](https://github.com/otfabric/mms-interop/releases/tag/v0.1.4) digests; local `:local` images are fine for development.

### Phase 4 — Control and scalar write

| Journey | libiec61850 | iec61850bean |
|---------|-------------|--------------|
| SPCSO1 direct + `--confirm-ref` → `confirmed` | required | required |
| SPCSO2 SBO + confirm | required | required |
| SPCSO3 SBOw + one `ctl_num` + confirm | required | **expected failure** (registered limitation) |
| `set object` `SetInt1.setVal[SP]` + `--verify` | required | required |

Mutating journeys use **isolated** adapter servers (no `t.Parallel()`). Bean SBOw asserts non-zero exit and unchanged `stVal` (not a skip).

### Phase 5 — External BRCB

`get report` exposes BRCB `purge_buf` / `entry_id_value` / `resv_tms` when present. `subscribe report --type BR` supports `--purge-buf`, `--entry-id`, `--resv-tms` written **before** enable. E2E: purge+GI then resume with EntryID (libiec61850 required; Bean EntryID best-effort).

### Phase 6 — Hardening

Unit: connection fault after select → Cancel cleanup; concurrent direct operate; short endurance loop.  
E2E: mode mismatch, confirmation mismatch, `FC=CO` reject. Missing reverse capability commands fail setup (see reverse harness). Full adapter fault-injection modes for mid-sequence disconnect remain available as a follow-up when adapters expose them.

### Self-smoke (not independent interop)

| Client | Server | CI job |
|--------|--------|--------|
| `iec61850ctl` | `iec61850ctl server start` | `e2e / self` |

Useful for release/example confidence. Same stack on both ends — not an independent interoperability claim.

## Fixture and image provenance

Checked in under `e2e/testdata/`:

| File | Role |
|------|------|
| `interop.icd` | SCL model (self-server + documentation) |
| `expected-values.json` | Expected values for **external** adapter assertions (copy of mms-interop `values.json`) |
| `self-server-values.json` | Serialized `domain.IED` seed for `server start --values` |
| `interop.lock.json` | `source_commit`, optional `source_tag`, file hashes, image digests, required commands |
| `examples.json` | Declarative examples → `docs/interop-examples.md` |

Never pass `expected-values.json` to `server start --values`.

```sh
make e2e-verify-fixtures
make e2e-update-fixtures MMS_INTEROP_DIR=../mms-interop
```

Local images default to `mms-interop-*:local`. CI uses digest pins from the lock file.

### Client IED name

| Stack | Client `IEC61850_IED_NAME` |
|-------|----------------------------|
| `libiec61850`, `iec61850bean` | `InteropIED` (required for domain normalisation) |
| `self`, `server-*` (CLI as server) | **unset** for CLI clients talking to the Go server |

## Local execution

```sh
make build-nocheck
make e2e-self                                          # self-smoke only
IEC61850CTL_E2E_STACKS=libiec61850 make e2e          # client → libiec61850
IEC61850CTL_E2E_STACKS=iec61850bean make e2e         # client → bean
IEC61850CTL_E2E_STACKS=server-libiec61850 make e2e          # reverse Phase 3A control/report
IEC61850CTL_E2E_STACKS=server-iec61850bean make e2e
IEC61850CTL_E2E_STACKS=server-reader-libiec61850 make e2e   # reverse Phase 3B reader
IEC61850CTL_E2E_STACKS=server-reader-iec61850bean make e2e
make e2e                                                    # default: libiec61850, iec61850bean, self
IEC61850CTL_E2E_STACKS=all make e2e                       # all CI directions (incl. reverse)
```

CI runs each stack as a separate matrix job (`fail-fast: false`). Requires Docker for external/reverse stacks. `IEC61850CTL_BIN` is set by the Makefile.

## Known limitations / deferred

- IEC61850bean SBOw as a **server** (forward `control operate` on SPCSO3) is a registered expected failure; when Bean gains support, replace the failure assertion (do not leave a silent skip). Phase 3A still exercises Bean as a **client** against the Go CLI server.
- Complex JSON encodings (timestamps, bit/octet strings, arrays, structures) are deferred; see [AUTOMATION.md](AUTOMATION.md).
- Adapter fault-injection modes (e.g. mid-sequence disconnect, `--skip-select`) are not yet exposed; negative coverage today is CLI/unit + mode/confirmation/CO reject e2e.
- File/journal reverse coverage and dynamic datasets are out of scope.
- HTTP mutation (control/write over the REST API) is out of scope.
