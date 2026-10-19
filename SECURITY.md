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

`control operate` and `set object` change the state of the IED they are pointed at, and `control operate` can operate switchgear and other primary equipment. `subscribe report` writes report control block settings, and with `--purge-buf` clears a buffered report's entries.

- **Do not point write or control commands at production equipment** unless you are authorized to operate it and the site's switching procedures allow it. There is no confirmation prompt
- Use `control inspect` and `control operate --dry-run` to see the planned sequence before operating
- `set object` rejects `FC=CO`, so it cannot be used to bypass the control model. It does not stop writes to settings (`SP`, `SE`, `CF` and others)
- `discover` opens TCP connections to every address in the given range. Scan only networks you are allowed to scan

### No transport security or authentication

- The client connects over plain ISO-on-TCP (port 102). There is no TLS (IEC 62351-3) and no ACSE or other authentication; anyone on the path can read or alter the traffic. Use it only on isolated, trusted networks, or through a VPN
- `server start` serves an MMS server without authentication, accepts reads, writes and controls from any client, and binds to `0.0.0.0` by default. Use `--host 127.0.0.1` unless other hosts must reach it
- `http` serves an unauthenticated, unencrypted HTTP API on `:8080` (all interfaces) by default. It exposes no write or control operation, but anyone who can reach it can browse and read the device behind it. Bind it to `127.0.0.1` with `--listen` or put an authenticating proxy in front
