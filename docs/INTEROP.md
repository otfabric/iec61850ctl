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

Journeys include association/browse, ST/MX reads against `expected-values.json`, datasets, RCB discovery/config, URCB GI subscribe (`--format jsonl`), and negative CLI cases.

### Reverse server direction (Phase 3A)

| External adapter | Server | CI job |
|------------------|--------|--------|
| libiec61850-ied-controller / -reporter | `iec61850ctl server start` | `e2e / server-libiec61850` |
| iec61850bean-ied-controller / -reporter | `iec61850ctl server start` | `e2e / server-iec61850bean` |

Phase 3A claims:

- external **control** interoperability (direct, SBO, SBOw against SPCSO1/2/3);
- external **URCB** interoperability (enable → write → report → disable → conclude).

Phase 3A does **not** claim general external browsing/reading. That is **Phase 3B**, deferred until mms-interop declares generic reader commands in its capability manifest (for example `*-ied-client`).

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
IEC61850CTL_E2E_STACKS=server-libiec61850 make e2e   # reverse Phase 3A
IEC61850CTL_E2E_STACKS=server-iec61850bean make e2e
make e2e                                               # default: libiec61850, iec61850bean, self
IEC61850CTL_E2E_STACKS=all make e2e                  # all five directions (incl. reverse)
```

CI runs each of the five stacks as a separate matrix job (`fail-fast: false`). Requires Docker for external/reverse stacks. `IEC61850CTL_BIN` is set by the Makefile.

## Known limitations / deferred

- BRCB external subscription is not a CI gate (discovery/config only on the client matrix).
- Complex JSON encodings (timestamps, bit/octet strings, arrays, structures) are deferred; see [AUTOMATION.md](AUTOMATION.md).
- Negative SBO/SBOw sequences wait on adapter modes such as `--skip-select`.
- File/journal reverse coverage and dynamic datasets are out of scope.
- Phase 3B general reader adapters are not claimed until present in mms-interop capabilities.
- IEC61850bean SBOw as a **server** (go-client → bean) remains a known upstream limitation documented in mms-interop; Phase 3A exercises Bean as a **client** against the Go CLI server.
