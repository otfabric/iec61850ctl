# Command reference

Every command of `iec61850ctl` and its flags, as `--help` prints them. Task-oriented examples are in the [README](../README.md); output formats in [json-output.md](json-output.md); exit codes in [exitcodes.md](exitcodes.md).

## Global flags

Accepted by every command.

| Flag | Type | Meaning |
|---|---|---|
| `--ae-qualifier` | int | called ACSE AE-qualifier, the IED's OSI-AE-Qualifier (default 12) |
| `--ap-title` | string | called ACSE AP-title, the IED's OSI-AP-Title ('1,3,9999,23' or dotted; '' omits it) (default "1.1.1.999.1") |
| `--connect-timeout` | duration | timeout for each connect attempt (TCP + COTP + MMS association) (default 10s) |
| `--debug` | bool | shorthand for --log-level=debug |
| `--host` | string | host address of the IEC 61850 server (env: IEC61850_HOST) |
| `--ied-name` | string | IED name for MMS domain normalisation / SCL IED selection (env: IEC61850_IED_NAME) |
| `--local-tsel` | string | calling COTP transport selector in hex ('' omits it) (default "0001") |
| `--log-format` | string | structured log format: text, json (default "text") |
| `--log-level` | string | structured log level on stderr: debug, info, warn, error, off (default "warn") |
| `--port` | int | port of the IEC 61850 server (env: IEC61850_PORT) (default 102) |
| `--psel` | string | called presentation selector in hex, the IED's OSI-PSEL ('' omits it) (default "00000001") |
| `--remote-tsel` | string | called COTP transport selector in hex, the IED's OSI-TSEL ('' omits it) (default "0001") |
| `--retry` | int | number of connect retries with exponential backoff and jitter |
| `--retry-max-delay` | duration | maximum delay between connect retries (default 5s) |
| `--ssel` | string | called session selector in hex, the IED's OSI-SSEL ('' omits it) (default "0001") |
| `--timeout` | duration | timeout for each MMS request (default 10s) |
| `--tpdu-size` | int | COTP TPDU size proposed in the connection request, 128..65531 (65531 omits it) (default 1024) |

`-h`, `--help` prints the help of any command.

## Commands

