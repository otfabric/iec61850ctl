# IEC 61850 data model (for iec61850ctl)

This note summarizes the IEC 61850 object model as used by **iec61850ctl** over MMS (IEC 61850-8-1). For CLI usage see [../README.md](../README.md).

## Hierarchy

```
IED (Intelligent Electronic Device)
└── Logical Device (LD) — e.g. "MNSREF615LD0"
    │
    ├── Logical Node (LN) — e.g. "FMMXU1" (frequency measurement)
    │   ├── Data Object (DO) — e.g. "Hz"
    │   │   └── Data Attribute (DA) — e.g. "mag.f"
    │   │       └── Value: 50.0
    │   │
    │   └── Data Object (DO) — e.g. "Mod"
    │       └── Data Attribute (DA) — e.g. "stVal"
    │
    ├── Logical Node (LN) — "LLN0" (device-level)
    │   ├── Data Set (DS) — e.g. "MeasFlt"
    │   ├── Report Control Block (RCB) — URCB (RP) or BRCB (BR)
    │   ├── Log Control Block (LCB)
    │   └── Log / Journal — e.g. "EventLog"
    │
    └── Files (MMS file service)
```

| Concept | Role |
|---------|------|
| **IED** | Physical device (protection relay, bay controller, …) |
| **LD** | Logical device; in MMS mapping this is typically the domain name |
| **LN** | Functional grouping (e.g. `MMXU`, `GGIO`, `LLN0`) |
| **DO** | Contained data (measurement, status, setting) |
| **DA** | Leaf or structured attribute (`mag.f`, `q`, `t`, …) |
| **FC** | Functional constraint — scopes how a DA is accessed (see below) |
| **Data set** | Named list of member references, often used by reports |
| **RCB** | Report control block — buffered (`BR`) or unbuffered (`RP`) |
| **Journal** | Persistent log read via MMS journal services |

## Object references

iec61850ctl uses IEC 61850-style references:

```
LogicalDevice/LogicalNode.DataObject[.attr…]
```

Examples:

| Reference | Meaning |
|-----------|---------|
| `MNSREF615LD0/FMMXU1.Hz.mag.f` | Frequency magnitude (float) |
| `MNSREF615LD0/FMMXU1.Hz.q` | Quality |
| `MNSREF615LD0/FMMXU1.Hz.t` | Timestamp |

When reading with `get object`, pass `--fc` (default `MX`) so the client can form the MMS item path. Some UIs show FC in brackets: `…/FMMXU1.Hz.mag.f[MX]`.

Annotated path:

```
MNSREF615LD0/FMMXU1.Hz.mag.f
│           │ │      │  │
│           │ │      │  └─ attribute (float)
│           │ │      └──── magnitude
│           │ └─────────── data object (frequency)
│           └───────────── logical node
└───────────────────────── logical device (MMS domain)
```

## Functional constraints (FC)

Common FCs used with `get object --fc` and shown by `list das`:

| FC | Purpose |
|----|---------|
| **ST** | Status information |
| **MX** | Measurands (analogue values) |
| **SP** | Setpoints |
| **SV** | Substitution |
| **CF** | Configuration |
| **DC** | Description |
| **SG** / **SE** | Setting groups |
| **CO** | Control |
| **RP** | Unbuffered report control |
| **BR** | Buffered report control |
| **LG** | Log control |

Reporting and control blocks are not ordinary process data: list them with `list reports` / `get report`, and subscribe with `subscribe report --type BR|RP`.

Controllable objects use FC **CO** under the hood, but the CLI must not bypass the control model with `set object --fc CO`. Use:

- `control inspect` — read `ctlModel` / controllability
- `control operate` — atomic select/operate on one association (`--mode auto` by default)

Scalar settings (e.g. ING `setVal`) use `set object --fc SP` (or other non-CO FCs).

## Reports and journals

- **URCB** (`--type RP`): unbuffered reports; typically require reservation before enable.
- **BRCB** (`--type BR`): buffered reports; may include entry ID / buffer overflow fields. Pre-enable: `--purge-buf`, `--entry-id HEX`, `--resv-tms`.
- **GI** (`--interrogation` on subscribe): general interrogation — requests a snapshot report (may be empty).
- **Journals**: list with `list journals --ld …`, read with `get journal` (time range or start-after).

## SCL vs online model

| Source | Commands |
|--------|----------|
| Online IED (MMS) | `list`, `get`, `set`, `control`, `tree`, `subscribe`, `find`, `http` |
| Offline SCL (CID/ICD/SCD) | `scl parse`, `scl convert`, `server start --scl` |

The SCL file defines the engineering model. The live MMS server may expose a subset or different naming; always verify with `list` / `tree` against the device.

## Further reading

- Server simulation and value seeding: [SERVER.md](SERVER.md)
- Automation contract: [AUTOMATION.md](AUTOMATION.md)
- Interoperability: [INTEROP.md](INTEROP.md)
- Stack libraries: [go-iec61850](https://github.com/otfabric/go-iec61850), [go-mms](https://github.com/otfabric/go-mms)
