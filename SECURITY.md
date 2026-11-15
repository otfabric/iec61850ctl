# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in `iec61850ctl`, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, please email **security@otfabric.io**, or open a private advisory on GitHub (the [Security](https://github.com/otfabric/iec61850ctl/security) tab → **Advisories** → **Report a vulnerability**), with:

- A description of the vulnerability
- Steps to reproduce the issue, ideally the exact command line and, where relevant, an SCL file or server that triggers it
- The affected version(s) (`iec61850ctl version`)
- Any potential impact you've identified

We will acknowledge your report within 48 hours and aim to provide a fix or mitigation within 7 days for critical issues.

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest release | Yes |
| Older releases | Best effort |

## Security Considerations

### The CLI acts on real equipment

`control operate` and `set object` change the state of the IED they are pointed at, and `control operate` can operate switchgear and other primary equipment. `subscribe report` writes report control block settings, and with `--purge-buf` clears a buffered report's entries. `goose gocb set` and `sv svcb set` enable, disable or reconfigure GOOSE and Sampled Values publishing on the IED.

`goose publish` and `sv publish` inject frames onto the station or process bus, where protection IEDs subscribe to them. A GOOSE trip or an SV stream with the wrong values can cause a protection function to operate.

- **Do not point write or control commands at production equipment** unless you are authorized to operate it and the site's switching procedures allow it. There is no interactive confirmation prompt; `control operate`, `set object`, `goose gocb set`, `sv svcb set` and `subscribe report --purge-buf` refuse to run without `--yes` (exit `2`, before connecting), so a script must confirm each state change explicitly
- `goose publish` and `sv publish` require an explicit `--iface` and `--yes` (refused with exit `13`, the organisation safety code, otherwise), and send test (GOOSE) or simulation (SV) frames unless you add both `--test=false`/`--simulation=false` and `--live`. Subscribers that honour those flags ignore the frames, but not every subscriber does: publish only on isolated test networks unless you intend to drive the subscribers
- Use `control inspect` and `control operate --dry-run` to see the planned sequence before operating
- `set object` rejects `FC=CO`, so it cannot be used to bypass the control model. It does not stop writes to settings (`SP`, `SE`, `CF` and others)
- `discover` opens TCP connections to every address in the given range. Scan only networks you are allowed to scan

### No transport security or authentication

- The client connects over plain ISO-on-TCP (port 102). There is no TLS (IEC 62351-3) and no ACSE or other authentication; anyone on the path can read or alter the traffic. Use it only on isolated, trusted networks, or through a VPN
- `server start` serves an MMS server without authentication, accepts reads, writes and controls from any client, and binds to `0.0.0.0` by default. Use `--host 127.0.0.1` unless other hosts must reach it
- GOOSE and Sampled Values have no authentication (no IEC 62351-6): any host on the LAN segment can publish frames a subscriber will accept, and `sniff` sees all traffic on the segment. Raw sockets need `CAP_NET_RAW` on Linux or `/dev/bpf*` access on macOS/BSD; grant the capability to the binary rather than running it as root
- `http` serves an unauthenticated, unencrypted HTTP API on `:8080` (all interfaces) by default. It exposes no write or control operation, but anyone who can reach it can browse and read the device behind it. Bind it to `127.0.0.1` with `--listen` or put an authenticating proxy in front

### Resource limits

Each input that comes from a file, a request or a device is bounded. The bounds lie far above any real SCL file, HTTP request or IED file. The tests listed in [TESTING.md](TESTING.md#budgets) check each bound, the size bounds with an allocation budget.

| Input | Bound | Beyond the bound |
|---|---|---|
| SCL file read by `scl parse` and `scl convert` | 1 GiB (1,073,741,824 bytes) | refused after reading 1 GiB + 1 byte: exit `3`, nothing written |
| Type references on one path of the expanded model (DOType, SDO DOType, DAType) | 64 | refused: exit `3`; `scl convert` and `scl parse --flatten` write nothing, `scl parse` ends its tree where the bound is reached |
| Expanded DO, SDO, DA and BDA instances of one `scl parse` or `scl convert` | 33,554,432, a path counting one more for each 256 bytes | refused as above |
| SCL files of `server start`, `goose` and `sv` | the same bounds, applied by go-iec61850 | refused when the file is loaded (see [docs/SERVER.md](docs/SERVER.md#limits)) |
| `http` request body (`/api/find/path`, `/api/find/bulk`) | 1 MiB (1,048,576 bytes) | `413`, code `invalid_input` |
| `http` `ln_pattern` | 256 bytes | `400`, code `invalid_input` |
| `http` mappings of one `/api/find/bulk` request | 10,000 | `400`, code `invalid_input` |
| File held in memory by `get file` | 256 MiB (268,435,456 bytes) | the download stops after 256 MiB, nothing is written to stdout or `--output`: exit `10` |
| Each MMS request, also each one inside `get file`, `list files`, `get journal` and `subscribe report` | `--timeout` (default 10s; without the flag 60s for `get file`, 30s for the other three) | the command stops: exit `5` |
| Cancel and LastApplError read after a failed select or operate | 2 s each | the step is reported as failed in `cleanup` / left out |
| `server start` MMS connections at once | `--max-connections` (default 5) | the connection is closed at once, before the association |
| `server start --values` UTC time milliseconds, time quality, bit string number | 0..65535, 0..255, 0..65535 | the file is refused: exit `3` |

### Text from devices and frames

Names, values and messages that come from a device, a GOOSE or SV frame or a file are written to the terminal as text. On stdout and stderr every C0 control character other than newline and tab, DEL and every C1 control character is written as `\u00XX`, and a byte that is not valid UTF-8 as `\xNN`. JSON output stays valid and decodes to the strings the device sent; YAML output escapes them itself, and CSV and table output carry the escapes as text. A newline or tab in a device's text is kept. `get file` writes the file's bytes to stdout unchanged, and the files a command writes with `--output` are not filtered.