| Command | Purpose |
|---|---|
| `iec61850ctl completion` | Generate the autocompletion script for the specified shell |
| `iec61850ctl completion bash` | Generate the autocompletion script for bash |
| `iec61850ctl completion fish` | Generate the autocompletion script for fish |
| `iec61850ctl completion powershell` | Generate the autocompletion script for powershell |
| `iec61850ctl completion zsh` | Generate the autocompletion script for zsh |
| `iec61850ctl control` | Inspect and operate IEC 61850 controllable objects |
| `iec61850ctl control inspect` | Read the control model of a controllable object |
| `iec61850ctl control operate` | Atomically select (if required) and operate a controllable object |
| `iec61850ctl discover` | Discover IEC 61850 devices on the network |
| `iec61850ctl find` | Find paths matching criteria |
| `iec61850ctl find bulk` | Find paths for all entries in a mapping file with minimal device calls |
| `iec61850ctl find path` | Find paths matching logical node and data object criteria |
| `iec61850ctl get` | Get/read values from the IEC 61850 server |
| `iec61850ctl get ds` | Get a specific data set by name |
| `iec61850ctl get file` | Download a file from the IEC 61850 server |
| `iec61850ctl get journal` | Read journal entries by time range or from a start time |
| `iec61850ctl get object` | Read a specific data object from the IEC 61850 server |
| `iec61850ctl get report` | Get detailed configuration of a specific report control block |
| `iec61850ctl goose` | GOOSE: list control blocks, sniff, subscribe, publish, read/write GoCBs |
| `iec61850ctl goose gocb` | Read or write a GOOSE control block over MMS |
| `iec61850ctl goose gocb get` | Read a GoCB (GoEna, GoID, DatSet, ConfRev, DstAddress, times) |
| `iec61850ctl goose gocb set` | Write GoCB attributes (GoEna written last) |
| `iec61850ctl goose list` | List GOOSE control blocks (GSEControl + GSE address + dataset) from SCL |
| `iec61850ctl goose publish` | Publish a GoCB from SCL (test frames unless --test=false --live) |
| `iec61850ctl goose sniff` | Decode and print GOOSE frames seen on an interface |
| `iec61850ctl goose subscribe` | Subscribe to one GoCB from SCL with validation and supervision |
| `iec61850ctl help` | Help about any command |
| `iec61850ctl http` | Start HTTP/REST API server |
| `iec61850ctl list` | List devices, nodes, and objects from the IEC 61850 server |
| `iec61850ctl list das` | List all data attributes within a specific data object |
| `iec61850ctl list dos` | List all data objects within a specific logical node |
| `iec61850ctl list dss` | List all data sets in a logical node |
| `iec61850ctl list files` | List files available on the IEC 61850 server |
| `iec61850ctl list journals` | List journals within a logical device |
| `iec61850ctl list lds` | List all logical devices (alias: domains) available on the IEC 61850 server |
| `iec61850ctl list lns` | List all logical nodes within a specific logical device |
| `iec61850ctl list reports` | List all report control blocks in a logical node or across the server |
| `iec61850ctl scl` | Parse and convert SCL/CID/ICD files (no server connection) |
| `iec61850ctl scl convert` | Convert an SCL/CID file to CSV |
| `iec61850ctl scl parse` | Parse an SCL/CID file and print structure or flat list |
| `iec61850ctl server` | Run MMS server from SCL, or export libiec61850 .cfg |
| `iec61850ctl server generate-config` | Generate libiec61850 .cfg from serialized IED JSON |
| `iec61850ctl server start` | Start IEC 61850 MMS server from SCL file |
| `iec61850ctl set` | Write values to the IEC 61850 server |
| `iec61850ctl set object` | Write a scalar data attribute |
| `iec61850ctl subscribe` | Subscribe to server events (reports) with graceful cleanup |
| `iec61850ctl subscribe report` | Subscribe to RCB notifications with auto-cleanup |
| `iec61850ctl sv` | Sampled Values: list control blocks, sniff, subscribe, publish, read/write MSVCBs |
| `iec61850ctl sv list` | List SV control blocks (SampledValueControl + SMV address + dataset) from SCL |
| `iec61850ctl sv publish` | Publish an SVCB from SCL (simulation frames unless --simulation=false --live) |
| `iec61850ctl sv sniff` | Decode and print SV ASDUs seen on an interface (no supervision) |
| `iec61850ctl sv subscribe` | Subscribe to one SVCB from SCL with validation, supervision and live statistics |
| `iec61850ctl sv svcb` | Read or write a multicast SV control block (MSVCB) over MMS |
| `iec61850ctl sv svcb get` | Read an MSVCB (SvEna, MsvID, DatSet, ConfRev, SmpRate, OptFlds, SmpMod, DstAddress, noASDU) |
| `iec61850ctl sv svcb set` | Write MSVCB attributes (SvEna written last) |
| `iec61850ctl tree` | Display the complete IEC 61850 device tree |
| `iec61850ctl version` | Display version information |

### `iec61850ctl completion bash`

Generate the autocompletion script for bash.

| Flag | Type | Meaning |
|---|---|---|
| `--no-descriptions` | bool | disable completion descriptions |

### `iec61850ctl completion fish`

Generate the autocompletion script for fish.

| Flag | Type | Meaning |
|---|---|---|
| `--no-descriptions` | bool | disable completion descriptions |

### `iec61850ctl completion powershell`

Generate the autocompletion script for powershell.

| Flag | Type | Meaning |
|---|---|---|
| `--no-descriptions` | bool | disable completion descriptions |

### `iec61850ctl completion zsh`

Generate the autocompletion script for zsh.

| Flag | Type | Meaning |
|---|---|---|
| `--no-descriptions` | bool | disable completion descriptions |

### `iec61850ctl control inspect`

Read the control model of a controllable object.

| Flag | Type | Meaning |
|---|---|---|
| `--format` | string | Output format: text, json (default "text") |
| `--object` | string | controllable DO reference without FC (required) |

### `iec61850ctl control operate`

Atomically select (if required) and operate a controllable object.

