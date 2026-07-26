# iec61850ctl Releases

## v0.1.0

**Date:** 2026-07-26

### Summary

Automation-safe CLI contracts, black-box e2e against libiec61850 and IEC61850bean, and reverse server interoperability (Phase 3A controller/reporter).

### Highlights

- **Stdout/stderr split** — command results on stdout; connection status and diagnostics on stderr
- **JSON / JSONL contracts** — browse formats remain broad; get/dataset/report use `text|json`; `subscribe report` supports `jsonl` (see [docs/AUTOMATION.md](docs/AUTOMATION.md))
- **IED name** — `--ied-name` / `IEC61850_IED_NAME` for client dial and server SCL selection
- **Black-box e2e** — five CI directions: client → libIEC / Bean, self-smoke, reverse libIEC / Bean ([docs/INTEROP.md](docs/INTEROP.md))
- **Fixture/image locking** — `e2e/testdata/interop.lock.json` with hashes and digest-pinned adapter images
- **Structured server readiness** — `--ready-json` / `--fixture-id` for automation and Docker-to-host tests

### Scope notes

- Phase 3A covers control and URCB reverse paths only; general external reader coverage is Phase 3B (pending mms-interop capabilities).
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
