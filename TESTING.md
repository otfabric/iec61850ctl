# Testing

How iec61850ctl is tested, and which test covers which part of the lifecycle of its long-running modes. The commands are in the [Makefile](Makefile). CI (`.github/workflows/ci.yml`, the organisation's `go-ci.yml`) runs the tests on Linux and on Windows.

## Running the tests

| Command | What it runs |
|---|---|
| `make test` | every unit and in-process CLI test, with the race detector |
| `make coverage-pkg-check` | the coverage gate: each package, `cmd` included, at 90 % or more |
| `make fuzz` | every fuzz target for `FUZZTIME` (default 10s) |
| `make e2e-self` | the black-box CLI suite against `server start` (`IEC61850CTL_E2E_STACKS=self`) |
| `make e2e` | the same suite against the interop stacks of iec61850-interop |
| `make check` | format, lint, vet, govulncheck, tests, coverage gate and docsync |

The CLI tests in `cmd` run the command tree in-process (`runCLI` in `cmd/harness_test.go`) against a server started from `e2e/testdata/interop.icd` on a loopback port, a fake connection (`withFakeConn`) or an in-memory GOOSE/SV hub (`withHub`, `withSVHub`). No unit or CLI test needs a network beyond loopback or a raw-socket permission.

## Rules the tests follow

- No sleep as synchronisation. A test waits for a condition: a channel, a probe that receives the first frame of a publisher (`cmd/docs_capture_test.go`), a poll with a growing pause and a deadline (`waitFor` in `pkg/stack/server/max_connections_test.go`, `waitGoroutines` in `cmd/chaos_test.go`), or `testing/synctest` for timers (`TestRetryDoContext`).
- Goroutine leaks are found by stack, not by count: `goroutineStacks` records the goroutines before a test, and `waitGoroutines` fails with the stack of each goroutine the test started that still runs after 5 s.
- A bound on an input has an allocation or work budget test next to it: the test reads `runtime.MemStats.TotalAlloc` around the call and fails when the call allocates more than a small multiple of the bound, for an input at the bound and for one far beyond it (see [Budgets](#budgets)).
- A test that reads a document or fixture of the repository as text normalises `"\r\n"` to `"\n"` first (the Windows checkout converts line endings), and a test of a measured duration accepts zero (the Windows monotonic clock can report it).
- A fix of a behaviour comes with a test that fails without the fix.

## Budgets

| Bound (SECURITY.md) | Test |
|---|---|
| SCL document size, type reference depth, expanded nodes | `pkg/scl`: `TestParseSizeLimit`, `TestTypeDepthLimit`, `TestExpansionBudget`, `TestExpansionBudgetAllocation`; `FuzzParse` runs with a lowered node budget |
| `scl parse` / `scl convert` exit code at a bound | `cmd`: `TestSCLExpansionBoundExit` |
| HTTP request body, `ln_pattern` length, bulk mappings | `internal/transport/http`: `TestFindBodyLimit`, `TestFindPatternLimit`, `TestBulkMappingsLimit` |
| `get file` buffered file size | `cmd`: `TestGetFileMemoryBound` |
| Per-request timeout inside a download and a subscription | `cmd`: `TestChaosSilentPeerEachRequest`; `pkg/stack/client`: `TestNewConnectionBoundsEachMMSRequest`; `cmd`: `TestRequestTimeoutPrecedence` |
| `server start --max-connections` | `pkg/stack/server`: `TestMaxConnections`; `cmd`: `TestServerStartMaxConnections` |
| Cleanup after a failed select or operate | `pkg/service`: `TestControllerCleanupAfterContextEnd`, `TestControllerCleanupBounded` |

## Lifecycle matrix

The connection lifecycle of MMS, GOOSE and SV is go-mms's and go-iec61850's, and is tested there. The rows below cover the long-running types iec61850ctl owns: `pkg/stack/server.Running` (`server start`), `internal/transport/http.Server` (`http`), `pkg/service.Reporter` (`subscribe report`), Subscribe and Publish of `pkg/gooseops` and `pkg/svops` (`goose`, `sv`), and `pkg/stack/client.timeoutConn` (`--timeout` on every client command).

| Row | Covered by | Not covered |
|---|---|---|
| L1 Close twice | `Running`: `TestStartServesAndCloses`; `http.Server`: `TestClose` | `Reporter` (its stop is once-guarded) |
| L2 Concurrent Close | — | `Running` (`sync.Once`), `http.Server` |
| L3 Close during a request | `timeoutConn`: `TestTimeoutConnBaseCancellation` (Read, DownloadFile and SubscribeReport end with the base context) | — |
| L4 Handshake, L5 write, L7 late response | go-mms and go-iec61850 | — |
| L6 Cancel during a request | `TestTimeoutConnBaseCancellation`; `cmd`: `TestExitInterrupted`; control cleanup after Ctrl-C: `TestControllerCleanupAfterContextEnd` | — |
| L8 Reconnect | n/a: no reconnect; the dial retry and its cancel: `TestRetryDoContext` | — |
| L9 Callback calls back into the type | — | `Reporter`'s per-report hook |
| L10 Slow callback | — | `Reporter`'s per-report hook |
| L11 Queue full, L12 overflow reported | go-iec61850 (report queue of 64) | no CLI test of a full queue |
| L13 Concurrent requests | — | concurrent HTTP requests on one association |
| L14 Startup failure | `Running`: `TestStartErrors`, `TestStartBadValuesJSON`, `TestStartRefusesValueOutOfRange`; `http.Server`: `TestServeListenerError`, `TestStartListenError`, `TestNewServerConnectionRefused`; `cmd`: `TestSCLInvalidExit` | the release of listener and files on a failed start is not asserted |
| L15 Shutdown | `Running`: `TestStartContextCancelStops`, `cmd` `TestServerStartCommand`; `http.Server`: `TestServeGracefulShutdown`; `Reporter`: `TestReporterRun_KeepAliveCancelled`; GOOSE/SV: `TestSubscribeWaitsForReceiver`, `TestGooseSubscribeAndSniff`, `TestSVSubscribeAndSniff`; no goroutine left: `TestChaos`, `TestChaosSubscribeConnectionLoss`, `TestChaosSilentPeerEachRequest` | `Running` under load |
| L16 Peer gone | `timeoutConn`: `TestTimeoutConnBoundsSilentPeer`, `cmd` `TestChaosSilentPeerEachRequest`; `Reporter`: `TestReporterRun_StreamClosedByPeer`, `TestReporterRun_KeepAliveDetectsLoss`, `cmd` `TestChaosSubscribeConnectionLoss` | — |

## Output

- Output that comes from a device, a frame or a file reaches stdout and stderr escaped: `cmd`: `TestPrintableWriter`, `TestDeviceTextEscaped`, `TestGooseSniffTextEscaped`. The bytes of `get file` and the completion scripts pass unchanged: `TestGetFileBytesUnchanged`, `TestCompletionScriptsUnchanged`.
- Golden files under `cmd/testdata/golden` pin the text and JSON of the commands; `go test ./cmd -run <Test> -update` rewrites them.
- The exit-code table of the root help is compared with `docs/exitcodes.md` and `pkg/errs`: `TestRootHelpExitCodes`.