| Flag | Type | Meaning |
|---|---|---|
| `--check-interlock` | bool | set interlockCheck bit |
| `--check-synchro` | bool | set synchrocheck bit |
| `--confirm-ref` | string | complete reference for status read-back, e.g. LD/LN.DO.stVal[ST] |
| `--ctl-num` | int | control sequence number 1..255 (0=auto) |
| `--dry-run` | bool | plan the sequence without writing SBO/SBOw/Oper |
| `--format` | string | Output format: text, json (default "text") |
| `--mode` | string | control mode: auto\|direct\|sbo\|sbow (default "auto") |
| `--no-confirm` | bool | skip confirmation even if --confirm-ref is set |
| `--object` | string | controllable DO reference without FC (required) |
| `--or-cat` | string | originator category (default "remote-control") |
| `--or-ident` | string | originator identifier (UTF-8, max 64 bytes) |
| `--test` | bool | set Test bit (also disables confirmation) |
| `--type` | string | value type: bool\|int\|uint\|float\|enum\|string (required) |
| `--value` | string | control value (required unless --dry-run with inspect-only planning) |
| `--yes` | bool | confirm that the control may change device state (required unless --dry-run) |

### `iec61850ctl discover`

Discover IEC 61850 devices on the network.

| Flag | Type | Meaning |
|---|---|---|
| `--host` | string | subnet (10.0.0.0/24) or IP range (10.0.0.5-25) to scan (required) |
| `--port` | int | port to scan (default 102) |
| `--resolve-mac` | bool | resolve MAC addresses (requires root privileges) |

### `iec61850ctl find bulk`

Find paths for all entries in a mapping file with minimal device calls.

| Flag | Type | Meaning |
|---|---|---|
| `--mapping` | string | path to mapping JSON file (required) |
| `--output` | string | path to output results JSON file (required) |

### `iec61850ctl find path`

Find paths matching logical node and data object criteria.

| Flag | Type | Meaning |
|---|---|---|
| `--detailed` | bool | show detailed information including DA details (FC, Type, Value) |
| `--include-das` | bool | include all leaf data attributes |
| `--ln` | string | logical node pattern (regex, required) |
| `--path` | string | data object path (DO or DO.DA format, exact match, required) |

### `iec61850ctl get ds`

Get a specific data set by name.

| Flag | Type | Meaning |
|---|---|---|
| `--detailed` | bool | Read and show current values for each member |
| `--format` | string | Output format: text, json (default "text") |
| `--ld` | string | Logical device name (required) |
| `--ln` | string | Logical node name (required) |
| `--name` | string | Data set name (required) |

### `iec61850ctl get file`

Download a file from the IEC 61850 server.

| Flag | Type | Meaning |
|---|---|---|
| `--detailed` | bool | show download metadata (size, checksum) |
| `--name` | string | filename to download (required) |
| `--output` | string | local file path to save (optional, defaults to stdout) |

### `iec61850ctl get journal`

Read journal entries by time range or from a start time.

| Flag | Type | Meaning |
|---|---|---|
| `--domain` | string | MMS domain (required); use 'list journals' or try an LD name from 'list lds' |
| `--from` | string | start time (required): RFC3339, 2006-01-02 15:04:05.000 UTC, or ms since epoch |
| `--journal` | string | journal/log name within the domain (required) |
| `--to` | string | end time (optional): same format as --from; if omitted, entries after --from are returned |

### `iec61850ctl get object`

Read a specific data object from the IEC 61850 server.

| Flag | Type | Meaning |
|---|---|---|
| `--detailed` | bool | show detailed information including FC and Type |
| `--fc` | string | functional constraint (ST, MX, SP, SV, CF, DC, SG, SE, SR, OR, BL, EX, CO, RP, BR, ALL) (default "MX") |
| `--format` | string | Output format: text, json (default "text") |
| `--object` | string | object reference to read (required) |
| `--type` | string | value type hint (unused; type is inferred from the server) (default "???") |

### `iec61850ctl get report`

Get detailed configuration of a specific report control block.

| Flag | Type | Meaning |
|---|---|---|
| `--detailed` | bool | Fetch and print the report's dataset (members and current values) |
| `--format` | string | Output format: text, json (default "text") |
| `--ld` | string | Logical device name (required) |
| `--ln` | string | Logical node name (required) |
| `--report` | string | Report name (required) |

### `iec61850ctl goose gocb get`

Read a GoCB (GoEna, GoID, DatSet, ConfRev, DstAddress, times).

| Flag | Type | Meaning |
|---|---|---|
| `--format` | string | Output format: text, json (default "text") |
| `--gocb` | string | GoCB name (gcb1) or item (LLN0$GO$gcb1) (required) |
| `--ld` | string | logical device (required) |

