# Interoperability

Black-box CLI interoperability for `iec61850ctl`. This complements the protocol-depth matrix in [go-iec61850/INTEROP.md](https://github.com/otfabric/go-iec61850/blob/main/INTEROP.md).

## Ownership

| Repository | Owns |
|------------|------|
| [iec61850-interop](https://github.com/otfabric/iec61850-interop) | Adapter containers, fixtures, readiness/capability contracts |
| [go-iec61850](https://github.com/otfabric/go-iec61850) | Exhaustive Go API interoperability matrix |
| **iec61850ctl** | Operator journeys through the built CLI binary (`e2e/`) |

Assertions live in consumer repos. Adapters and fixtures never embed pass/fail criteria.

The `e2e` package treats `iec61850ctl` as an **external executable**. It does not import `cmd`, `internal`, or `pkg`.

## Matrices

### Client direction (independent interop)

Every iec61850-interop stack is a peer: [libiec61850](https://libiec61850.com), [IEC61850bean](https://www.beanit.com/iec-61850/), [dscsystems/go-iec61850](https://github.com/dscsystems/go-iec61850) and [iec61850-rust](https://github.com/csp0924/iec61850-rust). The harness keeps them in one registry (`adapterStacks` in `e2e/harness_test.go`), so each journey runs against all four.

| Client | Server | CI job |
|--------|--------|--------|
| `iec61850ctl` | libiec61850-ied-server | `e2e / libiec61850` |
| `iec61850ctl` | iec61850bean-ied-server | `e2e / iec61850bean` |
| `iec61850ctl` | dscsystems-ied-server | `e2e / dscsystems` |
| `iec61850ctl` | iec61850-rust-ied-server | `e2e / iec61850-rust` |

Journeys include association/browse, ST/MX reads against `expected-values.json`, datasets, RCB discovery/config, URCB GI subscribe (`--format jsonl`), atomic control operate (SPCSO1/2/3; Bean SBOw = expected failure), scalar write to `GGIO1.SetInt1.setVal[SP]`, BRCB purge/resume (iec61850-rust = expected failure), and negative control/write cases.

### Reverse server direction (Phase 3A + 3B)

| External adapter | Server | CI job |
|------------------|--------|--------|
| `<stack>-ied-controller` / `-ied-reporter` | `iec61850ctl server start` | `e2e / server-<stack>` |
| `<stack>-ied-client` | `iec61850ctl server start` | `e2e / server-reader-<stack>` |

`<stack>` is each of `libiec61850`, `iec61850bean`, `dscsystems` and `iec61850-rust`: eight reverse jobs.

**Phase 3A** claims:

- external **control** interoperability (direct, SBO, SBOw against SPCSO1/2/3);
- external **URCB** interoperability (enable → write → report → disable → conclude).

**Phase 3B** claims general external browse/read/write/dataset via capability-declared `*-ied-client` adapters:

- association and conclude;
- LD / LN / DO discovery;
- ST, MX, CF, DC reads against seeded values;
- ST write (`Mod.stVal=5`) and dataset read reflecting the write.

Requires iec61850-interop adapters that declare the `*-ied-client` commands in `fixtures/capabilities.json`. The lock file pins the [iec61850-interop `v1.0.1`](https://github.com/otfabric/iec61850-interop/releases/tag/v1.0.1) image digests (IED fixture `iec61850-v3`); local `:local` images are fine for development.

### GOOSE (raw Ethernet)

| Publisher | Subscriber | CI job |
|-----------|------------|--------|
| `<stack>-goose-publisher` | `iec61850ctl goose subscribe` and `goose sniff` | `e2e / goose-<stack>` |
| `iec61850ctl goose publish` | `<stack>-goose-subscriber` | `e2e / goose-<stack>` |

`<stack>` is `libiec61850`, `dscsystems` or `iec61850-rust` (IEC61850bean has no GOOSE adapter). The fixture is iec61850-interop `goose-v1` (`interop-goose.cid` and `goose.json`, control block `gcbAll`: eight members including a float32, bit strings, a UTC time and a nested structure).

GOOSE has no IP, so each journey creates its own Docker bridge network and runs `iec61850ctl` there too: a static Linux build (built by the test for the Docker server's architecture, or `IEC61850CTL_LINUX_BIN`), mounted into a container with `NET_RAW`. This works on Linux CI and on Docker Desktop, where the bridge lives in the VM.

| Journey | Asserted |
|---------|----------|
| peer → `goose subscribe` | frames of both states (stNum 1 = `values`, stNum 2 = `update`) with every value, gocbRef/datSet/goID/APPID/confRev, increasing sqNum, no `invalid` event, a supervision `timeout` after the publisher stops |
| peer → `goose sniff` | frames decoded and filtered on APPID |
| `goose publish` → peer | both states, `test=true`, the control block fields, the seven settable `update` values, sqNum from 0 and increasing, a supervision `timeout` after the run |

### Sampled Values (raw Ethernet)

| Publisher | Subscriber | CI job |
|-----------|------------|--------|
| `<stack>-sv-publisher` | `iec61850ctl sv subscribe` and `sv sniff` | `e2e / sv-<stack>` |
| `iec61850ctl sv publish` | `<stack>-sv-subscriber` | `e2e / sv-<stack>` |

`<stack>` is `libiec61850`, `dscsystems` or `iec61850-rust` (IEC61850bean has no SV support). The fixture is iec61850-interop `sv-v1` (`interop-sv.cid` and `sv.json`). Every journey runs for both streams: `MSVCB01`, the 9-2LE profile (4000 samples/s, one ASDU per frame, no optional fields), and `MSVCB02`, a generic dataset (1000 samples/s, four ASDUs per frame, datSet/refrTm/smpRate/smpMod sent). The network setup is the GOOSE one.

| Journey | Asserted |
|---------|----------|
| peer → `sv subscribe` | sampled ASDUs with svID/APPID/destination/confRev/noASDU, exactly the optional fields the stream sends, smpCnt within the wrap, the fixture `values` and then the `update`; no `invalid` event; at most 1 % lost and no duplicates; at least 90 % of the published samples received; the smpCnt wrap (4000 / 1000); a supervision `timeout` after the publisher stops |
| peer → `sv sniff` | ASDUs decoded with the SCL layout, matching one of the two value sets |
| `sv publish` → peer | the header with the simulation bit, the fixture values (qualities as 13-bit strings), the optional fields; at most 1 % lost beyond the samples the publisher reports as `skipped`; at least 90 % of the published samples received |
| `sv svcb get` (forward stacks) | `MSVCB02` of the IED fixture: MsvID, DatSet, ConfRev, SmpRate, SmpMod, OptFlds, DstAddress, noASDU, for the stacks whose IED server models it (lock `svcb_over_mms`); a failed read for the others |

Registered limitations (iec61850-interop `capabilities.json`), asserted rather than skipped:
- **libiec61850 subscriber, two-octet smpMod.** libiec61850's SVReceiver accepts smpMod only as one octet. go-iec61850 sends two, as libiec61850's own publisher does, so `sv publish` of `MSVCB02` → `libiec61850-sv-subscriber` must receive nothing.
- **SVCB over MMS.** iec61850-rust and IEC61850bean IED servers do not expose `LLN0$MS$MSVCB02`; `sv svcb get` against them must fail.

### Phase 4 — Control and scalar write

| Journey | libiec61850 | iec61850bean |
|---------|-------------|--------------|
| SPCSO1 direct + `--confirm-ref` → `confirmed` | required | required |
| SPCSO2 SBO + confirm | required | required |
| SPCSO3 SBOw + one `ctl_num` + confirm | required | **expected failure** (registered limitation) |
| `set object` `SetInt1.setVal[SP]` + `--verify` | required | required |

dscsystems and iec61850-rust: all four required.

Mutating journeys use **isolated** adapter servers (no `t.Parallel()`). Bean SBOw asserts non-zero exit and unchanged `stVal` (not a skip).

### Phase 5 — External BRCB

`get report` exposes BRCB `purge_buf` / `entry_id_value` / `resv_tms` when present. `subscribe report --type BR` supports `--purge-buf`, `--entry-id`, `--resv-tms` written **before** enable. E2E: purge+GI then resume with EntryID (libiec61850 and dscsystems required; Bean EntryID best-effort; iec61850-rust expected failure, see below).

### Phase 6 — Hardening

Unit: connection fault after select → Cancel cleanup; concurrent direct operate; short endurance loop.  
E2E: mode mismatch, confirmation mismatch, `FC=CO` reject. Missing reverse capability commands fail setup (see reverse harness). Full adapter fault-injection modes for mid-sequence disconnect remain available as a follow-up when adapters expose them.

### Self-smoke (not independent interop)

| Client | Server | CI job |
|--------|--------|--------|
| `iec61850ctl` | `iec61850ctl server start` | `e2e / self` |

Useful for release/example confidence. Same stack on both ends — not an independent interoperability claim.

## Fixture and image provenance

Checked in under `e2e/testdata/`:

| File | Role |
|------|------|
| `interop.icd` | SCL model (self-server + documentation) |
| `expected-values.json` | Expected values for **external** adapter assertions (copy of iec61850-interop `values.json`) |
| `self-server-values.json` | Serialized `domain.IED` seed for `server start --values` |
| `interop-goose.cid`, `goose.json` | GOOSE fixture `goose-v1` (control blocks, members, value sets) |
| `interop-sv.cid`, `sv.json` | SV fixture `sv-v1` (streams, members, value sets); `interop-sv.cid` is also `pkg/svops/testdata` |
| `interop.lock.json` | `source_commit`, optional `source_tag`, fixture revisions, file hashes, image digests per stack, required commands, `svcb_over_mms` per stack |
| `examples.json` | Declarative examples → `docs/interop-examples.md` |

Never pass `expected-values.json` to `server start --values`.

```sh
make e2e-verify-fixtures
make e2e-update-fixtures IEC61850_INTEROP_DIR=../iec61850-interop
```

Local images default to `iec61850-interop-*:local`. CI uses digest pins from the lock file.

### Client IED name

| Stack | Client `IEC61850_IED_NAME` |
|-------|----------------------------|
| `libiec61850`, `iec61850bean`, `dscsystems`, `iec61850-rust` | `InteropIED` (required for domain normalisation) |
| `self`, `server-*` (CLI as server) | **unset** for CLI clients talking to the Go server |

## Local execution

```sh
make build-nocheck
make e2e-self                                          # self-smoke only
IEC61850CTL_E2E_STACKS=dscsystems make e2e            # client → one stack
IEC61850CTL_E2E_STACKS=server-iec61850-rust make e2e  # reverse control/report
IEC61850CTL_E2E_STACKS=server-reader-dscsystems make e2e
IEC61850CTL_E2E_STACKS=goose-libiec61850 make e2e     # GOOSE both directions
IEC61850CTL_E2E_STACKS=sv-dscsystems make e2e         # Sampled Values both directions, both streams
make e2e                                              # default: the four client stacks + self
IEC61850CTL_E2E_STACKS=all make e2e                   # all 19 CI jobs' stacks
```

Images come from `<STACK>_IMAGE` (`LIBIEC61850_IMAGE`, `IEC61850BEAN_IMAGE`, `DSCSYSTEMS_IMAGE`, `IEC61850RUST_IMAGE`), else the lock digest when `CI` is set, else `iec61850-interop-<stack>:local`.

CI runs each stack as a separate matrix job (`fail-fast: false`). Requires Docker for external/reverse stacks. `IEC61850CTL_BIN` is set by the Makefile.

## Known limitations / deferred

- **iec61850-rust server, BRCB.** A read of BRCB `TimeOfEntry` is answered object-access-unsupported, and so is a read of the whole BRCB. `get report` and `subscribe report --type BR` on a BRCB therefore fail with exit `6`. The e2e asserts that deterministic failure, naming `ObjectAccessUnsupported` (`adapterStack.brcbUnreadable`). Its URCBs work, since the client reserves before enabling.
- **dscsystems server, unknown LD.** GetNameList for an unknown domain returns an empty list rather than an error. `list lns` detects an empty LD missing from the server directory and exits `11`, the same as against the other stacks.
- **Peer limitations upstream.** iec61850-interop `fixtures/capabilities.json` lists the rest, such as ST writes being denied by every adapter server and dscsystems describing Enum as Integer. None of them affects these journeys.
- IEC61850bean SBOw as a **server** (forward `control operate` on SPCSO3) is a registered expected failure; when Bean gains support, replace the failure assertion (do not leave a silent skip). Phase 3A still exercises Bean as a **client** against the Go CLI server.
- Complex JSON encodings (timestamps, bit/octet strings, arrays, structures) are deferred; see [AUTOMATION.md](AUTOMATION.md).
- Adapter fault-injection modes (e.g. mid-sequence disconnect, `--skip-select`) are not yet exposed; negative coverage today is CLI/unit + mode/confirmation/CO reject e2e.
- File/journal reverse coverage and dynamic datasets are out of scope.
- HTTP mutation (control/write over the REST API) is out of scope.
