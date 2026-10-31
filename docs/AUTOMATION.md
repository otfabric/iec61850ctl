# Automation contract

Authoritative machine-facing contract for scripting `iec61850ctl`: the MMS client commands, `goose`, `sv`, `server start` and `http`. Human-oriented text output remains the default for interactive use.

## Streams and exit codes

| Stream | Contents |
|--------|----------|
| **stdout** | Command results only (text, JSON, or JSON Lines) |
| **stderr** | Connection status, warnings, diagnostics, structured logs (`--log-level`, `--log-format text|json`) |

- Successful commands exit `0`.
- Failures exit with a code that classifies the cause (stable contract):

| Code | Meaning |
|------|---------|
| `0` | Success |
| `2` | Usage error: invalid flag, argument or value, including a refused `goose`/`sv publish` without `--yes`, `--iface` or `--live` |
| `3` | Invalid reference, configuration or SCL |
| `4` | Connection failed or lost (including a dropped report subscription) |
| `5` | Timeout (connect or request), or `--count` not reached before `--duration` |
| `6` | Rejected by the server (including failed select/operate, an MMS access-denied result and a refused GoCB/MSVCB write) |
| `8` | Service not supported by the server or platform (e.g. raw Ethernet on Windows) |
| `9` | Permission: authentication failed, a service the server refuses as not permitted, or a raw-socket permission error |
| `10` | Failure (unclassified) |
| `11` | Not found (object, file, dataset, IED, RCB, journal, control block, network interface), including a server answer of MMS *object-non-existent* |
| `12` | Write verification (`--verify`), control confirmation (`--confirm-ref`), or GoCB/MSVCB enable read-back mismatch |
| `130` | Interrupted (SIGINT / SIGTERM) |

- `goose publish` and `sv publish` refused by their guard rails (no `--iface`, no `--yes`, or operational frames without `--live`) exit `2` before anything is sent.
- For `control operate` / `set object` with `--format json`, protocol outcomes that reach execution still print one JSON document; non-zero exit is driven by `status` / `write_ok` ([json-output.md](json-output.md)), not by omitting JSON.
- Runtime errors are printed once to stderr (usage is not appended).
- `--help` still prints help to stdout and exits `0`.

Connection status looks like:

```text
Connecting to 127.0.0.1:1102
```

and is always written to **stderr**.

## Flag and environment precedence

| Setting | Flag | Environment | Precedence |
|---------|------|-------------|------------|
| Host | `--host` | `IEC61850_HOST` | flag → env → error if missing |
| Port | `--port` | `IEC61850_PORT` | flag (when given, even `102`) → env → `102`; an invalid env value is a usage error |
| IED name | `--ied-name` | `IEC61850_IED_NAME` | flag → env → empty |

An empty IED name preserves legacy behaviour (no MMS domain prefix stripping).

| Target | `IEC61850_IED_NAME` / `--ied-name` |
|--------|-------------------------------------|
| External iec61850-interop servers (libiec61850, IEC61850bean, dscsystems, iec61850-rust) | Set to `InteropIED` so LD names appear without the IED prefix |
| `iec61850ctl server start` (self-server) | **Leave unset** — Go server domains are bare LD names (`InteropLD`); setting an IED name breaks browse/read |

The same `--ied-name` flag selects an IED from SCL for `server start` (server-side SCL selection), independent of the client dial setting above.

## Output formats

| Command family | Supported `--format` |
|----------------|----------------------|
| `list lds`, `list lns`, `list dos`, `list das` | `text`, `json`, `csv`, `table`, `yaml` |
| `list dss`, `list reports`, `get object`, `get ds`, `get report` | `text`, `json` |
| `control inspect`, `control operate`, `set object` | `text`, `json` |
| `subscribe report` | `text`, `jsonl` |
| `goose list`, `goose gocb get`, `goose gocb set`, `sv list`, `sv svcb get`, `sv svcb set` | `text`, `json` |
| `goose sniff`, `goose subscribe`, `goose publish`, `sv sniff`, `sv subscribe`, `sv publish` | `text`, `jsonl` |

Unknown formats exit `2` (no silent fallback to text).

### JSON on failure (control / set)

With `--format json`, validation and protocol journeys that reach execution emit **exactly one** JSON document on stdout even when exiting non-zero. Flag parsing / malformed-input errors may emit no JSON. Diagnostics remain on stderr.

## JSON shapes

The shapes of `--format json` and `--format jsonl` are in [json-output.md](json-output.md).

## GOOSE and Sampled Values

`goose` and `sv` follow the same rules: results on stdout, diagnostics (including undecodable frames and the LIVE warning) on stderr.

- `list`, `gocb get|set` and `svcb get|set` with `--format json` print exactly one JSON document.
- `sniff`, `subscribe` and `publish` with `--format jsonl` print one object per line, each with an `event` field, and end with exactly one `summary` record (also after Ctrl+C or `--duration`).

| Command | `event` values |
|---------|----------------|
| `goose sniff` | `frame`, `summary` |
| `goose subscribe` | `state`, `message`, `timeout`, `stnum-gap`, `sqnum-gap`, `duplicate`, `out-of-order`, `restart`, `invalid`, `supervision`, `summary` |
| `goose publish` | `summary` |
| `sv sniff` | `asdu`, `summary` |
| `sv subscribe` | `asdu` (every `--sample-every`), `start`, `gap`, `duplicate`, `out-of-order`, `timeout`, `restored`, `synch`, `confrev`, `invalid`, `stats` (every `--stats-interval`), `summary` |
| `sv publish` | `summary` |

```json
{"event":"summary","frames":1,"filtered":0,"decode_errors":0}
{"event":"summary","sv_id":"InteropSv92LE","app_id":"0x4000","dst":"01:0c:cd:04:00:00","simulation":true,"rate":4000,"no_asdu":1,"samples":7956,"frames":7956,"skipped":45,"late":97,"send_errors":0,"next_smp_cnt":1}
```

APPIDs are hex strings (`"0x4000"`), MACs are `aa:bb:cc:dd:ee:ff`. The full record shapes, with captured examples, are in [GOOSE.md](GOOSE.md) and [SV.md](SV.md).

## Client lifecycle

Client commands open a session through a shared helper that:

1. Resolves host/port/IED name.
2. Writes the connection target to stderr.
3. Dials with `DialOptions.IEDName`, bounding each attempt by `--connect-timeout` and retrying `--retry` times with exponential backoff and jitter (capped by `--retry-max-delay`). Every request is bounded by `--timeout`.
4. Defers bounded teardown: `Close` (≈2s) then `Abort` (≈2s) on failure/timeout.

Automation should treat hang-free exit as part of the contract, especially against IEC61850bean.

## Server readiness JSON

See [SERVER.md](SERVER.md) for `--ready-json` / `--fixture-id`. When enabled, exactly one readiness JSON line is written to **stdout** after bind; the human listening message remains on stderr.

## HTTP API

`iec61850ctl http` returns JSON bodies and `{"error","code"}` errors whose `code` follows the same classes as the exit codes. See [HTTP.md](HTTP.md).
