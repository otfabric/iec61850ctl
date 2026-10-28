# HTTP API

`iec61850ctl http` runs a small JSON API in front of **one** IEC 61850
device. It opens one MMS association at startup and serves browse, dataset,
report, journal and path-search requests over it.

The API is **read-only**. It has no control, write, file transfer, report
subscription, GOOSE or Sampled Values endpoints; use the CLI for those.

```sh
iec61850ctl http --iec-host 192.0.2.10 --listen 127.0.0.1:8080
```

| Flag | Default | Meaning |
|------|---------|---------|
| `--iec-host` | (required) | device address |
| `--iec-port` | `102` | device MMS port |
| `--listen` | `:8080` | HTTP listen address |

The global `--ied-name`, `--connect-timeout`, `--timeout`, `--retry` and
logging flags apply to the device connection. A device that cannot be
reached at startup exits `3`. Ctrl+C drains open requests for up to 5 s,
then closes the association.

## Deployment

The API has **no authentication and no TLS**, and the default `--listen
:8080` binds every interface. Anyone who can reach the port can browse the
device model and read values through it.

- Bind to loopback (`--listen 127.0.0.1:8080`) or a management interface.
- Put a reverse proxy in front for TLS and authentication when it must be
  reachable from other hosts.
- Never expose it on a network you would not expose the device's MMS port on.

Server timeouts: read 15 s, write 15 s, idle 60 s. Each device request is
also bounded by the global `--timeout`.

## Endpoints

Responses are JSON. Every endpoint accepts only the method listed; any
other method returns `405`. A path that is not listed returns Go's plain-text
`404 page not found`.

| Method | Path | Parameters | Returns |
|--------|------|------------|---------|
| GET | `/health` | | `{"status":"ok"}` (does not touch the device) |
| GET | `/api/logical-devices` | | LDs with LN, dataset and RCB counts |
| GET | `/api/logical-devices/names` | | LD names |
| GET | `/api/logical-nodes` | `ld` | LNs of an LD with counts |
| GET | `/api/logical-nodes/names` | `ld` | LN names |
| GET | `/api/data-objects` | `ld`, `ln` | DOs of an LN |
| GET | `/api/data-objects/names` | `ld`, `ln` | DO names |
| GET | `/api/data-attributes` | `ld`, `ln`, `do`, optional `detailed=true` | DAs of a DO, grouped, with FC, type and value |
| GET | `/api/datasets` | `ld`, `ln` | datasets with their members |
| GET | `/api/datasets/names` | `ld`, `ln` | dataset names |
| GET | `/api/datasets/details` | `ld`, `ln`, `name` | one dataset's members |
| GET | `/api/datasets/with-values` | `ld`, `ln`, `name`, optional `read_values=false` | one dataset with member values |
| GET | `/api/reports` | `ld`, `ln` | RCBs of an LN |
| GET | `/api/reports/all` | | every RCB of the device |
| GET | `/api/reports/names` | `ld`, `ln` | RCB names, split into buffered and unbuffered |
| GET | `/api/reports/details` | `ld`, `ln`, `name` | one RCB's configuration |
| GET | `/api/journals` | `ld` | journals of an LD |
| GET | `/api/journals/entries` | `domain`, `journal`, `from`, optional `to` | journal entries; `from`/`to` in Unix milliseconds |
| POST | `/api/find/path` | JSON body | LN/DO path search (as `find path`) |
| POST | `/api/find/bulk` | JSON body | bulk path resolution (as `find bulk`) |

The examples below were run against `iec61850ctl server start --scl
e2e/testdata/interop.icd`.

```console
$ curl -s http://127.0.0.1:8080/api/logical-devices
{"logical_devices":[{"name":"InteropLD","ln_count":4,"ds_count":2,"urcb_count":1,"brcb_count":1}]}

$ curl -s 'http://127.0.0.1:8080/api/reports/names?ld=InteropLD&ln=LLN0'
{"buffered":["brcb01"],"unbuffered":["urcb01"]}

$ curl -s 'http://127.0.0.1:8080/api/datasets/with-values?ld=InteropLD&ln=LLN0&name=dsInterop'
{"name":"dsInterop","is_deletable":false,"member_count":2,"members":[{"index":1,"ref":"InteropLD/GGIO1.SPS1.stVal[ST]","value":"false"},{"index":2,"ref":"InteropLD/LLN0.Mod.stVal[ST]","value":"0"}]}
```

### Path search

`POST /api/find/path` takes the fields of `find path`: `LNPattern` (regex),
`DOName` (exact DO), and optionally `DAName`, `IncludeDAs` and `Detailed`.

```console
$ curl -s -d '{"LNPattern":"GGIO","DOName":"SPCSO1"}' http://127.0.0.1:8080/api/find/path
{"matches":[{"LD":"InteropLD","LN":"GGIO1","DO":"SPCSO1","DataAttributes":null}],"call_count":3}
```

`POST /api/find/bulk` takes the entries of a `find bulk` mapping file under
the key `mappings` (the CLI file uses `iec61850Mappings`):

```console
$ curl -s -d '{"mappings":[{"controlledPropertyId":"sw1","baseLn":"GGIO","DoDaPath":"SPCSO1.stVal"}]}' http://127.0.0.1:8080/api/find/bulk
{"entries":[{"controlledPropertyId":"sw1","paths":["InteropLD/GGIO1.SPCSO1.stVal"]}],"call_count":4}
```

`call_count` is the number of MMS requests the search made.

## Errors

Errors are `{"error": "<message>", "code": "<code>"}`. The status and code
come from the same error classes as the CLI exit codes:

| Status | `code` | Cause | CLI exit code |
|--------|--------|-------|---------------|
| `400` | `invalid_input` | missing or malformed parameter or body | `2` |
| `403` | `rejected` | the device refused the request; some devices also report a nonexistent LD or RCB this way | `6` |
| `404` | `not_found` | object or dataset not found | `5` |
| `405` | `method_not_allowed` | wrong HTTP method | |
| `501` | `unsupported` | service not supported by the device | `8` |
| `502` | `connection_failed` | association lost or device unreachable | `3` |
| `503` | `interrupted` | the server is shutting down | `130` |
| `504` | `timeout` | device request timed out | `4` |
| `500` | `internal` | anything else | `1` |

```console
$ curl -s 'http://127.0.0.1:8080/api/journals/entries?domain=InteropLD'
{"error":"Missing required parameters: domain, journal, from","code":"invalid_input"}
```

The `error` text is for humans and may change; match on `code`.
