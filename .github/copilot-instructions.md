# iec61850ctl - AI Coding Agent Instructions

## Project Overview

Command-line tool for IEC 61850 MMS protocol communication (industrial automation/electrical substations). Supports reading data objects, listing logical devices (LDs), logical nodes (LNs), data objects (DOs), report subscription, SCL parsing, and a built-in MMS server. Pure Go — no CGO.

**Repository**: [github.com/otfabric/iec61850ctl](https://github.com/otfabric/iec61850ctl)

Operational policy: see [AGENTS.md](../AGENTS.md) — agents may edit code/docs; humans commit, push, tag, and release.

## Architecture

- **Cobra-based CLI**: Subcommand structure using `github.com/spf13/cobra`
- **Domain layer**: `pkg/domain/` — IED model types (LD, LN, DO, DA, FC, values, reports, journal)
- **Service layer**: `pkg/service/` — business logic (explorer, reader, reporter, tree serialization)
- **Stack layer**: `pkg/stack/` — protocol adapters
  - `pkg/stack/client/` — MMS client via go-iec61850 (`DialOptions.IEDName`)
  - `pkg/stack/server/` — MMS server from SCL, readiness JSON, `.cfg` export, interop control registration
- **SCL parsing**: `pkg/scl/` — local SCL/CID/ICD parsing (offline, no server connection)
- **Formatting**: `pkg/formatter/` — text/JSON/YAML/CSV/table; restricted `text|json` for get/list dss/reports; `jsonl` for subscribe
- **HTTP API**: `internal/transport/http/` — optional REST wrapper
- **Black-box e2e**: `e2e/` (`//go:build e2e`) — treats the built binary as an external process
- **External dependencies**:
  - [github.com/otfabric/go-iec61850](https://github.com/otfabric/go-iec61850) v1.0.5 — IEC 61850 client/server, SCL, directory discovery
  - [github.com/otfabric/go-mms](https://github.com/otfabric/go-mms) v1.0.5 — MMS encoding and ISO transport
- **Module**: `github.com/otfabric/iec61850ctl` (binary: `iec61850ctl`)
- **Go version**: 1.24+ (see `go.mod`)

## Project Structure

```
cmd/                    Cobra commands (discover, get, list, subscribe, server, scl, tree, http, ...)
internal/
  app/                  Command handlers wired to services
  transport/http/       Optional HTTP server
pkg/
  domain/               Core IEC 61850 domain types
  service/              Explorer, reader, reporter, tree serialization
  stack/
    client/             go-iec61850 client connection adapter
    server/             SCL-based server run + libiec61850 .cfg export
  scl/                  Offline SCL parse/convert
  formatter/            Output formatters
  view/                 View models for formatters
  network/              Device discovery scanner
e2e/                    Black-box CLI interoperability tests (build tag e2e)
docs/                   PROTOCOL, SERVER, AUTOMATION, INTEROP, examples
main.go                 Entry point
```

## Command Structure

```
iec61850ctl
  ├── discover / scan   Network discovery
  ├── find              Path search and bulk read
  ├── get               object, file, report, ds, journal
  ├── list              lds, lns, dos, das, dss, reports, files, journals
  ├── subscribe report  RCB subscription (--format text|jsonl)
  ├── server
  │   ├── start             --scl, --ied-name, --values, --ready-json, --fixture-id
  │   └── generate-config   Export libiec61850 .cfg (external tools only)
  ├── scl               parse, convert (offline SCL)
  ├── tree              Device tree display and --serialize
  ├── http              REST API fronting one device
  ├── version
  └── completion
```

## Key Domain Concepts

**Functional Constraints (FC)**: IEC 61850 data categorization (MX, SP, ST, CF, etc.). Defined in `pkg/domain/fc.go`. `AllFCs()` returns **19** FCs (excludes `ALL` / empty).

**Hierarchical Data Model**: LD → LN → DO → DA (DAs can nest). Data Sets and Reports are LN-level metadata.

**Object Reference Format**: `LD/LN.DO.attribute` (e.g. `MNSREF615LD0/FMMXU1.Hz.mag.f`)

**Directory discovery**: Uses go-iec61850 client APIs via `pkg/service` explorer.

**IED name**: `--ied-name` / `IEC61850_IED_NAME` for client domain normalisation. Required against external mms-interop servers (`InteropIED`); **must be unset** when speaking to `iec61850ctl server start`.

## Code Patterns

### Subcommand Structure

All subcommands follow this pattern (see `cmd/*.go`):

1. Define command with `&cobra.Command{}` — set `Use`, `Short`, `Long`, `RunE`
2. Add flags in `init()` function
3. Register with parent command
4. Prefer `openClientSession` / `clientSession` in `cmd/session.go` for MMS client commands (stderr connection status, bounded Close/Abort)

### Connection Management

```go
session, err := openClientSession()
if err != nil {
    return err
}
defer session.Close()
// session.Conn is service.IEC61850Connection
```

`DialOptions.IEDName` is set from `--ied-name` / `IEC61850_IED_NAME`.

### Server (SCL-first)

```go
server.Run(server.RunConfig{
    SclPath:    "file.icd",
    IEDName:    "", // optional; defaults to first IED
    ValuesPath: "device.json",
    Host:       "0.0.0.0",
    Port:       102,
    ReadyJSON:  true,
    FixtureID:  "iec61850-v2",
})
```

Serialized `--values` JSON uses PascalCase (`Meta`, `LogicalDevices`, `Leaves`). Every start also registers InteropLD SPCSO1/2/3 control handlers for reverse e2e.

## Automation & interop docs

- [docs/AUTOMATION.md](../docs/AUTOMATION.md) — stdout/stderr, JSON/JSONL contracts
- [docs/INTEROP.md](../docs/INTEROP.md) — five CI e2e directions; Phase 3A vs 3B
- [docs/SERVER.md](../docs/SERVER.md) — readiness JSON, value seeding

## Error Handling

- **MUST** follow CLAUDE.md ERR-1: wrap with `%w` and context
- Return errors from `RunE` — Cobra prints once to stderr (`SilenceUsage` / `SilenceErrors`)

## Build & Test

```bash
make build-nocheck
make check
make e2e                 # default: libiec61850, iec61850bean, self
make e2e-self
IEC61850CTL_E2E_STACKS=all make e2e
```

No CGO required for builds. E2E needs Docker for external/reverse stacks. Cross-compile with `CGO_ENABLED=0`.

## Import Conventions

- Internal: `github.com/otfabric/iec61850ctl/pkg/domain`, `.../pkg/service`, `.../pkg/stack/client`
- External: `github.com/spf13/cobra`, `github.com/otfabric/go-iec61850`, `github.com/otfabric/go-mms`
- No vendor directory — use Go modules
- `e2e/` must not import `cmd`, `internal`, or `pkg`
