# GOOSE

`iec61850ctl goose` works with IEC 61850-8-1 GOOSE on raw Ethernet. It uses the
[go-iec61850](https://github.com/otfabric/go-iec61850) `goose` package for the
frame codec, publisher, subscriber and raw transports.

| Command | Network | Purpose |
|---------|---------|---------|
| [`goose list`](#goose-list) | none | Lists GoCBs from SCL: GSEControl, GSE address and dataset |
| [`goose sniff`](#goose-sniff) | receive | Decodes every GOOSE frame, with optional filters |
| [`goose subscribe`](#goose-subscribe) | receive | Validated subscription to one GoCB, with supervision |
| [`goose publish`](#goose-publish) | **send** | Publishes one GoCB (test frames unless `--live`) |
| [`goose gocb get`, `goose gocb set`](#goose-gocb-get--set) | MMS (`--host`) | Reads or writes a GoCB on an IED |

The example sessions on this page are real output. They were captured with
`TestCaptureDocExamples` (`cmd/docs_capture_test.go`) against an in-process
`iec61850ctl` server for the iec61850-interop GOOSE fixture
(`pkg/gooseops/testdata/interop-goose.cid`). That server publishes both GoCBs
on an in-memory hub that stands in for `eth0`. Its references use the bare LD
as MMS domain, which is why the subscribe examples pass `--bare-domain`.

## Interfaces and permissions

| Platform | Transport | Needs |
|----------|-----------|-------|
| Linux | `AF_PACKET`, with a kernel filter on EtherType `0x88B8` | `CAP_NET_RAW`, e.g. `sudo setcap cap_net_raw+ep $(which iec61850ctl)` |
| macOS, *BSD | `/dev/bpf*` | read/write access to `/dev/bpf*` (on macOS, the `access_bpf` group) |
| Windows | unsupported | exit code `8` |

A missing interface exits with code `5`, and a permission error with `6`.

## Control block selection

`--gocb` accepts any of these forms:
- the GSEControl name (`gcb1`)
- `LD/name`
- `IED/LD/name`
- the gocbRef, either on-wire (`IED1LD0/LLN0$GO$gcb1`) or with a bare LD (`LD0/LLN0$GO$gcb1`)

A name that matches several blocks is a usage error (exit `2`).

The SCL binding uses the on-wire references of IEC 61850-8-1, where the MMS
domain is the IED name followed by the LD instance. Some publishers use the
bare LD instance as their domain instead, such as go-iec61850 servers and
`iec61850ctl server start`. For those, pass `--bare-domain` to `subscribe`
(match their references) or `publish` (send that form).

## goose list

Lists every GSEControl block of an SCL file (SCD, CID or ICD) with its GSE
address and dataset. No network access.

| Flag | Default | Meaning |
|------|---------|---------|
| `--scl` | (required) | SCL file |
| `--format` | `text` | `text` or `json` |

```console
$ iec61850ctl goose list --scl pkg/gooseops/testdata/interop-goose.cid
GOCBREF                              APPID   DST                VLAN  CONFREV  GOID              DATSET                                MEMBERS
InteropIEDInteropLD/LLN0$GO$gcbAll   0x1001  01:0c:cd:01:00:01  0/4   1        InteropGooseAll   InteropIEDInteropLD/LLN0$dsGooseAll   8
InteropIEDInteropLD/LLN0$GO$gcbBool  0x1002  01:0c:cd:01:00:02  0/4   1        InteropGooseBool  InteropIEDInteropLD/LLN0$dsGooseBool  2
```

The JSON form adds the GSE timing and the dataset members as `LD/LN.DO.DA[FC]`.
Blocks without a GSE address are listed with `has_address: false`.

```console
$ iec61850ctl goose list --scl pkg/gooseops/testdata/interop-goose.cid --format json
[
  {
    "ied": "InteropIED",
    "ld": "InteropLD",
    "name": "gcbAll",
    "gocb_ref": "InteropIEDInteropLD/LLN0$GO$gcbAll",
    "dat_set": "InteropIEDInteropLD/LLN0$dsGooseAll",
    "go_id": "InteropGooseAll",
    "conf_rev": 1,
    "has_address": true,
    "dst": "01:0c:cd:01:00:01",
    "app_id": "0x1001",
    "vlan_id": 0,
    "vlan_priority": 4,
    "min_time_ms": 10,
    "max_time_ms": 1000,
    "members": [
      "InteropLD/GGIO1.SPS1.stVal[ST]",
      "InteropLD/LLN0.Mod.stVal[ST]",
      "InteropLD/MMTR1.TotVAh.actVal[ST]",
      "InteropLD/MMXU1.TotW.mag.f[MX]",
      "InteropLD/GGIO1.SPS1.q[ST]",
      "InteropLD/GGIO1.SPS1.t[ST]",
      "InteropLD/GGIO1.DPS1.stVal[ST]",
      "InteropLD/MMXU1.TotW[MX]"
    ]
  },
  {
    "ied": "InteropIED",
    "ld": "InteropLD",
    "name": "gcbBool",
    "gocb_ref": "InteropIEDInteropLD/LLN0$GO$gcbBool",
    "dat_set": "InteropIEDInteropLD/LLN0$dsGooseBool",
    "go_id": "InteropGooseBool",
    "conf_rev": 1,
    "has_address": true,
    "dst": "01:0c:cd:01:00:02",
    "app_id": "0x1002",
    "vlan_id": 0,
    "vlan_priority": 4,
    "min_time_ms": 10,
    "max_time_ms": 1000,
    "members": [
      "InteropLD/GGIO1.SPS1.stVal[ST]",
      "InteropLD/GGIO1.SPS1.q[ST]"
    ]
  }
]
```

## goose sniff

Decodes every GOOSE frame on an interface, without validation or supervision.

| Flag | Default | Meaning |
|------|---------|---------|
| `--iface` | (required) | interface to capture on |
| `--appid` | all | only these APPIDs (hex as in SCL, `d:<n>` for decimal; repeatable or comma-separated) |
| `--dst` | all | only frames to this destination MAC; joins that multicast group instead of going promiscuous |
| `--gocb-ref` | all | only frames with this gocbRef |
| `--count` | `0` (unlimited) | stop after this many matching frames |
| `--duration` | `0` (until Ctrl+C) | stop after this long; with `--count`, exit `4` if the count was not reached |
| `--format` | `text` | `text` or `jsonl` |

Frames that cannot be decoded are reported on stderr and counted in the
summary. Each text record is one header line followed by the dataset values.

```console
$ iec61850ctl goose sniff --iface eth0 --appid 0x1002 --count 3
Sniffing GOOSE on eth0 (Ctrl+C to stop)
12:58:06.362 frame     02:00:00:00:00:01 -> 01:0c:cd:01:00:02 appid=0x1002 vlan=0/4 InteropLD/LLN0$GO$gcbBool st=1 sq=1 ttl=2000ms confRev=1
    [0] false
    [1] 0x0000
12:58:06.383 frame     02:00:00:00:00:01 -> 01:0c:cd:01:00:02 appid=0x1002 vlan=0/4 InteropLD/LLN0$GO$gcbBool st=1 sq=2 ttl=2000ms confRev=1
    [0] false
    [1] 0x0000
12:58:06.424 frame     02:00:00:00:00:01 -> 01:0c:cd:01:00:02 appid=0x1002 vlan=0/4 InteropLD/LLN0$GO$gcbBool st=1 sq=3 ttl=2000ms confRev=1
    [0] false
    [1] 0x0000
Frames: 3 (filtered 3, undecodable 0)
```

In JSON Lines each frame is one object, and the summary is the last line:

```console
$ iec61850ctl goose sniff --iface eth0 --gocb-ref 'InteropLD/LLN0$GO$gcbBool' --count 1 --format jsonl
Sniffing GOOSE on eth0 (Ctrl+C to stop)
{"event":"frame","received":"2026-10-28T12:58:06.50544Z","src":"02:00:00:00:00:01","dst":"01:0c:cd:01:00:02","app_id":"0x1002","vlan_id":0,"vlan_priority":4,"simulation":false,"gocb_ref":"InteropLD/LLN0$GO$gcbBool","dat_set":"InteropLD/LLN0$dsGooseBool","go_id":"InteropGooseBool","t":"2026-10-28T12:58:06.351800025Z","ttl_ms":2000,"st_num":1,"sq_num":4,"test":false,"conf_rev":1,"nds_com":false,"entries":2,"values":[false,{"bits":"0000","len":13}]}
{"event":"summary","frames":1,"filtered":0,"decode_errors":0}
```

## goose subscribe

Subscribes to one GoCB from SCL, with validation and supervision.

| Flag | Default | Meaning |
|------|---------|---------|
| `--iface` | (required) | interface to receive on |
| `--scl` | (required) | SCL file describing the publisher |
| `--gocb` | (required) | control block (see [selection](#control-block-selection)) |
| `--bare-domain` | `false` | expect references with the bare LD as MMS domain |
| `--only-changes` | `false` | print state changes only, not retransmissions |
| `--accept-simulation` | `false` | accept frames with the Edition 2 simulation bit set |
| `--count` | `0` (unlimited) | stop after this many valid messages |
| `--duration` | `0` (until Ctrl+C) | stop after this long; with `--count`, exit `4` if the count was not reached |
| `--format` | `text` | `text` or `jsonl` |

The subscription filters on the GoCB's destination MAC, APPID and gocbRef.

It validates each frame's datSet, goID, confRev and the type of every dataset
member. A frame that fails validation produces an `invalid` event naming the
cause (confRev, datSet, goID or member mismatch, or ndsCom set).

Supervision runs per stream and is modelled on the LGOS logical node. It emits these events:

| Event | Meaning |
|-------|---------|
| `state` / `message` | a valid frame; `state` when stNum advanced |
| `timeout` | no frame within timeAllowedToLive |
| `stnum-gap` / `sqnum-gap` | missed states or retransmissions |
| `duplicate` / `out-of-order` / `restart` | sequence anomalies |
| `supervision` | the stream status changed |

Each run ends with a summary per stream: status, last stNum/sqNum and counters.

```console
$ iec61850ctl goose subscribe --iface eth0 --scl pkg/gooseops/testdata/interop-goose.cid --gocb gcbBool --bare-domain --count 3
Subscribing to InteropLD/LLN0$GO$gcbBool on eth0 (Ctrl+C to stop)
12:58:06.665 supervision 02:00:00:00:00:01 0x1002 InteropLD/LLN0$GO$gcbBool status=ok
12:58:06.665 state     02:00:00:00:00:01 -> 01:0c:cd:01:00:02 appid=0x1002 vlan=0/4 InteropLD/LLN0$GO$gcbBool st=1 sq=5 ttl=2000ms confRev=1
    [0] false
    [1] 0x0000
12:58:06.987 message   02:00:00:00:00:01 -> 01:0c:cd:01:00:02 appid=0x1002 vlan=0/4 InteropLD/LLN0$GO$gcbBool st=1 sq=6 ttl=2000ms confRev=1
    [0] false
    [1] 0x0000
12:58:07.627 message   02:00:00:00:00:01 -> 01:0c:cd:01:00:02 appid=0x1002 vlan=0/4 InteropLD/LLN0$GO$gcbBool st=1 sq=7 ttl=2000ms confRev=1
    [0] false
    [1] 0x0000

Messages: 3
Stream 02:00:00:00:00:01 0x1002 InteropLD/LLN0$GO$gcbBool: status=ok st=1 sq=7 messages=3 states=1 timeouts=0 invalid=0 gaps=0/0 dup=0 ooo=0 restarts=0
```

With `--only-changes` only state changes are printed. In JSON Lines, the
`supervision` and `summary` records carry the full supervision state:

```console
$ iec61850ctl goose subscribe --iface eth0 --scl pkg/gooseops/testdata/interop-goose.cid --gocb gcbBool --bare-domain --only-changes --count 1 --format jsonl
Subscribing to InteropLD/LLN0$GO$gcbBool on eth0 (Ctrl+C to stop)
{"event":"supervision","received":"2026-10-28T12:58:08.629031Z","stream":"02:00:00:00:00:01 0x1002 InteropLD/LLN0$GO$gcbBool","status":"ok","state":{"src":"02:00:00:00:00:01","app_id":"0x1002","gocb_ref":"InteropLD/LLN0$GO$gcbBool","status":"ok","st":true,"sim_st":false,"nds_com":false,"last_st_num":1,"last_sq_num":8,"conf_rev":1,"rx_conf_rev":1,"messages":1,"state_changes":1,"duplicates":0,"out_of_order":0,"st_num_gaps":0,"sq_num_gaps":0,"restarts":0,"timeouts":0,"invalid":0,"dropped_events":0}}
{"event":"state","received":"2026-10-28T12:58:08.629031Z","src":"02:00:00:00:00:01","dst":"01:0c:cd:01:00:02","app_id":"0x1002","vlan_id":0,"vlan_priority":4,"simulation":false,"gocb_ref":"InteropLD/LLN0$GO$gcbBool","dat_set":"InteropLD/LLN0$dsGooseBool","go_id":"InteropGooseBool","t":"2026-10-28T12:58:06.351800025Z","ttl_ms":2000,"st_num":1,"sq_num":8,"test":false,"conf_rev":1,"nds_com":false,"entries":2,"values":[false,{"bits":"0000","len":13}]}
{"event":"summary","messages":1,"streams":[{"src":"02:00:00:00:00:01","app_id":"0x1002","gocb_ref":"InteropLD/LLN0$GO$gcbBool","status":"ok","st":true,"sim_st":false,"nds_com":false,"last_st_num":1,"last_sq_num":8,"conf_rev":1,"rx_conf_rev":1,"messages":1,"state_changes":1,"duplicates":0,"out_of_order":0,"st_num_gaps":0,"sq_num_gaps":0,"restarts":0,"timeouts":0,"invalid":0,"dropped_events":0}]}
```

Without `--bare-domain`, the subscription expects the SCL's on-wire gocbRef
(`InteropIEDInteropLD/...`). That filter matches none of this publisher's
frames, so `--count` is not reached and the command exits `4`:

```console
$ iec61850ctl goose subscribe --iface eth0 --scl pkg/gooseops/testdata/interop-goose.cid --gocb gcbBool --count 1 --duration 2s
Subscribing to InteropIEDInteropLD/LLN0$GO$gcbBool on eth0 (Ctrl+C to stop)

Messages: 0
received 0 of 1 GOOSE message(s) before --duration elapsed
$ echo $?
4
```

## goose publish

Publishes one GoCB from SCL: the initial state, optional further state
changes, then retransmissions.

| Flag | Default | Meaning |
|------|---------|---------|
| `--iface` | (required, never defaulted) | interface to publish on |
| `--scl` | (required) | SCL file with the control block |
| `--gocb` | (required) | control block (see [selection](#control-block-selection)) |
| `--yes` | `false` | confirm that frames may be sent on `--iface` (required) |
| `--test` | `true` | set the test flag in every frame; `--test=false` requires `--live` |
| `--live` | `false` | allow `--test=false` frames, which subscribers act on |
| `--value` | zero values | dataset member as `INDEX=VALUE` or `MEMBER=VALUE` (repeatable) |
| `--simulation` | `false` | set the Edition 2 simulation bit |
| `--src` | interface address | source MAC |
| `--bare-domain` | `false` | send references with the bare LD as MMS domain |
| `--changes` | `0` | further state changes (same values, new stNum) after the initial state |
| `--interval` | `1s` | delay between state changes |
| `--duration` | `0` (until Ctrl+C) | keep retransmitting this long after the last change |
| `--format` | `text` | `text` or `jsonl` |

### Safety

Publishing puts frames on the station bus, so three guard rails apply:

1. `--iface` must be given explicitly. It is never defaulted.
2. `--yes` is required.
3. Frames carry **test=true**. Subscribers that honour the test flag do not act on them.
   - To send operational frames you must combine `--test=false` with `--live`.
   - The command then prints a `WARNING: publishing LIVE GOOSE` line on stderr.

The checks run before the interface is opened, so a refused publish sends nothing:

```console
$ iec61850ctl goose publish --iface eth0 --scl pkg/gooseops/testdata/interop-goose.cid --gocb gcbBool
refusing to publish GOOSE on eth0 without --yes: frames reach every subscriber on that LAN
$ echo $?
2
```

```console
$ iec61850ctl goose publish --iface eth0 --scl pkg/gooseops/testdata/interop-goose.cid --gocb gcbBool --yes --test=false
refusing to publish with test=false without --live: subscribers would act on these frames
$ echo $?
2
```

### Values and state changes

`--value` takes either `INDEX=VALUE` or `MEMBER=VALUE`. Members you do not set
keep the zero value of their type. Each value is parsed as its member's type:

| Member type | Format |
|-------------|--------|
| boolean | `true` / `false` |
| integer, unsigned | numbers |
| float | numbers |
| visible string | printable ASCII |
| bit string | hex, optionally `hex/len` |
| octet string | hex |
| UTC time | `now` or RFC 3339 |

Structures cannot be set from text.

Retransmissions follow IEC 61850-8-1 Annex C: MinTime first, doubling up to
the MaxTime heartbeat. Both times come from the SCL GSE address. The run ends
on Ctrl+C or after `--duration`, and the summary shows the final stNum, sqNum
and the number of frames sent. Here the initial state (stNum 1) is followed
by two state changes:

```console
$ iec61850ctl goose publish --iface eth0 --scl pkg/gooseops/testdata/interop-goose.cid --gocb gcbBool --yes --value 0=true --changes 2 --interval 500ms --duration 2s
Publishing InteropIEDInteropLD/LLN0$GO$gcbBool on eth0 -> 01:0c:cd:01:00:02 appid 0x1002 test=true (Ctrl+C to stop)
Published InteropIEDInteropLD/LLN0$GO$gcbBool (appid 0x1002 -> 01:0c:cd:01:00:02, test=true): stNum=3 sqNum=7 frames=20
```

```sh
# Set a member by reference instead of index
iec61850ctl goose publish --iface eth0 --scl device.cid --gocb gcb1 --yes \
  --value 'LD0/GGIO1.Ind1.q[ST]=0000' --duration 5s
```

### Live frames

```console
$ iec61850ctl goose publish --iface eth0 --scl pkg/gooseops/testdata/interop-goose.cid --gocb gcbBool --yes --test=false --live --duration 1s --format jsonl
WARNING: publishing LIVE GOOSE (test=false) on eth0
Publishing InteropIEDInteropLD/LLN0$GO$gcbBool on eth0 -> 01:0c:cd:01:00:02 appid 0x1002 test=false (Ctrl+C to stop)
{"event":"summary","gocb_ref":"InteropIEDInteropLD/LLN0$GO$gcbBool","app_id":"0x1002","dst":"01:0c:cd:01:00:02","test":false,"st_num":1,"sq_num":6,"sent":7}
```

## goose gocb get / set

`goose gocb get` and `goose gocb set` read and write a GoCB over MMS. They use
the global connection flags (`--host`, `--port`, timeouts), like every other
MMS command.

| Flag | Command | Meaning |
|------|---------|---------|
| `--ld` | both | logical device (required) |
| `--gocb` | both | GoCB name (`gcb1`) or item (`LLN0$GO$gcb1`, `LLN0.GO.gcb1`) (required) |
| `--format` | both | `text` or `json` |
| `--enable` / `--disable` | set | write GoEna |
| `--go-id` | set | new GoID (only while disabled) |
| `--dat-set` | set | new DatSet reference (only while disabled; the server increments ConfRev) |

```console
$ iec61850ctl --host 127.0.0.1 --port 10102 goose gocb get --ld InteropLD --gocb gcbBool
Connecting to 127.0.0.1:10102
GoCB InteropLD/LLN0$GO$gcbBool
  GoEna:      true
  GoID:       InteropGooseBool
  DatSet:     InteropLD/LLN0$dsGooseBool
  ConfRev:    1
  NdsCom:     false
  DstAddress: 01:0c:cd:01:00:02 appid 0x1002 vlan 0 prio 4
  MinTime:    10ms
  MaxTime:    1000ms
  FixedOffs:  false
```

```console
$ iec61850ctl --host 127.0.0.1 --port 10102 goose gocb get --ld InteropLD --gocb gcbBool --format json
Connecting to 127.0.0.1:10102
{
  "reference": "InteropLD/LLN0$GO$gcbBool",
  "go_ena": true,
  "go_id": "InteropGooseBool",
  "dat_set": "InteropLD/LLN0$dsGooseBool",
  "conf_rev": 1,
  "nds_com": false,
  "dst": "01:0c:cd:01:00:02",
  "vlan_priority": 4,
  "vlan_id": 0,
  "app_id": "0x1002",
  "min_time_ms": 10,
  "max_time_ms": 1000,
  "fixed_offs": false
}
```

- **Writable attributes.** Only GoEna, GoID and DatSet can be written.
- **Order.** GoEna is written last. That lets you change GoID or DatSet and enable in one call, but only if the block is already disabled.
- **Disabled-only changes.** Servers accept GoID and DatSet changes only while GoEna is false. A write the server refuses exits with code `6`.
- **After a write.** `set` prints the GoCB as read back from the server. If `--enable` or `--disable` was given and GoEna reads back the other value (the server accepted the write but does not publish), `set` exits with code `7`.

A GoID change on an enabled block is refused:

```console
$ iec61850ctl --host 127.0.0.1 --port 10102 goose gocb set --ld InteropLD --gocb gcbBool --go-id renamed
Connecting to 127.0.0.1:10102
set GoCB InteropLD/LLN0$GO$gcbBool: iec61850: set GoCB InteropLD/LLN0$GO$gcbBool.GoID: mms: data access error: TemporarilyUnavailable
$ echo $?
6
```

Disable the block first, then rename and re-enable it in one call:

```console
$ iec61850ctl --host 127.0.0.1 --port 10102 goose gocb set --ld InteropLD --gocb gcbBool --disable
Connecting to 127.0.0.1:10102
GoCB InteropLD/LLN0$GO$gcbBool
  GoEna:      false
  GoID:       InteropGooseBool
  DatSet:     InteropLD/LLN0$dsGooseBool
  ConfRev:    1
  NdsCom:     false
  DstAddress: 01:0c:cd:01:00:02 appid 0x1002 vlan 0 prio 4
  MinTime:    10ms
  MaxTime:    1000ms
  FixedOffs:  false
```

```console
$ iec61850ctl --host 127.0.0.1 --port 10102 goose gocb set --ld InteropLD --gocb gcbBool --go-id renamed --enable
Connecting to 127.0.0.1:10102
GoCB InteropLD/LLN0$GO$gcbBool
  GoEna:      true
  GoID:       renamed
  DatSet:     InteropLD/LLN0$dsGooseBool
  ConfRev:    1
  NdsCom:     false
  DstAddress: 01:0c:cd:01:00:02 appid 0x1002 vlan 0 prio 4
  MinTime:    10ms
  MaxTime:    1000ms
  FixedOffs:  false
```

## Exit codes

The codes are the same as for the rest of the CLI ([AUTOMATION.md](AUTOMATION.md)).

| Code | GOOSE cases |
|------|-------------|
| `2` | missing `--yes`, `--iface` or `--live`; bad `--value`, APPID or MAC; an SCL that cannot be parsed |
| `4` | `--count` not reached before `--duration` (sniff and subscribe) |
| `5` | interface or control block not found |
| `6` | raw socket permission denied; GoCB write rejected |
| `8` | platform without a raw transport; a connection without GoCB access |

## Limitations

These come from go-iec61850 v1.2:
- No GOOSE security (IEC 62351-6).
- No fixed-offset encoding.
- No GSSE.

Sampled Values have their own command tree: see [SV.md](SV.md).

## Interoperability

The e2e suite runs `goose subscribe`, `goose sniff` and `goose publish`
against the GOOSE publishers and subscribers of libiec61850, dscsystems and
iec61850-rust (iec61850-interop `goose-v1`) on a Docker bridge network. See
[INTEROP.md](INTEROP.md#goose-raw-ethernet).

## Server

`server start` does not publish GOOSE on a network interface. Its GoCB
engine is only used in tests, through the in-memory hub.