### `iec61850ctl goose gocb set`

Write GoCB attributes (GoEna written last).

| Flag | Type | Meaning |
|---|---|---|
| `--dat-set` | string | new DatSet reference (only while disabled; increments ConfRev) |
| `--disable` | bool | set GoEna=false |
| `--enable` | bool | set GoEna=true |
| `--format` | string | Output format: text, json (default "text") |
| `--go-id` | string | new GoID (only while disabled) |
| `--gocb` | string | GoCB name (gcb1) or item (LLN0$GO$gcb1) (required) |
| `--ld` | string | logical device (required) |
| `--yes` | bool | confirm that the GoCB write may change device state (required) |

### `iec61850ctl goose list`

List GOOSE control blocks (GSEControl + GSE address + dataset) from SCL.

| Flag | Type | Meaning |
|---|---|---|
| `--format` | string | Output format: text, json (default "text") |
| `--scl` | string | SCL file (SCD/CID/ICD) (required) |

### `iec61850ctl goose publish`

Publish a GoCB from SCL (test frames unless --test=false --live).

| Flag | Type | Meaning |
|---|---|---|
| `--bare-domain` | bool | publish references with the bare LD as MMS domain |
| `--changes` | int | additional state changes to publish after the initial state |
| `--duration` | duration | keep retransmitting this long after the last change (0 = until Ctrl+C) |
| `--format` | string | Output format: text, jsonl (default "text") |
| `--gocb` | string | control block: name, LD/name, IED/LD/name or gocbRef (required) |
| `--iface` | string | network interface to publish on (required, never defaulted) |
| `--interval` | duration | delay between state changes (default 1s) |
| `--live` | bool | allow test=false frames that subscribers act on |
| `--scl` | string | SCL file with the control block (required) |
| `--simulation` | bool | set the Edition 2 simulation bit |
| `--src` | string | source MAC (default: the interface address) |
| `--test` | bool | set the test flag in every frame; --test=false requires --live (default true) |
| `--value` | stringArray | dataset value as INDEX=VALUE or MEMBER=VALUE (repeatable; others are zero) |
| `--yes` | bool | confirm that frames may be sent on --iface (required) |

### `iec61850ctl goose sniff`

Decode and print GOOSE frames seen on an interface.

| Flag | Type | Meaning |
|---|---|---|
| `--appid` | strings | only frames with these APPIDs (hex, repeatable) |
| `--count` | int | stop after this many frames/messages (0 = unlimited) |
| `--dst` | string | only frames to this destination MAC (joins the multicast group) |
| `--duration` | duration | stop after this long (0 = until Ctrl+C); with --count, exit 4 if not reached |
| `--format` | string | Output format: text, jsonl (default "text") |
| `--gocb-ref` | string | only frames with this gocbRef |
| `--iface` | string | network interface to capture on (required) |

### `iec61850ctl goose subscribe`

Subscribe to one GoCB from SCL with validation and supervision.

| Flag | Type | Meaning |
|---|---|---|
| `--accept-simulation` | bool | accept frames with the simulation bit set |
| `--bare-domain` | bool | expect references with the bare LD as MMS domain (LD/LLN0$GO$gcb) instead of IED+LD |
| `--count` | int | stop after this many frames/messages (0 = unlimited) |
| `--duration` | duration | stop after this long (0 = until Ctrl+C); with --count, exit 4 if not reached |
| `--format` | string | Output format: text, jsonl (default "text") |
| `--gocb` | string | control block: name, LD/name, IED/LD/name or gocbRef (required) |
| `--iface` | string | network interface to receive on (required) |
| `--only-changes` | bool | print state changes only, not retransmissions |
| `--scl` | string | SCL file describing the publisher (required) |

### `iec61850ctl help`

Help about any command.

No flags of its own.

### `iec61850ctl http`

Start HTTP/REST API server.

| Flag | Type | Meaning |
|---|---|---|
| `--iec-host` | string | IEC 61850 device host (required) |
| `--iec-port` | int | IEC 61850 device port (default 102) |
| `--listen` | string | HTTP server listen address (default ":8080") |

### `iec61850ctl list das`

List all data attributes within a specific data object.

| Flag | Type | Meaning |
|---|---|---|
| `--detailed` | bool | show timestamp and quality metadata |
| `--do` | string | data object name (required) |
| `--format` | string | Output format: text, json, csv, table, yaml (default "text") |
| `--ld` | string | logical device name (required) |
| `--ln` | string | logical node name (required) |

