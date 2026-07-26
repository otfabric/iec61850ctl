# iec61850ctl Releases

## v0.0.1

**Date:** 2026-07-26

### Summary

Initial public OT Fabric release of **iec61850ctl**: a pure-Go IEC 61850 MMS CLI and local SCL-driven server on [go-iec61850](https://github.com/otfabric/go-iec61850) v1.0.4 and [go-mms](https://github.com/otfabric/go-mms) v1.0.4.

### Highlights

- Client browse/read for LDs, LNs, DOs, DAs, datasets, reports, files, and journals
- Report subscribe with GI and optional dataset baseline sync
- `tree` (including `--serialize`) and `find` / `find bulk`
- Offline `scl parse` / `scl convert`
- `server start --scl` with optional `--values` seeding; `generate-config` for external `.cfg` export only
- `http` REST front-end for selected list/get/find operations
- MIT license, GitHub Releases, `CGO_ENABLED=0` cross-builds

### Dependencies

- `github.com/otfabric/go-iec61850` **v1.0.4**
- `github.com/otfabric/go-mms` **v1.0.4**
- Go **1.24+**

### Scope

- GOOSE / Sampled Values publishing and sniffing are **out of scope** for this release.
- Home: [github.com/otfabric/iec61850ctl](https://github.com/otfabric/iec61850ctl)

---
