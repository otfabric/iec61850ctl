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
| `1` | Failure (unclassified) |
| `2` | Usage error, invalid flag value, malformed reference/input/config |
| `3` | Connection failed or lost (including a dropped report subscription) |
| `4` | Timeout (`--connect-timeout` / `--timeout`); for `goose`/`sv` `sniff` and `subscribe`, `--count` not reached before `--duration` |
| `5` | Not found (object, file, dataset, IED, RCB, journal, GoCB/SVCB, network interface), including a server answer of MMS *object-non-existent* |
| `6` | Rejected or access denied (including failed select/operate, a refused GoCB/MSVCB write, and raw-socket permission errors) |
| `7` | Write verification (`--verify`), control confirmation (`--confirm-ref`), or `goose gocb set` / `sv svcb set` GoEna/SvEna read-back mismatch |
| `8` | Service not supported by the server, or by the platform (raw Ethernet on Windows) |
| `130` | Interrupted (SIGINT / SIGTERM) |

- `goose publish` and `sv publish` refused by their guard rails (no `--iface`, no `--yes`, or operational frames without `--live`) exit `2` before anything is sent.
- For `control operate` / `set object` with `--format json`, protocol outcomes that reach execution still print one JSON document; non-zero exit is driven by `status` / `write_ok` (see below), not by omitting JSON.
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

Every JSON command emits **exactly one** JSON document on stdout (plus a trailing newline). Status text never appears on stdout.

### `get object --format json`

```json
{
  "object": "InteropLD/MMXU1.TotW.mag.f",
  "fc": "MX",
  "type": "float",
  "value": 1234.5
}
```

`object`, `fc`, and `type` are always present. `value` is a typed JSON scalar or `null`.

### Scalar value encoding (stable)

| MMS kind | JSON |
|----------|------|
| boolean | JSON boolean |
| integer / unsigned | JSON number |
| float | JSON number |
| visible / MMS string | JSON string |
| null | JSON `null` |

### Deferred complex encodings

Timestamps, bit strings, octet strings, arrays, and structures do **not** yet have a stable public JSON encoding. They may appear as interim strings; do not depend on that form in automation until documented here as stable.

### `list dss --format json`

Top-level JSON array. Empty result: `[]`.

```json
[{ "name": "dsInterop" }, { "name": "dsSvGeneric" }]
```

With `--detailed`, entries may include `is_deletable`, `member_count`, and `members`.

### `get ds --format json`

Single object using the dataset view (`name`, `is_deletable`, `member_count`, `members`). `--detailed` populates member values when available.

### `list reports --format json`

Top-level array of report summaries (scoped and `--all` share the same shape):

```json
[
  {
    "ld": "InteropLD",
    "ln": "LLN0",
    "name": "urcb01",
    "buffered": false,
    "ref": "InteropLD/LLN0.RP.urcb01"
  },
  {
    "ld": "InteropLD",
    "ln": "LLN0",
    "name": "brcb01",
    "buffered": true,
    "ref": "InteropLD/LLN0.BR.brcb01"
  }
]
```

### `get report --format json`

Single envelope (never two consecutive documents). Nested `report` always includes `trigger_options` and `optional_fields`; other fields are omitted when unset (`omitempty`):

```json
{
  "report": {
    "name": "urcb01",
    "ld": "InteropLD",
    "ln": "LLN0",
    "buffered": false,
    "ref": "InteropLD/LLN0.RP.urcb01",
    "rpt_id": "interop_urcb01",
    "dat_set": "InteropLD/LLN0$dsInterop",
    "enabled": false,
    "conf_rev": 1,
    "intg_pd_ms": 0,
    "buf_tm_ms": 0,
    "reserved": false,
    "sq_num": 0,
    "trigger_options": {
      "data_change": true,
      "quality_change": false,
      "data_update": false,
      "periodic": false,
      "gi": true
    },
    "optional_fields": {
      "sequence_number": true,
      "time_stamp": true,
      "reason_code": true,
      "data_set_name": false,
      "data_reference": false,
      "buffer_overflow": false,
      "entry_id": false,
      "config_revision": false
    }
  },
  "data_set": null
}
```

With `--detailed`, `data_set` holds the dataset view when available. `dat_set` is the server's MMS form (`LD/LN$name`).

For **buffered** RCBs, the nested `report` may also include BRCB runtime fields when the server exposes them: `purge_buf` (bool), `entry_id_value` (hex string), `resv_tms` (int). These are distinct from `optional_fields.entry_id` (OptFlds bit).

### `control inspect --format json`

```json
{
  "object": "InteropLD/GGIO1.SPCSO2",
  "control_model": {"code": 2, "name": "sbo-with-normal-security"},
  "controllable": true,
  "requires_select": true
}
```

`enhanced_security` is present (`true`) only for the `*-with-enhanced-security` models.

### `control operate --format json`

```json
{
  "object": "InteropLD/GGIO1.SPCSO2",
  "control_model": {"code": 2, "name": "sbo-with-normal-security"},
  "controllable": true,
  "requires_select": true,
  "mode": "sbo",
  "requested_value": true,
  "ctl_num": 1,
  "operations": [
    {"operation": "select", "ok": true, "detail": "InteropLD/GGIO1.SPCSO2"},
    {"operation": "operate", "ok": true}
  ],
  "confirmation": {
    "attempted": true,
    "object": "InteropLD/GGIO1.SPCSO2.stVal[ST]",
    "fc": "ST",
    "value": true,
    "matched": true
  },
  "status": "confirmed"
}
```

Status values: `planned` | `operated` | `confirmed` | `operated-unconfirmed` | `confirmation-mismatch` | `failed`.  
Non-zero exit: `failed`, `operated-unconfirmed`, `confirmation-mismatch`.

Confirmation only via `--confirm-ref` (compact FC form allowed, e.g. `…stVal[ST]`). `--test` / `--no-confirm` → status `operated` (not `operated-unconfirmed`). Best-effort `cleanup` after select-without-complete-operate is **not** a rollback. Optional `last_appl_error` appears only when the control-object reference matches the failed target.

### `set object --format json`

```json
{
  "object": "InteropLD/GGIO1.SetInt1.setVal",
  "fc": "SP",
  "type": "int",
  "requested_value": 5,
  "write_ok": true,
  "verification": {"attempted": true, "matched": true, "value": 5}
}
```

`FC=CO` is rejected client-side before dial (`use control operate`). Types are never inferred: `--type` is required (`bool|int|uint|float|enum|string`).

## Report subscription JSONL

`subscribe report --format jsonl` emits one JSON object per stdout line. Diagnostics remain on stderr.

A BRCB run with `--sync --interrogation --show-values --max-reports 1`:

```json
{"event":"baseline","data_set":"InteropLD/LLN0$dsInterop","values":[false,1]}
{"event":"report","rpt_id":"interop_brcb01","sequence_number":1,"entry_id":"0000000000000001","values":[false,1],"reasons":["gi"]}
{"event":"summary","reports_received":1,"clean_disable":true,"duration_ms":104}
```

- `--show-values` controls inclusion of `values`.
- BRCB `entry_id` is hex when the indication includes EntryID.
- BRCB pre-enable: `--purge-buf`, `--entry-id HEX`, `--resv-tms N` (require `--type BR`); written before `RptEna`.
- The `summary` event is emitted only after subscription cleanup completes.
- Reason tokens include `gi`, `dchg`, `qchg`, `dupd`, `integrity`.

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