### `iec61850ctl list dos`

List all data objects within a specific logical node.

| Flag | Type | Meaning |
|---|---|---|
| `--detailed` | bool | show data attribute count for each data object |
| `--format` | string | Output format: text, json, csv, table, yaml (default "text") |
| `--ld` | string | logical device name (required) |
| `--ln` | string | logical node name (required) |

### `iec61850ctl list dss`

List all data sets in a logical node.

| Flag | Type | Meaning |
|---|---|---|
| `--detailed` | bool | Show detailed information including DataSet members |
| `--format` | string | Output format: text, json (default "text") |
| `--ld` | string | Logical device name (required) |
| `--ln` | string | Logical node name (required) |

### `iec61850ctl list files`

List files available on the IEC 61850 server.

| Flag | Type | Meaning |
|---|---|---|
| `--detailed` | bool | show file metadata (size, last modified) |
| `--path` | string | directory path / filename pattern (optional) |
| `--recursive` | bool | list files with path-depth indentation |

### `iec61850ctl list journals`

List journals within a logical device.

| Flag | Type | Meaning |
|---|---|---|
| `--domain` | string | MMS domain name (alias for --ld) |
| `--ld` | string | logical device (LD) name to list journals from |

### `iec61850ctl list lds`

List all logical devices (alias: domains) available on the IEC 61850 server.

| Flag | Type | Meaning |
|---|---|---|
| `--detailed` | bool | Show detailed breakdown of DataSets and Reports per LN |
| `--format` | string | Output format: text, json, csv, table, yaml (default "text") |

### `iec61850ctl list lns`

List all logical nodes within a specific logical device.

| Flag | Type | Meaning |
|---|---|---|
| `--detailed` | bool | Show detailed LN information including DO/DataSet/Report counts |
| `--format` | string | Output format: text, json, csv, table, yaml (default "text") |
| `--ld` | string | logical device name (required) |

### `iec61850ctl list reports`

List all report control blocks in a logical node or across the server.

| Flag | Type | Meaning |
|---|---|---|
| `--all` | bool | List reports in all logical devices and logical nodes (mutually exclusive with --ld and --ln) |
| `--detailed` | bool | Show detailed report configuration |
| `--format` | string | Output format: text, json (default "text") |
| `--ld` | string | Logical device name (required unless --all) |
| `--ln` | string | Logical node name (required unless --all) |

### `iec61850ctl scl convert`

Convert an SCL/CID file to CSV.

| Flag | Type | Meaning |
|---|---|---|
| `--input` | string | path to SCL/CID file (required) |
| `--output` | string | path to output CSV file (required) |
| `--separator` | string | CSV field separator (e.g. "," or "\|") (default ",") |

### `iec61850ctl scl parse`

Parse an SCL/CID file and print structure or flat list.

| Flag | Type | Meaning |
|---|---|---|
| `--detailed` | bool | include FC, type, and enum in output |
| `--flatten` | bool | output flat path list instead of tree |
| `--input` | string | path to SCL/CID file (required) |

### `iec61850ctl server generate-config`

Generate libiec61850 .cfg from serialized IED JSON.

| Flag | Type | Meaning |
|---|---|---|
| `--input` (`-i`) | string | path to serialized IED JSON (required) |
| `--name` | string | MODEL name in .cfg (MMS domain = name+LD); empty = LD name only (match source device) |
| `--output` (`-o`) | string | path to write .cfg file (required) |

### `iec61850ctl server start`

Start IEC 61850 MMS server from SCL file.

| Flag | Type | Meaning |
|---|---|---|
| `--allow-readonly-fc-writes` | bool | accept client writes to status and measured values (FC ST, MX, SG, SR, OR, EX); simulators and test fixtures only, IEC 61850-7-2 makes them read-only |
| `--files-dir` | string | directory served read-only over MMS file services (list files / get file) |
| `--fixture-id` | string | optional fixture identifier included in readiness JSON |
| `--host` | string | bind address (default "0.0.0.0") |
| `--instance-domain-names` | bool | name each MMS domain by its LDevice@inst alone (InteropLD), as iec61850ctl before v0.9.0, instead of the IEC 61850-8-1 name (ldName, else IED name + inst: InteropIEDInteropLD) |
| `--max-connections` | int | reserved; not enforced yet (default 5) |
| `--port` (`-p`) | int | MMS port (0 = ephemeral; the bound address is printed and included in --ready-json) (default 102) |
| `--ready-json` | bool | emit one JSON readiness event on stdout after bind |
| `--scl` | string | path to SCL/CID/ICD file (required) |
| `--values` | string | path to serialized IED JSON (leaf values applied before start) |

