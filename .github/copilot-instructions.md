# iec61850ctl - AI Coding Agent Instructions

## Project Overview

Command-line tool for IEC 61850 MMS protocol communication (industrial automation/electrical substations). Supports reading data objects, listing logical devices (LDs), logical nodes (LNs), data objects (DOs), report subscription, SCL parsing, and a built-in MMS server. Pure Go — no CGO.

**Repository**: [github.com/otfabric/iec61850ctl](https://github.com/otfabric/iec61850ctl)

## Architecture

- **Cobra-based CLI**: Subcommand structure using `github.com/spf13/cobra`
- **Domain layer**: `pkg/domain/` — IED model types (LD, LN, DO, DA, FC, values, reports, journal)
- **Service layer**: `pkg/service/` — business logic (explorer, reader, reporter, tree serialization)
- **Stack layer**: `pkg/stack/` — protocol adapters
  - `pkg/stack/client/` — MMS client via go-iec61850
  - `pkg/stack/server/` — MMS server from SCL, `.cfg` export
- **SCL parsing**: `pkg/scl/` — local SCL/CID/ICD parsing (offline, no server connection)
- **Formatting**: `pkg/formatter/` — text, JSON, YAML, CSV output
- **HTTP API**: `internal/transport/http/` — optional REST wrapper
- **External dependencies**:
  - [github.com/otfabric/go-iec61850](https://github.com/otfabric/go-iec61850) v1.0.4 — IEC 61850 client/server, SCL, directory discovery
  - [github.com/otfabric/go-mms](https://github.com/otfabric/go-mms) v1.0.4 — MMS encoding and ISO transport
- **Module**: `github.com/otfabric/iec61850ctl` (binary: `iec61850ctl`)
- **Go version**: 1.24+ (see `go.mod`)

## Project Structure

```
cmd/                    Cobra commands (discover, get, list, subscribe, server, scl, tree, ...)
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
main.go                 Entry point
```

## Command Structure

```
iec61850ctl
  ├── discover / scan   Network discovery
  ├── find              Path search and bulk read
  ├── get               object, file, report, ds, journal
  ├── list              lds, lns, dos, das, dss, reports, files, journals
  ├── subscribe report  RCB subscription
  ├── server
  │   ├── start             Start MMS server from SCL (--scl, --ied-name, --values)
  │   └── generate-config   Export libiec61850 .cfg from serialized JSON (external tools only)
  ├── scl               parse, convert (offline SCL)
  ├── tree              Device tree display and --serialize
  └── version
```

## Key Domain Concepts

**Functional Constraints (FC)**: IEC 61850 data categorization (MX, SP, ST, CF, etc.). Defined in `pkg/domain/fc.go`. All 15 FC types supported.

**Hierarchical Data Model**: LD → LN → DO → DA (DAs can nest). Data Sets and Reports are LN-level metadata.

**Object Reference Format**: `LD/LN.DO.attribute` (e.g. `MNSREF615LD0/FMMXU1.Hz.mag.f`)

**Directory discovery**: Uses go-iec61850 client APIs via `pkg/service` explorer (GetLogicalDeviceNames, GetLogicalDeviceDirectory, GetLogicalNodeDirectory, GetDataDirectoryWithFC, etc.).

## Code Patterns

### Subcommand Structure

All subcommands follow this pattern (see `cmd/*.go`):

1. Define command with `&cobra.Command{}` — set `Use`, `Short`, `Long`, `RunE`
2. Add flags in `init()` function
3. Register with parent command
4. Implement `RunE` handler: create connection via stack, call service layer, format output

### Connection Management

Use `pkg/stack/client.NewConnection()` with `ConnectionInput`:

```go
conn, err := client.NewConnection(client.ConnectionInput{
    Host:           host,
    Port:           port,
    ConnectTimeout: 10,
    RequestTimeout: 10,
})
defer conn.Close()
```

Returns `service.IEC61850Connection` — a service-layer adapter over go-iec61850.

### Server (SCL-first)

`server start` loads SCL via `pkg/stack/server.Run()`:

```go
server.Run(server.RunConfig{
    SclPath:    "file.icd",
    IEDName:    "",          // optional; defaults to first IED
    ValuesPath: "device.json", // optional value seeding
    Host:       "0.0.0.0",
    Port:       102,
})
```

`server generate-config` exports libiec61850 `.cfg` from serialized IED JSON — export-only, not used at runtime.

## Error Handling

- **MUST** follow CLAUDE.md ERR-1: wrap with `%w` and context
- **MUST** defer `conn.Close()` after successful connection
- Return errors from `RunE` handlers — Cobra displays them automatically

## Adding New Subcommands

1. Create `cmd/<feature>.go`
2. Define command var with cobra.Command
3. Add flags in `init()`, mark required if needed
4. Register with parent command
5. Implement handler in `internal/app/` or inline in `cmd/`
6. Use `pkg/stack/client.NewConnection()` for MMS client commands
7. Follow existing output format patterns

## Build & Test

```bash
go build -o iec61850ctl .
go test ./...
./iec61850ctl --help
./iec61850ctl list lds --host <server>
./iec61850ctl server start --scl cids/MNSREX615.cid --port 102
```

No CGO required. Cross-compile with `CGO_ENABLED=0`.

## Import Conventions

- Internal: `github.com/otfabric/iec61850ctl/pkg/domain`, `.../pkg/service`, `.../pkg/stack/client`
- External: `github.com/spf13/cobra`, `github.com/otfabric/go-iec61850`, `github.com/otfabric/go-mms`
- No vendor directory — use Go modules
