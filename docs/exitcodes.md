# Process exit codes

iec61850ctl's exit codes are a script-facing contract, defined in `pkg/errs/exit.go`. They follow the organisation table (otfabric/engineering `standards/api-design.md#cli-exit-codes`); 11, 12 and 13 are this CLI's protocol-specific codes. The contract per command is in [AUTOMATION.md](AUTOMATION.md).

| Code | Meaning |
|------|---------|
| `0` | Success |
| `2` | Usage error: invalid flag, argument or value, a `goose`/`sv publish` without `--iface`, and a missing confirmation: `control operate` (unless `--dry-run`) or `set object` without `--yes`, refused before the device is contacted |
| `3` | Invalid reference, configuration or SCL |
| `4` | Connection failed or lost (including a dropped report subscription) |
| `5` | Timeout (connect or request), or `--count` not reached before `--duration` |
| `6` | Rejected by the server (including failed select/operate, an MMS access-denied result and a refused GoCB/MSVCB write) |
| `8` | Service not supported by the server or platform (e.g. raw Ethernet on Windows) |
| `9` | Permission: authentication failed, a service the server refuses as not permitted, or a raw-socket permission error |
| `10` | Failure (unclassified) |
| `11` | Not found (object, file, dataset, IED, RCB, journal, control block, network interface), including a server answer of MMS *object-non-existent* |
| `12` | Write verification (`--verify`), control confirmation (`--confirm-ref`), or GoCB/MSVCB enable read-back mismatch |
| `13` | Safety refusal: `goose publish` / `sv publish` without `--yes`, or with the test marker off (`--test=false` / `--simulation=false`) without `--live`; nothing is sent |
| `130` | Interrupted (SIGINT / SIGTERM) |

## Changes in v0.10.0

| Case | Old | New |
|---|---|---|
| `goose publish` / `sv publish` without `--yes` | 2 | 13 |
| `goose publish` / `sv publish` with the test marker off and no `--live` | 2 | 13 |
| `control operate` without `--yes` (not `--dry-run`) | runs | 2 |
| `set object` without `--yes` | runs | 2 |

## Mapping from versions before v0.5.0

| Case | Old | New |
|---|---|---|
| Unclassified failure | 1 | 10 |
| Invalid flag, argument or value | 2 | 2 |
| Invalid reference, configuration or SCL | 2 | 3 |
| Connection failed or lost | 3 | 4 |
| Timeout | 4 | 5 |
| Not found | 5 | 11 |
| Rejected by the server | 6 | 6 |
| Authentication failed, service not permitted, raw-socket permission | 6 | 9 |
| Write or enable verification mismatch | 7 | 12 |
| Unsupported | 8 | 8 |
| Interrupted | 130 | 130 |