### `iec61850ctl set object`

Write a scalar data attribute.

| Flag | Type | Meaning |
|---|---|---|
| `--fc` | string | functional constraint (required) |
| `--format` | string | Output format: text, json (default "text") |
| `--object` | string | object reference (required) |
| `--type` | string | value type: bool\|int\|uint\|float\|enum\|string (required) |
| `--value` | string | value to write (required) |
| `--verify` | bool | read back the same reference after write |
| `--yes` | bool | confirm that the write may change device state (required) |

### `iec61850ctl subscribe report`

Subscribe to RCB notifications with auto-cleanup.

| Flag | Type | Meaning |
|---|---|---|
| `--duration` | string | Auto-terminate after duration (e.g. 30s, 5m, 1h) |
| `--entry-id` | string | BRCB: resume EntryID as hex octets |
| `--format` | string | Output format: text, jsonl (default "text") |
| `--interrogation` | bool | Trigger General Interrogation (GI) on the RCB after enabling to request an immediate report (may include 0 items) |
| `--keepalive` | duration | Probe the connection at this interval and fail with exit 3 when it is lost (0 disables) (default 5s) |
| `--ld` | string | Logical device name (required) |
| `--ln` | string | Logical node name (required) |
| `--max-reports` | int | Exit after receiving this many reports (0 = no limit) |
| `--purge-buf` | bool | BRCB: purge buffer before enable (requires --yes) |
| `--report` | string | Report control block name (required) |
| `--resv-tms` | int | BRCB: write ResvTms before enable |
| `--show-values` | bool | Show data set values in each report |
| `--sync` | bool | Read the report dataset once to obtain a full baseline snapshot (one-shot state sync) |
| `--type` | string | Report type: BR (buffered/BRCB) or RP (unbuffered/URCB) (default "BR") |
| `--yes` | bool | confirm that --purge-buf may discard the BRCB buffer on the device (required with --purge-buf) |

### `iec61850ctl sv list`

List SV control blocks (SampledValueControl + SMV address + dataset) from SCL.

| Flag | Type | Meaning |
|---|---|---|
| `--format` | string | Output format: text, json (default "text") |
| `--scl` | string | SCL file (SCD/CID/ICD) (required) |

### `iec61850ctl sv publish`

Publish an SVCB from SCL (simulation frames unless --simulation=false --live).

| Flag | Type | Meaning |
|---|---|---|
| `--bare-domain` | bool | send datSet with the bare LD as MMS domain |
| `--duration` | duration | stop after this long (0 = until Ctrl+C) |
| `--format` | string | Output format: text, jsonl (default "text") |
| `--frequency` | float | nominal frequency in Hz (SmpPerPeriod rate and default waveform frequency) (default 50) |
| `--iface` | string | network interface to publish on (required, never defaulted) |
| `--live` | bool | allow simulation=false frames that subscribers act on |
| `--scl` | string | SCL file with the control block (required) |
| `--simulation` | bool | set the simulation bit in every frame; --simulation=false requires --live (default true) |
| `--smp-synch` | uint8 | smpSynch to send: 0 none, 1 local, 2 global, 5-254 PTP grandmaster |
| `--src` | string | source MAC (default: the interface address) |
| `--svcb` | string | control block: name, LD/name, IED/LD/name or SvCBRef (required) |
| `--value` | stringArray | sample leaf as INDEX=VALUE or NAME=VALUE: member ref, REF#N, or 9-2LE channel IA..VN / qIA..qVN (repeatable; others are zero/good) |
| `--waveform` | stringArray | 9-2LE sine as CHANNEL=PEAK[,HZ[,PHASE_DEG]] in A (IA..IN) or V (VA..VN) (repeatable) |
| `--yes` | bool | confirm that frames may be sent on --iface (required) |

### `iec61850ctl sv sniff`

Decode and print SV ASDUs seen on an interface (no supervision).

