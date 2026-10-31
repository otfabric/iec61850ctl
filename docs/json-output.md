# JSON output

The machine-readable output of `iec61850ctl`. Streams, exit codes and the formats each command accepts are in [AUTOMATION.md](AUTOMATION.md). A breaking change to a field name or shape is called out in RELEASE.md.

## Conventions

- `--format json` prints exactly one indented JSON document and a trailing newline on stdout. `--format jsonl` prints one object per line, each with an `event` field. Status text never appears on stdout.
- `IEC61850CTL_FORMAT` (or the older `IEC61850CTL_OUTPUT_FORMAT`) sets the format when `--format` is not given.
- Field names are snake_case.
- A failure has no JSON form: the message goes to stderr, stdout stays empty and the exit code classifies the cause. Only `control operate` and `set object` print a document on failure (below). This differs from the organisation's `{"error":{...}}` convention. The HTTP API has its own `{"error","code"}` body ([HTTP.md](HTTP.md)).
- `version` has text output only.
- Records carry no `schemaVersion`.

## `list lds --format json`

Array of logical devices.

```json
[
  {
    "name": "InteropLD",
    "ln_count": 4,
    "ds_count": 2,
    "urcb_count": 1,
    "brcb_count": 1
  }
]
```

## `list lns --format json`

Array of logical nodes.

```json
[
  {
    "name": "GGIO1",
    "do_count": 6,
    "ds_count": 0,
    "rcb_count": 0
  }
]
```

## `list dos --format json`

Array of `{ "name", "da_count" }`.

## `list das --format json`

Array of data attributes. `name` is always present; `ref`, `fc`, `type`, `value`, `time`, `quality` and `children` are omitted when empty. `value` is a string here, unlike `get object`.

```json
[
  {
    "name": "ctlModel",
    "fc": "CF",
    "type": "INT",
    "value": "1"
  },
  {
    "name": "d",
    "fc": "DC",
    "type": "STRING"
  }
]
```

## `get object --format json`

```json
{
  "object": "InteropLD/MMXU1.TotW.mag.f",
  "fc": "MX",
  "type": "float",
  "value": 1234.5
}
```

`object`, `fc`, and `type` are always present. `value` is a typed JSON scalar or `null`.

## Scalar value encoding (stable)

| MMS kind | JSON |
|----------|------|
| boolean | JSON boolean |
| integer / unsigned | JSON number |
| float | JSON number |
| visible / MMS string | JSON string |
| null | JSON `null` |

## Deferred complex encodings

Timestamps, bit strings, octet strings, arrays, and structures do **not** yet have a stable public JSON encoding. They may appear as interim strings; do not depend on that form in automation until documented here as stable.

## `list dss --format json`

Top-level JSON array. Empty result: `[]`.

```json
[{ "name": "dsInterop" }, { "name": "dsSvGeneric" }]
```

With `--detailed`, entries may include `is_deletable`, `member_count`, and `members`.

## `get ds --format json`

Single object using the dataset view (`name`, `is_deletable`, `member_count`, `members`). `--detailed` populates member values when available.

## `list reports --format json`

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

## `get report --format json`

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

## `control inspect --format json`

```json
{
  "object": "InteropLD/GGIO1.SPCSO2",
  "control_model": {"code": 2, "name": "sbo-with-normal-security"},
  "controllable": true,
  "requires_select": true
}
```

`enhanced_security` is present (`true`) only for the `*-with-enhanced-security` models.

## `control operate --format json`

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

## `set object --format json`

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

## `subscribe report --format jsonl`

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

Event names per command are in [AUTOMATION.md](AUTOMATION.md#goose-and-sampled-values); full records, with captured examples, in [GOOSE.md](GOOSE.md) and [SV.md](SV.md).

## Server readiness

`server start --ready-json` writes one JSON line to stdout after bind ([SERVER.md](SERVER.md)).
