# Errors

`iec61850ctl` classifies every failure by wrapping one of the sentinel errors of
`pkg/errs` (with `%w`, or through `errs.Mark`, `errs.Newf`, `errs.Invalidf` and `errs.Safetyf`).
`errs.ExitCode` maps the error chain to the process exit code with
`errors.Is`/`errors.As`; error strings are never inspected. The exit codes are
listed in [docs/exitcodes.md](docs/exitcodes.md); the full API is in
[docs/api/pkg-errs.md](docs/api/pkg-errs.md).

## Sentinels (`pkg/errs`)

| Error | Meaning | Exit code |
|---|---|---|
| `errs.ErrInvalidInput` | Invalid command-line input or a malformed argument | 2 |
| `errs.ErrInvalidReference` | An object reference string is malformed | 3 |
| `errs.ErrInvalidConfig` | A configuration failed validation | 3 |
| `errs.ErrConnectionFailed` | The connection to the server failed or was lost | 4 |
| `errs.ErrTimeout` | An operation timed out waiting for a response | 5 |
| `errs.ErrRejected` | The server rejected an operation (write, control, RCB update) | 6 |
| `errs.ErrUnsupported` | The server, platform or tool does not support the request | 8 |
| `errs.ErrPermissionDenied` | Access denied by the server | 9 |
| `errs.ErrNotFound` | The requested LD, LN, DO, DA, dataset, report, IED or file does not exist | 11 |
| `errs.ErrVerifyMismatch` | A read-back after a write did not match the requested value | 12 |
| `errs.ErrSafety` | A safety guard rail refused the operation (organisation safety code 13) (`goose`/`sv publish` without `--yes`, or unmarked frames without `--live`) | 13 |

The go-iec61850, go-mms, `context`, `io/fs` and `net` errors that `errs.ExitCode`
also recognises map to the same codes (for example `context.DeadlineExceeded` to
5, `context.Canceled` to 130, an MMS *object-non-existent* result to 11, see
`errs.IsObjectNonExistent`). An error that matches none of them exits 10.

## `errs.ExitError`

`errs.ExitError` carries an explicit exit code (`Code`) for an error chain
(`Err`); `errs.WithExitCode` creates one. `errs.ExitCode` returns its `Code`
before any other classification.

## Re-exports (`pkg/service`)

`pkg/service` re-exports seven of the sentinels under the same names, as the
same values, so `errors.Is` matches either form: `service.ErrNotFound`,
`service.ErrConnectionFailed`, `service.ErrInvalidInput`, `service.ErrTimeout`,
`service.ErrPermissionDenied`, `service.ErrInvalidReference` and
`service.ErrInvalidConfig`.