| Flag | Type | Meaning |
|---|---|---|
| `--appid` | strings | only frames with these APPIDs (hex, repeatable) |
| `--count` | int | stop after this many matching ASDUs (0 = unlimited) |
| `--dst` | string | only frames to this destination MAC (joins the multicast group) |
| `--duration` | duration | stop after this long (0 = until Ctrl+C); with --count, exit 4 if not reached |
| `--format` | string | Output format: text, jsonl (default "text") |
| `--iface` | string | network interface to capture on (required) |
| `--layout` | string | decode samples with this layout: 9-2LE or a list such as FLOAT32,Quality (default: hex) |
| `--sample-every` | int | print every Nth matching ASDU (default 1) |
| `--scl` | string | SCL file: with --svcb, filter on and decode that control block's stream |
| `--sv-id` | string | only ASDUs with this svID |
| `--svcb` | string | control block in --scl: name, LD/name, IED/LD/name or SvCBRef |

### `iec61850ctl sv subscribe`

Subscribe to one SVCB from SCL with validation, supervision and live statistics.

| Flag | Type | Meaning |
|---|---|---|
| `--accept-simulation` | bool | accept frames with the simulation bit set |
| `--bare-domain` | bool | expect datSet with the bare LD as MMS domain (LD/LLN0$ds) instead of IED+LD |
| `--count` | int | stop after this many accepted ASDUs (0 = unlimited) |
| `--duration` | duration | stop after this long (0 = until Ctrl+C); with --count, exit 4 if not reached |
| `--format` | string | Output format: text, jsonl (default "text") |
| `--iface` | string | network interface to receive on (required) |
| `--sample-every` | int | print every Nth accepted ASDU (0 = none) |
| `--scl` | string | SCL file describing the publisher (required) |
| `--stats-interval` | duration | print stream statistics this often (0 = only at the end) (default 1s) |
| `--svcb` | string | control block: name, LD/name, IED/LD/name or SvCBRef (required) |
| `--timeout` | duration | supervision timeout: a silent stream reports a timeout after this long (default 100ms) |

### `iec61850ctl sv svcb get`

Read an MSVCB (SvEna, MsvID, DatSet, ConfRev, SmpRate, OptFlds, SmpMod, DstAddress, noASDU).

| Flag | Type | Meaning |
|---|---|---|
| `--format` | string | Output format: text, json (default "text") |
| `--ld` | string | logical device (required) |
| `--svcb` | string | MSVCB name (MSVCB01) or item (LLN0$MS$MSVCB01) (required) |

### `iec61850ctl sv svcb set`

Write MSVCB attributes (SvEna written last).

| Flag | Type | Meaning |
|---|---|---|
| `--dat-set` | string | new DatSet reference (only while disabled) |
| `--disable` | bool | set SvEna=false |
| `--enable` | bool | set SvEna=true |
| `--format` | string | Output format: text, json (default "text") |
| `--ld` | string | logical device (required) |
| `--msv-id` | string | new MsvID (only while disabled) |
| `--opt-flds` | string | new OptFlds: comma list of refresh-time, sample-synchronized, sample-rate, data-set, security, or none (only while disabled) |
| `--smp-mod` | string | new SmpMod: SmpPerPeriod, SmpPerSec, SecPerSmp (only while disabled) |
| `--smp-rate` | uint16 | new SmpRate (only while disabled) |
| `--svcb` | string | MSVCB name (MSVCB01) or item (LLN0$MS$MSVCB01) (required) |
| `--yes` | bool | confirm that the MSVCB write may change device state (required) |

### `iec61850ctl tree`

Display the complete IEC 61850 device tree.

| Flag | Type | Meaning |
|---|---|---|
| `--flatten` | bool | display output in flat format (LD/LN.DO.DA[FC]: value [type]) |
| `--include` | string | comma-separated: data_sets, report_control_blocks, or all (only with --serialize) |
| `--interval` | string | delay between each MMS call (e.g. 100ms, 1s); 0 = no delay (default "0") |
| `--output` | string | write serialized JSON to file (default: stdout) |
| `--path` | string | limit traversal to specific path (e.g., MNSREF615LD0/FMMXU1 or MNSREF615LD0/FMMXU1.Hz) |
| `--serialize` | bool | output JSON for server consumption (see docs/SERVER.md); takes precedence over --flatten |

### `iec61850ctl version`

Display version information.

No flags of its own.
