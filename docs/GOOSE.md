# GOOSE

`iec61850ctl goose` works with IEC 61850-8-1 GOOSE on raw Ethernet. It uses the
[go-iec61850](https://github.com/otfabric/go-iec61850) `goose` package for the
frame codec, publisher, subscriber and raw transports.

| Command | Network | Purpose |
|---------|---------|---------|
| `goose list --scl` | none | Lists GoCBs from SCL: GSEControl, GSE address and dataset |
| `goose sniff --iface` | receive | Decodes every GOOSE frame, with optional filters |
| `goose subscribe --iface --scl --gocb` | receive | Validated subscription to one GoCB, with supervision |
| `goose publish --iface --scl --gocb --yes` | **send** | Publishes one GoCB |
| `goose gocb get / set` | MMS (`--host`) | Reads or writes a GoCB on an IED |

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

## list

```sh
iec61850ctl goose list --scl station.scd
iec61850ctl goose list --scl station.scd --format json
```

The JSON form adds the dataset members as `LD/LN.DO.DA[FC]`. Blocks without a
GSE address are listed with `has_address: false`.

## sniff

```sh
iec61850ctl goose sniff --iface eth0
iec61850ctl goose sniff --iface eth0 --appid 0x1001 --appid 0x1002 --count 20 --format jsonl
iec61850ctl goose sniff --iface eth0 --dst 01:0c:cd:01:00:01 --gocb-ref IED1LD0/LLN0\$GO\$gcb1
```

- Without `--dst`, the interface is put in promiscuous mode. With `--dst`, it joins that multicast group only.
- APPIDs are hex, as in SCL. Use `d:<n>` for decimal.
- Frames that cannot be decoded are reported on stderr and counted.

## subscribe

```sh
iec61850ctl goose subscribe --iface eth0 --scl station.scd --gocb IED1/LD0/gcb1 --duration 30s
iec61850ctl goose subscribe --iface eth0 --scl device.cid --gocb gcb1 --count 5 --duration 10s --format jsonl
```

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

Two flags change what is received:
- `--only-changes` drops retransmissions.
- `--accept-simulation` accepts frames with the Edition 2 simulation bit set.

## publish

Publishing puts frames on the station bus, so three guard rails apply:

1. `--iface` must be given explicitly. It is never defaulted.
2. `--yes` is required.
3. Frames carry **test=true**. Subscribers that honour the test flag do not act on them.
   - To send operational frames you must combine `--test=false` with `--live`.
   - The command then prints a `WARNING: publishing LIVE GOOSE` line on stderr.

```sh
# Test frames: initial state, then retransmissions for 10 s
iec61850ctl goose publish --iface eth0 --scl device.cid --gocb gcb1 --yes --duration 10s

# Set member values and publish three further state changes, 1 s apart
iec61850ctl goose publish --iface eth0 --scl device.cid --gocb gcb1 --yes \
  --value 0=true --value 'LD0/GGIO1.Ind1.q[ST]=0000' --changes 3 --interval 1s --duration 5s
```

- **Values.** `--value` takes either `INDEX=VALUE` or `MEMBER=VALUE`. Members you do not set keep the zero value of their type. Each value is parsed as its member's type:

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
- **Timing.** Retransmissions follow IEC 61850-8-1 Annex C: MinTime first, doubling up to the MaxTime heartbeat. Both times come from the SCL GSE address.
- **Exit.** The run ends on Ctrl+C or after `--duration`. The summary shows the final stNum, sqNum and the number of frames sent.

## GoCB over MMS

```sh
iec61850ctl goose gocb get --host 192.0.2.10 --ld LD0 --gocb gcb1 --format json
iec61850ctl goose gocb set --host 192.0.2.10 --ld LD0 --gocb gcb1 --disable
iec61850ctl goose gocb set --host 192.0.2.10 --ld LD0 --gocb gcb1 --go-id NEWID --dat-set 'LD0/LLN0$ds2' --enable
```

- **Writable attributes.** Only GoEna, GoID and DatSet can be written. GoEna is written last, so you can change GoID or DatSet and enable in one call.
- **Disabled-only changes.** Servers accept GoID and DatSet changes only while GoEna is false. A write the server refuses exits with code `6`.
- **After a write.** `set` prints the GoCB as read back from the server.

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

These come from go-iec61850 v1.1.0:
- No GOOSE security (IEC 62351-6).
- No fixed-offset encoding.
- No GSSE or Sampled Values.

## Interoperability

The e2e suite runs `goose subscribe`, `goose sniff` and `goose publish`
against the GOOSE publishers and subscribers of libiec61850, dscsystems and
iec61850-rust (mms-interop `goose-v1`) on a Docker bridge network. See
[INTEROP.md](INTEROP.md#goose-raw-ethernet).

## Server

`server start` does not publish GOOSE on a network interface. Its GoCB
engine is only used in tests, through the in-memory hub.
