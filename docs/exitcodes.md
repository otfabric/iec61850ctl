# Process exit codes

iec61850ctl's exit codes are a script-facing contract. Changes are noted in RELEASE.md.

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Failure (unclassified) |
| 2 | Usage error or invalid input, reference or configuration (including a refused `goose`/`sv publish` without `--yes`, `--iface` or `--live`) |
| 3 | Connection failed or lost |
| 4 | Timeout (connect or request), or `--count` not reached before `--duration` |
| 5 | Not found (object, file, IED, RCB, control block, network interface) |
| 6 | Rejected or access denied (including failed controls and raw-socket permission errors) |
| 7 | Write verification, control confirmation, or GoCB/MSVCB enable read-back mismatch |
| 8 | Service not supported by the server or platform |
| 130 | Interrupted (SIGINT / SIGTERM) |

Defined in `pkg/errs/exit.go`. The contract per command is in [AUTOMATION.md](AUTOMATION.md).
