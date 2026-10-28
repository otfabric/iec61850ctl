# Sampled Values

`iec61850ctl sv` works with IEC 61850-9-2 Sampled Values on raw Ethernet,
including the UCA 9-2LE profile. It uses the
[go-iec61850](https://github.com/otfabric/go-iec61850) `sv` package for the
frame codec, publisher, subscriber and raw transports.

| Command | Network | Purpose |
|---------|---------|---------|
| [`sv list`](#sv-list) | none | Lists SV control blocks from SCL: SampledValueControl, SMV address, dataset layout |
| [`sv sniff`](#sv-sniff) | receive | Decodes SV ASDUs, with optional filters and sample decoding |
| [`sv subscribe`](#sv-subscribe) | receive | Validated subscription to one SVCB, with supervision and live statistics |
| [`sv publish`](#sv-publish) | **send** | Publishes one SVCB, with static values or 9-2LE sine waveforms (simulation frames unless `--live`) |
| [`sv svcb get`, `sv svcb set`](#sv-svcb-get--set) | MMS (`--host`) | Reads or writes a multicast SV control block (MSVCB) on an IED |

The example sessions on this page are real output. They were captured with
`TestCaptureDocExamples` (`cmd/docs_capture_test.go`) against the mms-interop
SV fixture (`pkg/svops/testdata/interop-sv.cid`). It has two streams:

| Control block | svID | APPID | Destination | Dataset | Rate | ASDUs/frame |
|---|---|---|---|---|---|---|
| `MSVCB01` (9-2LE) | `InteropSv92LE` | `0x4000` | `01:0c:cd:04:00:00` | `PhsMeas1`: IA IB IC IN VA VB VC VN, each INT32 + quality | 80/period at 50 Hz = 4000/s | 1 |
| `MSVCB02` (generic) | `InteropSvGeneric` | `0x4001` | `01:0c:cd:04:00:01` | `dsSvGeneric`: FLOAT32, quality, INT32U, quality, INT32 | 1000/s | 4 |

For the receive and MMS examples, an in-process `iec61850ctl` server publishes
both blocks on an in-memory hub that stands in for `eth0`. Its samples are
zero, and its datSet uses the bare LD as MMS domain (see `--bare-domain`).

## Interfaces and permissions

| Platform | Transport | Needs |
|----------|-----------|-------|
| Linux | `AF_PACKET`, with a kernel filter on EtherType `0x88BA` | `CAP_NET_RAW`, e.g. `sudo setcap cap_net_raw+ep $(which iec61850ctl)` |
| macOS, *BSD | `/dev/bpf*` | read/write access to `/dev/bpf*` (on macOS, the `access_bpf` group) |
| Windows | unsupported | exit code `8` |

A missing interface exits with code `5`, and a permission error with `6`.

A 9-2LE stream is 4000 frames per second per merging unit. On a busy capture
host, prefer `subscribe`, which prints statistics and every Nth sample, over
printing every ASDU.

## Control block selection

`--svcb` accepts any of these forms:
- the SampledValueControl name (`MSVCB01`)
- `LD/name`
- `IED/LD/name`
- the reference, either on-wire (`IED1LD0/LLN0$MS$MSVCB01`) or with a bare LD (`LD0/LLN0$MS$MSVCB01`)

A name that matches several blocks is a usage error (exit `2`).

## Samples, layouts and qualities

An ASDU's sample (`seqData`) is the dataset in the IEC 61850-9-2 fixed-length
encoding. Without its layout it is opaque bytes. The layout comes from the SCL
dataset: each member contributes one leaf, or one leaf per attribute for a
structured member (an SAV `Amp` is `INT32,Quality`). `sv list` prints it.

- **9-2LE.** The layout of the UCA 9-2LE dataset is eight INT32 + quality
  pairs: four currents (1 mA per count) and four voltages (10 mV per count).
  `sniff` and `subscribe` show those in amperes and volts. `--value` and
  `--waveform` address the channels as `IA IB IC IN VA VB VC VN`, and their
  qualities as `qIA` … `qVN`.
- **Quality.** A quality value is one of:
  - names joined with `+`: `good`, `invalid`, `questionable`, `test`, `derived`, `old-data`, `substituted`, `failure`, `operator-blocked` (e.g. `invalid+test`);
  - the 32-bit word in hex or decimal: `0x0802`;
  - 13 MSB-first binary digits, as in the MMS JSON and the mms-interop fixtures: `0000000000010` is `test`.

  In JSON output a quality is the MMS bit string `{"bits":"0000","len":13}`.

## sv list

Lists every SampledValueControl block of an SCL file with its SMV address,
rate and dataset layout. No network access.

| Flag | Default | Meaning |
|------|---------|---------|
| `--scl` | (required) | SCL file |
| `--format` | `text` | `text` or `json` |

`PROFILE` is `9-2LE` when the layout is the 9-2LE dataset. `OCTETS` is the
sample size.

```console
$ iec61850ctl sv list --scl pkg/svops/testdata/interop-sv.cid
SVCBREF                              APPID   DST                VLAN  SVID              SMPRATE          NOASDU  CONFREV  PROFILE  DATSET                                OCTETS
InteropIEDInteropLD/LLN0$MS$MSVCB01  0x4000  01:0c:cd:04:00:00  0/4   InteropSv92LE     80 SmpPerPeriod  1       1        9-2LE    InteropIEDInteropLD/LLN0$PhsMeas1     64
InteropIEDInteropLD/LLN0$MS$MSVCB02  0x4001  01:0c:cd:04:00:01  0/4   InteropSvGeneric  1000 SmpPerSec   4       1        generic  InteropIEDInteropLD/LLN0$dsSvGeneric  20
```

The JSON form adds the optional fields (OptFlds), the layout and the members
with their leaf encodings:

```console
$ iec61850ctl sv list --scl pkg/svops/testdata/interop-sv.cid --format json
[
  {
    "ied": "InteropIED",
    "ld": "InteropLD",
    "name": "MSVCB01",
    "svcb_ref": "InteropIEDInteropLD/LLN0$MS$MSVCB01",
    "sv_id": "InteropSv92LE",
    "dat_set": "InteropIEDInteropLD/LLN0$PhsMeas1",
    "conf_rev": 1,
    "smp_rate": 80,
    "smp_mod": "SmpPerPeriod",
    "no_asdu": 1,
    "opt_flds": [
      "sample-synchronized"
    ],
    "has_address": true,
    "dst": "01:0c:cd:04:00:00",
    "app_id": "0x4000",
    "vlan_id": 0,
    "vlan_priority": 4,
    "profile": "9-2LE",
    "layout": "INT32,Quality,INT32,Quality,INT32,Quality,INT32,Quality,INT32,Quality,INT32,Quality,INT32,Quality,INT32,Quality",
    "sample_octets": 64,
    "members": [
      "InteropLD/TCTR1.Amp[MX] INT32,Quality",
      "InteropLD/TCTR2.Amp[MX] INT32,Quality",
      "InteropLD/TCTR3.Amp[MX] INT32,Quality",
      "InteropLD/TCTR4.Amp[MX] INT32,Quality",
      "InteropLD/TVTR1.Vol[MX] INT32,Quality",
      "InteropLD/TVTR2.Vol[MX] INT32,Quality",
      "InteropLD/TVTR3.Vol[MX] INT32,Quality",
      "InteropLD/TVTR4.Vol[MX] INT32,Quality"
    ]
  },
  {
    "ied": "InteropIED",
    "ld": "InteropLD",
    "name": "MSVCB02",
    "svcb_ref": "InteropIEDInteropLD/LLN0$MS$MSVCB02",
    "sv_id": "InteropSvGeneric",
    "dat_set": "InteropIEDInteropLD/LLN0$dsSvGeneric",
    "conf_rev": 1,
    "smp_rate": 1000,
    "smp_mod": "SmpPerSec",
    "no_asdu": 4,
    "opt_flds": [
      "refresh-time",
      "sample-synchronized",
      "sample-rate",
      "data-set"
    ],
    "has_address": true,
    "dst": "01:0c:cd:04:00:01",
    "app_id": "0x4001",
    "vlan_id": 0,
    "vlan_priority": 4,
    "profile": "generic",
    "layout": "FLOAT32,Quality,INT32U,Quality,INT32",
    "sample_octets": 20,
    "members": [
      "InteropLD/MMXU1.TotW.mag.f[MX] FLOAT32",
      "InteropLD/MMXU1.TotW.q[MX] Quality",
      "InteropLD/MMTR1.TotVAh.actVal[ST] INT32U",
      "InteropLD/MMTR1.TotVAh.q[ST] Quality",
      "InteropLD/LLN0.Mod.stVal[ST] INT32"
    ]
  }
]
```

## sv sniff

Decodes SV frames on an interface and prints their ASDUs, without validation
or supervision.

| Flag | Default | Meaning |
|------|---------|---------|
| `--iface` | (required) | interface to capture on |
| `--appid` | all | only these APPIDs (hex as in SCL; repeatable or comma-separated) |
| `--dst` | all | only frames to this destination MAC; joins that multicast group instead of going promiscuous |
| `--sv-id` | all | only ASDUs with this svID |
| `--scl`, `--svcb` | none | filter on that control block's address, APPID and svID, and decode its samples |
| `--layout` | none (hex) | decode samples with this layout: `9-2LE`, or leaf encodings such as `FLOAT32,Quality,INT32U` |
| `--sample-every` | `1` | print every Nth matching ASDU |
| `--count` | `0` (unlimited) | stop after this many matching ASDUs |
| `--duration` | `0` (until Ctrl+C) | stop after this long; with `--count`, exit `4` if the count was not reached |
| `--format` | `text` | `text` or `jsonl` |

`--appid`, `--dst` and `--sv-id` override the filter that `--svcb` sets, and
`--layout` overrides its layout. Without a layout, samples are shown as hex:

```console
$ iec61850ctl sv sniff --iface eth0 --count 3
Sniffing SV on eth0 (Ctrl+C to stop)
12:58:14.681 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=20 confRev=1 synch=0 asdu=1/1
    sample 00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000
12:58:14.682 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=21 confRev=1 synch=0 asdu=1/1
    sample 00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000
12:58:14.682 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=22 confRev=1 synch=0 asdu=1/1
    sample 00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000
ASDUs: 3 in 3 frame(s), printed 3 (filtered 0 frame(s), undecodable 0)
```

With `--scl` and `--svcb`, the 9-2LE samples are decoded into amperes and
volts. `--sample-every 1000` prints one ASDU per quarter second of a
4000/s stream:

```console
$ iec61850ctl sv sniff --iface eth0 --scl pkg/svops/testdata/interop-sv.cid --svcb MSVCB01 --sample-every 1000 --count 4000
Sniffing SV on eth0 (Ctrl+C to stop)
12:58:14.683 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=28 confRev=1 synch=0 asdu=1/1
    I [A]:     0.000     0.000     0.000     0.000   V [V]:      0.00      0.00      0.00      0.00  quality=good
12:58:14.934 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=1030 confRev=1 synch=0 asdu=1/1
    I [A]:     0.000     0.000     0.000     0.000   V [V]:      0.00      0.00      0.00      0.00  quality=good
12:58:15.184 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=2030 confRev=1 synch=0 asdu=1/1
    I [A]:     0.000     0.000     0.000     0.000   V [V]:      0.00      0.00      0.00      0.00  quality=good
12:58:15.434 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=3030 confRev=1 synch=0 asdu=1/1
    I [A]:     0.000     0.000     0.000     0.000   V [V]:      0.00      0.00      0.00      0.00  quality=good
ASDUs: 4000 in 4000 frame(s), printed 4 (filtered 251 frame(s), undecodable 0)
```

A layout without SCL decodes a generic stream. The JSON record carries every
header field, including the optional ones (`dat_set`, `refr_tm`, `smp_rate`,
`smp_mod`) when present, and the values in dataset order:

```console
$ iec61850ctl sv sniff --iface eth0 --appid 0x4001 --layout FLOAT32,Quality,INT32U,Quality,INT32 --count 1 --format jsonl
Sniffing SV on eth0 (Ctrl+C to stop)
{"event":"asdu","received":"2026-10-28T12:58:15.687764Z","src":"02:00:00:00:00:01","dst":"01:0c:cd:04:00:01","app_id":"0x4001","vlan_id":0,"vlan_priority":4,"simulation":false,"no_asdu":4,"asdu_index":0,"sv_id":"InteropSvGeneric","dat_set":"InteropLD/LLN0$dsSvGeneric","smp_cnt":8,"conf_rev":1,"refr_tm":"2026-10-28T12:58:15.68472898Z","smp_synch":0,"smp_rate":1000,"smp_mod":"SmpPerSec","values":[0,{"bits":"0000","len":13},0,{"bits":"0000","len":13},0]}
{"event":"summary","frames":1,"asdus":1,"printed":1,"filtered":12,"decode_errors":0}
```

## sv subscribe

Subscribes to one SVCB from SCL, with validation, supervision and periodic
statistics.

| Flag | Default | Meaning |
|------|---------|---------|
| `--iface` | (required) | interface to receive on |
| `--scl` | (required) | SCL file describing the publisher |
| `--svcb` | (required) | control block (see [selection](#control-block-selection)) |
| `--bare-domain` | `false` | expect datSet with the bare LD as MMS domain |
| `--accept-simulation` | `false` | accept frames with the simulation bit set (dropped otherwise, as by a device not in simulation mode) |
| `--sample-every` | `0` (none) | print every Nth accepted ASDU |
| `--stats-interval` | `1s` | print the statistics table this often (`0` = only the final summary) |
| `--timeout` | `100ms` | supervision timeout: a silent stream reports `timeout` after this long |
| `--count` | `0` (unlimited) | stop after this many accepted ASDUs |
| `--duration` | `0` (until Ctrl+C) | stop after this long; with `--count`, exit `4` if the count was not reached |
| `--format` | `text` | `text` or `jsonl` |

The subscription filters on the SVCB's destination MAC, APPID and svID. It
validates confRev, the datSet and smpRate/smpMod when the ASDU carries them,
and the sample size of the layout. smpCnt is supervised per stream, with the
wrap derived from the rate (4000 for 9-2LE).

These events are printed as they happen:

| Event | Meaning |
|-------|---------|
| `start` | first ASDU of a stream |
| `gap` | smpCnt skipped samples (`lost=N`) |
| `duplicate` / `out-of-order` | repeated or late smpCnt |
| `timeout` / `restored` | the stream went silent for `--timeout`, or came back |
| `synch` | smpSynch changed |
| `confrev` | confRev changed |
| `invalid` | an ASDU failed validation; printed once per distinct error, all counted in `INVALID` |

The statistics table has one row per stream:

| Column | Meaning |
|--------|---------|
| `STATUS` | `ok`, `timeout` or `invalid`; `(sim)` when the last ASDU had the simulation bit |
| `RATE/s` | measured ASDUs per second (over about 0.5 s) |
| `ASDUS` | ASDUs received |
| `LOST`, `DUP`, `OOO` | samples missing from smpCnt gaps, duplicates, out-of-order |
| `INVALID`, `TIMEOUTS` | ASDUs that failed validation; timeout transitions |
| `SMPCNT` | last smpCnt and the wrap in use |
| `SYNCH`, `CONFREV` | smpSynch and confRev of the last accepted ASDU |

A 9-2LE stream with one sample per second and the default statistics
interval:

```console
$ iec61850ctl sv subscribe --iface eth0 --scl pkg/svops/testdata/interop-sv.cid --svcb MSVCB01 --sample-every 4000 --duration 2500ms
Subscribing to InteropIEDInteropLD/LLN0$MS$MSVCB01 (svID InteropSv92LE) on eth0 (Ctrl+C to stop)
12:58:15.689 start       InteropSv92LE 0x4000 02:00:00:00:00:01 smpCnt=51
12:58:15.689 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=51 confRev=1 synch=0 asdu=1/1
    I [A]:     0.000     0.000     0.000     0.000   V [V]:      0.00      0.00      0.00      0.00  quality=good
12:58:16.689 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=51 confRev=1 synch=0 asdu=1/1
    I [A]:     0.000     0.000     0.000     0.000   V [V]:      0.00      0.00      0.00      0.00  quality=good
12:58:16 stats: 4001 ASDU(s)
  SVID           APPID   SRC                STATUS  RATE/s  ASDUS  LOST  DUP  OOO  INVALID  TIMEOUTS  SMPCNT   SYNCH  CONFREV
  InteropSv92LE  0x4000  02:00:00:00:00:01  ok      4002.6  4001   0     0    0    0        0         51/4000  0      1
12:58:16.818 gap         InteropSv92LE 0x4000 02:00:00:00:00:01 smpCnt=568 lost=14
12:58:17.570 gap         InteropSv92LE 0x4000 02:00:00:00:00:01 smpCnt=3574 lost=22
12:58:17.577 gap         InteropSv92LE 0x4000 02:00:00:00:00:01 smpCnt=3603 lost=7
12:58:17.619 gap         InteropSv92LE 0x4000 02:00:00:00:00:01 smpCnt=3771 lost=31
12:58:17 stats: 7927 ASDU(s)
  SVID           APPID   SRC                STATUS  RATE/s  ASDUS  LOST  DUP  OOO  INVALID  TIMEOUTS  SMPCNT   SYNCH  CONFREV
  InteropSv92LE  0x4000  02:00:00:00:00:01  ok      3972.2  7927   74    0    0    0        0         51/4000  0      1
12:58:17.708 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=125 confRev=1 synch=0 asdu=1/1
    I [A]:     0.000     0.000     0.000     0.000   V [V]:      0.00      0.00      0.00      0.00  quality=good
12:58:18.026 gap         InteropSv92LE 0x4000 02:00:00:00:00:01 smpCnt=1397 lost=12
12:58:18 summary: 9915 ASDU(s)
  SVID           APPID   SRC                STATUS  RATE/s  ASDUS  LOST  DUP  OOO  INVALID  TIMEOUTS  SMPCNT     SYNCH  CONFREV
  InteropSv92LE  0x4000  02:00:00:00:00:01  ok      3880.3  9915   86    0    0    0        0         2051/4000  0      1
```

In JSON Lines, `start`, `asdu`, `stats` and `summary` are separate records. A
generic stream from a publisher whose datSet uses the bare LD needs
`--bare-domain`:

```console
$ iec61850ctl sv subscribe --iface eth0 --scl pkg/svops/testdata/interop-sv.cid --svcb MSVCB02 --bare-domain --sample-every 500 --stats-interval 1s --count 1000 --format jsonl
Subscribing to InteropIEDInteropLD/LLN0$MS$MSVCB02 (svID InteropSvGeneric) on eth0 (Ctrl+C to stop)
{"event":"start","time":"2026-10-28T12:58:18.191769Z","src":"02:00:00:00:00:01","app_id":"0x4001","sv_id":"InteropSvGeneric","smp_cnt":512,"smp_synch":0,"conf_rev":1}
{"event":"asdu","received":"2026-10-28T12:58:18.191769Z","src":"02:00:00:00:00:01","dst":"01:0c:cd:04:00:01","app_id":"0x4001","vlan_id":0,"vlan_priority":4,"simulation":false,"no_asdu":4,"asdu_index":0,"sv_id":"InteropSvGeneric","dat_set":"InteropLD/LLN0$dsSvGeneric","smp_cnt":512,"conf_rev":1,"refr_tm":"2026-10-28T12:58:18.188728988Z","smp_synch":0,"smp_rate":1000,"smp_mod":"SmpPerSec","values":[0,{"bits":"0000","len":13},0,{"bits":"0000","len":13},0]}
{"event":"asdu","received":"2026-10-28T12:58:18.739357Z","src":"02:00:00:00:00:01","dst":"01:0c:cd:04:00:01","app_id":"0x4001","vlan_id":0,"vlan_priority":4,"simulation":false,"no_asdu":4,"asdu_index":0,"sv_id":"InteropSvGeneric","dat_set":"InteropLD/LLN0$dsSvGeneric","smp_cnt":12,"conf_rev":1,"refr_tm":"2026-10-28T12:58:18.688728988Z","smp_synch":0,"smp_rate":1000,"smp_mod":"SmpPerSec","values":[0,{"bits":"0000","len":13},0,{"bits":"0000","len":13},0]}
{"event":"summary","time":"2026-10-28T12:58:19.187981Z","asdus":1000,"streams":[{"src":"02:00:00:00:00:01","app_id":"0x4001","sv_id":"InteropSvGeneric","status":"ok","frames":250,"asdus":1000,"rate":885.7,"lost":0,"duplicates":0,"out_of_order":0,"invalid":0,"timeouts":0,"queue_drops":0,"last_smp_cnt":511,"smp_cnt_wrap":1000,"smp_synch":0,"conf_rev":1,"simulation":false}]}
```

Without `--bare-domain` every ASDU of that stream fails the datSet check. The
error is printed once, the table counts them all, and `--count` is not
reached (exit `4`):

```console
$ iec61850ctl sv subscribe --iface eth0 --scl pkg/svops/testdata/interop-sv.cid --svcb MSVCB02 --count 1 --duration 1s
Subscribing to InteropIEDInteropLD/LLN0$MS$MSVCB02 (svID InteropSvGeneric) on eth0 (Ctrl+C to stop)
12:58:19.191 invalid     InteropSvGeneric 0x4001 02:00:00:00:00:01 smpCnt=512 error=sv: InteropSvGeneric: datSet: want InteropIEDInteropLD/LLN0$dsSvGeneric, got InteropLD/LLN0$dsSvGeneric
12:58:20 summary: 1000 ASDU(s)
  SVID              APPID   SRC                STATUS   RATE/s  ASDUS  LOST  DUP  OOO  INVALID  TIMEOUTS  SMPCNT   SYNCH  CONFREV
  InteropSvGeneric  0x4001  02:00:00:00:00:01  invalid  1001.2  1000   0     0    0    1000     0         0/65536  0      0
received 0 of 1 SV ASDU(s) before --duration elapsed
$ echo $?
4
```

## sv publish

Publishes one SVCB from SCL at its configured rate, until Ctrl+C or
`--duration`.

| Flag | Default | Meaning |
|------|---------|---------|
| `--iface` | (required, never defaulted) | interface to publish on |
| `--scl` | (required) | SCL file with the control block |
| `--svcb` | (required) | control block (see [selection](#control-block-selection)) |
| `--yes` | `false` | confirm that frames may be sent on `--iface` (required) |
| `--simulation` | `true` | set the simulation bit in every frame; `--simulation=false` requires `--live` |
| `--live` | `false` | allow `--simulation=false` frames, which subscribers act on |
| `--value` | zero, good quality | sample leaf as `INDEX=VALUE` or `NAME=VALUE` (repeatable) |
| `--waveform` | none | 9-2LE sine on one channel: `CHANNEL=PEAK[,HZ[,PHASE_DEG]]` (repeatable) |
| `--frequency` | `50` | nominal frequency in Hz: the SmpPerPeriod rate and the default waveform frequency |
| `--smp-synch` | `0` | smpSynch to send: 0 none, 1 local, 2 global, 5–254 PTP grandmaster |
| `--src` | interface address | source MAC |
| `--bare-domain` | `false` | send datSet with the bare LD as MMS domain |
| `--duration` | `0` (until Ctrl+C) | stop after this long |
| `--format` | `text` | `text` or `jsonl` |

The header fields (svID, datSet, confRev, smpRate, smpMod, noASDU, OptFlds,
destination, APPID, VLAN) come from the SCL. smpCnt wraps at the samples per
second (4000 for 9-2LE).

### Safety

Sampled Values feed protection functions, so `sv publish` has the same guard
rails as `goose publish`:

1. `--iface` must be given explicitly. It is never defaulted.
2. `--yes` is required.
3. Frames carry the **simulation** bit (IEC 61850-9-2 Ed2). A subscriber that is not in simulation mode (LPHD.Sim false) drops them.
   - To send operational frames you must combine `--simulation=false` with `--live`.
   - The command then prints a `WARNING: publishing LIVE SV` line on stderr.

The checks run before the interface is opened, so a refused publish sends nothing:

```console
$ iec61850ctl sv publish --scl pkg/svops/testdata/interop-sv.cid --svcb MSVCB01 --yes
required flag(s) "iface" not set
$ echo $?
2
```

```console
$ iec61850ctl sv publish --iface eth0 --scl pkg/svops/testdata/interop-sv.cid --svcb MSVCB01 --yes --simulation=false
refusing to publish with simulation=false without --live: subscribers would act on these frames
$ echo $?
2
```

### Static values

`--value` sets one leaf of the sample. You can address a leaf by:
- its index in the layout (`0`, `1`, …);
- a member reference, for single-leaf members (`InteropLD/MMTR1.TotVAh.actVal`);
- `REF#N`, for leaf N of a structured member (`InteropLD/TCTR1.Amp#1`);
- for 9-2LE, a channel name (`IA` … `VN`, raw counts) or a channel quality (`qIA` … `qVN`).

Leaves you do not set are zero, and qualities are good. Values are parsed as
the leaf's encoding:

| Leaf encoding | Format |
|---------------|--------|
| BOOLEAN | `true` / `false` |
| INT8 … INT64, ENUM | integers (decimal or `0x`) |
| INT8U … INT32U | unsigned integers |
| FLOAT32, FLOAT64 | numbers |
| Dbpos | `intermediate`, `off`, `on`, `bad` or `0`–`3` |
| Quality | see [qualities](#samples-layouts-and-qualities) |
| Timestamp | `now` or RFC 3339 |

```console
$ iec61850ctl sv publish --iface eth0 --scl pkg/svops/testdata/interop-sv.cid --svcb MSVCB02 --yes --value 0=1234.5 --value InteropLD/MMTR1.TotVAh.actVal=42 --value 4=1 --duration 2s
Publishing InteropIEDInteropLD/LLN0$MS$MSVCB02 (svID InteropSvGeneric) on eth0 -> 01:0c:cd:04:00:01 appid 0x4001 simulation=true (Ctrl+C to stop)
Published InteropSvGeneric (appid 0x4001 -> 01:0c:cd:04:00:01, simulation=true, 1000 samples/s, 4 ASDU/frame): samples=2000 frames=500 skipped=0 late=10 send-errors=0
```

The summary counts the samples and frames sent. `skipped` counts samples
dropped to catch up after the publisher fell behind, and `late` counts frames
sent more than one sample period after their scheduled time.

### Waveform injection (9-2LE)

`--waveform CHANNEL=PEAK[,HZ[,PHASE_DEG]]` replaces one 9-2LE channel with a
sine:
- `PEAK` is the peak value, in amperes for `IA`–`IN` and in volts for `VA`–`VN`;
- `HZ` defaults to `--frequency`, and `PHASE_DEG` to 0.

Time is the sample index divided by the rate, so the signal is exact whatever
the scheduling jitter. Channels without a waveform keep their `--value` (or
zero). A balanced three-phase system of 100 A and 230 V rms (325 V peak):

```console
$ iec61850ctl sv publish --iface eth0 --scl pkg/svops/testdata/interop-sv.cid --svcb MSVCB01 --yes --waveform IA=100 --waveform IB=100,50,-120 --waveform IC=100,50,120 --waveform VA=325 --waveform VB=325,50,-120 --waveform VC=325,50,120 --duration 2s --format jsonl
Publishing InteropIEDInteropLD/LLN0$MS$MSVCB01 (svID InteropSv92LE) on eth0 -> 01:0c:cd:04:00:00 appid 0x4000 simulation=true (Ctrl+C to stop)
{"event":"summary","sv_id":"InteropSv92LE","app_id":"0x4000","dst":"01:0c:cd:04:00:00","simulation":true,"rate":4000,"no_asdu":1,"samples":7956,"frames":7956,"skipped":45,"late":97,"send_errors":0,"next_smp_cnt":1}
time=2026-10-28T13:58:22.537+01:00 level=WARN msg="sv: publisher behind schedule, samples skipped" svID=InteropSv92LE skipped=36
time=2026-10-28T13:58:22.558+01:00 level=WARN msg="sv: publisher behind schedule, samples skipped" svID=InteropSv92LE skipped=9
```

The same stream as a subscriber sees it. Every tenth sample is one eighth of a
50 Hz period, so the phases advance by 45° per line. Frames carry the
simulation bit, hence `--accept-simulation`:

```console
$ iec61850ctl sv subscribe --iface eth0 --scl pkg/svops/testdata/interop-sv.cid --svcb MSVCB01 --accept-simulation --sample-every 10 --stats-interval 0 --count 80
Subscribing to InteropIEDInteropLD/LLN0$MS$MSVCB01 (svID InteropSv92LE) on eth0 (Ctrl+C to stop)
12:58:24.399 start       InteropSv92LE 0x4000 02:00:00:00:00:01 smpCnt=807
12:58:24.399 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=807 confRev=1 synch=0 asdu=1/1 [sim]
    I [A]:    52.250   -99.966    47.716     0.000   V [V]:    169.81   -324.89    155.08      0.00  quality=good
12:58:24.401 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=817 confRev=1 synch=0 asdu=1/1 [sim]
    I [A]:    97.237   -68.835   -28.402     0.000   V [V]:    316.02   -223.72    -92.30      0.00  quality=good
12:58:24.404 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=827 confRev=1 synch=0 asdu=1/1 [sim]
    I [A]:    85.264     2.618   -87.882     0.000   V [V]:    277.11      8.51   -285.62      0.00  quality=good
12:58:24.406 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=837 confRev=1 synch=0 asdu=1/1 [sim]
    I [A]:    23.345    72.537   -95.882     0.000   V [V]:     75.87    235.75   -311.62      0.00  quality=good
12:58:24.409 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=847 confRev=1 synch=0 asdu=1/1 [sim]
    I [A]:   -52.250    99.966   -47.716     0.000   V [V]:   -169.81    324.89   -155.08      0.00  quality=good
12:58:24.411 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=857 confRev=1 synch=0 asdu=1/1 [sim]
    I [A]:   -97.237    68.835    28.402     0.000   V [V]:   -316.02    223.72     92.30      0.00  quality=good
12:58:24.414 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=867 confRev=1 synch=0 asdu=1/1 [sim]
    I [A]:   -85.264    -2.618    87.882     0.000   V [V]:   -277.11     -8.51    285.62      0.00  quality=good
12:58:24.416 asdu  02:00:00:00:00:01 -> 01:0c:cd:04:00:00 appid=0x4000 vlan=0/4 svID=InteropSv92LE smpCnt=877 confRev=1 synch=0 asdu=1/1 [sim]
    I [A]:   -23.345   -72.537    95.882     0.000   V [V]:    -75.87   -235.75    311.62      0.00  quality=good
12:58:24 summary: 80 ASDU(s)
  SVID           APPID   SRC                STATUS    RATE/s  ASDUS  LOST  DUP  OOO  INVALID  TIMEOUTS  SMPCNT    SYNCH  CONFREV
  InteropSv92LE  0x4000  02:00:00:00:00:01  ok (sim)  0.0     80     0     0    0    0        0         886/4000  0      1
```

`--waveform` needs the 9-2LE layout. On another control block it is a usage
error (exit `2`).

## sv svcb get / set

`sv svcb get` and `sv svcb set` read and write a multicast SV control block
(MSVCB, functional constraint MS) over MMS. They use the global connection
flags (`--host`, `--port`, timeouts). Unicast USVCBs are not supported.

| Flag | Command | Meaning |
|------|---------|---------|
| `--ld` | both | logical device (required) |
| `--svcb` | both | MSVCB name (`MSVCB01`) or item (`LLN0$MS$MSVCB01`, `LLN0.MS.MSVCB01`) (required) |
| `--format` | both | `text` or `json` |
| `--enable` / `--disable` | set | write SvEna |
| `--msv-id` | set | new MsvID (only while disabled) |
| `--dat-set` | set | new DatSet reference (only while disabled) |
| `--smp-rate` | set | new SmpRate (only while disabled) |
| `--opt-flds` | set | new OptFlds: comma list of `refresh-time`, `sample-synchronized`, `sample-rate`, `data-set`, `security`, or `none` (only while disabled) |
| `--smp-mod` | set | new SmpMod: `SmpPerPeriod`, `SmpPerSec`, `SecPerSmp` (only while disabled) |

```console
$ iec61850ctl --host 127.0.0.1 --port 10103 sv svcb get --ld InteropLD --svcb MSVCB01
Connecting to 127.0.0.1:10103
MSVCB InteropLD/LLN0$MS$MSVCB01
  SvEna:      true
  MsvID:      InteropSv92LE
  DatSet:     InteropLD/LLN0$PhsMeas1
  ConfRev:    1
  SmpRate:    80 SmpPerPeriod
  OptFlds:    sample-synchronized
  DstAddress: 01:0c:cd:04:00:00 appid 0x4000 vlan 0 prio 4
  noASDU:     1
```

```console
$ iec61850ctl --host 127.0.0.1 --port 10103 sv svcb get --ld InteropLD --svcb MSVCB02 --format json
Connecting to 127.0.0.1:10103
{
  "reference": "InteropLD/LLN0$MS$MSVCB02",
  "sv_ena": true,
  "msv_id": "InteropSvGeneric",
  "dat_set": "InteropLD/LLN0$dsSvGeneric",
  "conf_rev": 1,
  "smp_rate": 1000,
  "smp_mod": "SmpPerSec",
  "opt_flds": [
    "refresh-time",
    "sample-synchronized",
    "sample-rate",
    "data-set"
  ],
  "dst": "01:0c:cd:04:00:01",
  "vlan_priority": 4,
  "vlan_id": 0,
  "app_id": "0x4001",
  "no_asdu": 4
}
```

- **Order.** SvEna is written last. That lets you change attributes and enable in one call, but only if the block is already disabled.
- **Disabled-only changes.** Servers accept the other attributes only while SvEna is false. A write the server refuses exits with code `6`.
- **After a write.** `set` prints the MSVCB as read back from the server. If `--enable` or `--disable` was given and SvEna reads back the other value (the server accepted the write but does not publish), `set` exits with code `7`.

An OptFlds change on an enabled block is refused:

```console
$ iec61850ctl --host 127.0.0.1 --port 10103 sv svcb set --ld InteropLD --svcb MSVCB01 --opt-flds refresh-time,sample-rate
Connecting to 127.0.0.1:10103
set MSVCB InteropLD/LLN0$MS$MSVCB01: iec61850: set MSVCB InteropLD/LLN0$MS$MSVCB01.OptFlds: mms: data access error: TemporarilyUnavailable
$ echo $?
6
```

Disable the block first, then change it and re-enable it in one call:

```console
$ iec61850ctl --host 127.0.0.1 --port 10103 sv svcb set --ld InteropLD --svcb MSVCB01 --disable
Connecting to 127.0.0.1:10103
MSVCB InteropLD/LLN0$MS$MSVCB01
  SvEna:      false
  MsvID:      InteropSv92LE
  DatSet:     InteropLD/LLN0$PhsMeas1
  ConfRev:    1
  SmpRate:    80 SmpPerPeriod
  OptFlds:    sample-synchronized
  DstAddress: 01:0c:cd:04:00:00 appid 0x4000 vlan 0 prio 4
  noASDU:     1
```

```console
$ iec61850ctl --host 127.0.0.1 --port 10103 sv svcb set --ld InteropLD --svcb MSVCB01 --opt-flds refresh-time,sample-rate --enable
Connecting to 127.0.0.1:10103
MSVCB InteropLD/LLN0$MS$MSVCB01
  SvEna:      true
  MsvID:      InteropSv92LE
  DatSet:     InteropLD/LLN0$PhsMeas1
  ConfRev:    1
  SmpRate:    80 SmpPerPeriod
  OptFlds:    refresh-time,sample-rate
  DstAddress: 01:0c:cd:04:00:00 appid 0x4000 vlan 0 prio 4
  noASDU:     1
```

## Exit codes

The codes are the same as for the rest of the CLI ([AUTOMATION.md](AUTOMATION.md)).

| Code | SV cases |
|------|----------|
| `2` | missing `--yes`, `--iface` or `--live`; bad `--value`, `--waveform`, `--layout`, APPID or MAC; `--waveform` on a non-9-2LE block; an SCL that cannot be parsed |
| `4` | `--count` not reached before `--duration` (sniff and subscribe) |
| `5` | interface or control block not found |
| `6` | raw socket permission denied; MSVCB write rejected |
| `8` | platform without a raw transport; a connection without MSVCB access |

## Limitations

These come from go-iec61850 v1.2:
- No SV security (IEC 62351-6) and no unicast SVCBs (USVCB).
- smpMod is encoded in two octets. libiec61850's subscriber drops such ASDUs (see [INTEROP.md](INTEROP.md)).

`server start` does not publish SV on a network interface. Its SV engine is
only used in tests, through the in-memory hub.

## Interoperability

The e2e suite runs `sv subscribe`, `sv sniff` and `sv publish` for both
streams against the SV publishers and subscribers of libiec61850, dscsystems
and iec61850-rust (mms-interop `sv-v1`) on a Docker bridge network. It also
reads `MSVCB02` with `sv svcb get` from the libiec61850 and dscsystems IED
servers. See [INTEROP.md](INTEROP.md#sampled-values-raw-ethernet).
