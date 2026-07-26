# Automation contract

Authoritative machine-facing contract for scripting `iec61850ctl`. Human-oriented text output remains the default for interactive use.

## Streams and exit codes

| Stream | Contents |
|--------|----------|
| **stdout** | Command results only (text, JSON, or JSON Lines) |
| **stderr** | Connection status, warnings, diagnostics, debug |

- Successful commands exit `0`.
- Invalid flags, validation failures, connection failures, and remote errors exit non-zero.
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
| Port | `--port` | `IEC61850_PORT` | flag (if not default `102`) → env → `102` |
| IED name | `--ied-name` | `IEC61850_IED_NAME` | flag → env → empty |

An empty IED name preserves legacy behaviour (no MMS domain prefix stripping).

| Target | `IEC61850_IED_NAME` / `--ied-name` |
|--------|-------------------------------------|
| External mms-interop servers (libiec61850 / Bean) | Set to `InteropIED` so LD names appear without the IED prefix |
| `iec61850ctl server start` (self-server) | **Leave unset** — Go server domains are bare LD names (`InteropLD`); setting an IED name breaks browse/read |

The same `--ied-name` flag selects an IED from SCL for `server start` (server-side SCL selection), independent of the client dial setting above.

## Output formats

| Command family | Supported `--format` |
|----------------|----------------------|
| `list lds`, `list lns`, `list dos`, `list das` | `text`, `json`, `csv`, `table`, `yaml` |
| `list dss`, `list reports`, `get object`, `get ds`, `get report` | `text`, `json` |
| `subscribe report` | `text`, `jsonl` |

Unknown formats exit non-zero (no silent fallback to text) for the restricted families above.

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
[{ "name": "dsInterop" }]
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
    "name": "urcb0101",
    "buffered": false,
    "ref": "InteropLD/LLN0.RP.urcb0101"
  }
]
```

### `get report --format json`

Single envelope (never two consecutive documents). Nested `report` always includes `trigger_options` and `optional_fields`; other fields are omitted when unset (`omitempty`):

```json
{
  "report": {
    "ld": "InteropLD",
    "ln": "LLN0",
    "name": "urcb0101",
    "buffered": false,
    "ref": "InteropLD/LLN0.RP.urcb0101",
    "rpt_id": "interop_urcb01",
    "dat_set": "InteropLD/LLN0.dsInterop",
    "enabled": false,
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
      "data_set_name": true,
      "data_reference": false,
      "buffer_overflow": false,
      "entry_id": false,
      "config_revision": false
    }
  },
  "data_set": null
}
```

With `--detailed`, `data_set` holds the dataset view when available.

## Report subscription JSONL

`subscribe report --format jsonl` emits one JSON object per stdout line. Diagnostics remain on stderr.

Suggested events:

```json
{"event":"baseline","data_set":"…","values":[…]}
{"event":"report","rpt_id":"interop_urcb01","sequence_number":1,"values":[…],"reasons":["gi"]}
{"event":"summary","reports_received":1,"clean_disable":true,"duration_ms":1234}
```

- `--show-values` controls inclusion of `values`.
- The `summary` event is emitted only after subscription cleanup completes.
- Reason tokens include `gi`, `dchg`, `qchg`, `dupd`, `integrity`.

## Client lifecycle

Client commands open a session through a shared helper that:

1. Resolves host/port/IED name.
2. Writes the connection target to stderr.
3. Dials with `DialOptions.IEDName`.
4. Defers bounded teardown: `Close` (≈2s) then `Abort` (≈2s) on failure/timeout.

Automation should treat hang-free exit as part of the contract, especially against IEC61850bean.

## Server readiness JSON

See [SERVER.md](SERVER.md) for `--ready-json` / `--fixture-id`. When enabled, exactly one readiness JSON line is written to **stdout** after bind; the human listening message remains on stderr.
