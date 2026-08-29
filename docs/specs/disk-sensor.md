# Spec: Disk sensor

- **Status:** approved
- **Owns:** `internal/sensor/disk` (agent): mount enumeration, filtering, and the label
  contract of `disk.free_bytes` and `disk.free_pct`
- **Decisions:** [0003](../decisions/0003-sensors-are-modules.md),
  [0010](../decisions/0010-agent-configuration.md),
  [0012](../decisions/0012-threshold-model.md)

## Purpose

The disk sensor reads how much space each mounted volume has left and returns it as
measurements. It decides nothing: which filesystem types count comes from the hub's
configuration, and whether a value is alarming is the evaluation engine's business.

Its labels are a contract. They form the storage key together with the metric id, so a
volume that changes its labels between runs breaks its own history.

## Measurements

Every collected volume yields both metrics of [0012](../decisions/0012-threshold-model.md),
which needs an absolute and a proportional reading of the same volume:

| Metric | Value |
|---|---|
| `disk.free_bytes` | the space the system reports as available for important use |
| `disk.free_pct` | 100 × that value ÷ total size, rounded to two decimals |

Available, not free: the blocks a filesystem reserves for root are not space the machine
can use, and reporting them would delay every alert by the size of the reserve.

What "available" means is the operating system's answer, not a system call chosen once
([0014](../decisions/0014-macos-available-space.md)): on Linux the blocks `statfs` leaves
to an unprivileged user, on macOS `kCFURLVolumeAvailableCapacityForImportantUsageKey`,
which counts purgeable space — the local snapshots and caches macOS deletes by itself
when a volume fills. On a Mac the two differ by tens of gigabytes.

Both metrics carry the same labels, so one volume is one series in two units:

| Label | Value |
|---|---|
| `mount` | the mount point exactly as the OS reports it |
| `fs` | the filesystem type, lower case (`apfs`, `ext4`) |
| `removable` | `"true"` on a volume the OS reports as removable, `"false"` otherwise |

Removable is what the OS says it is — `MNT_REMOVABLE` on macOS,
`/sys/block/<device>/removable` on Linux — and `false` when that cannot be read. A mount
point under `/Volumes` or `/mnt` proves nothing: an internal disk mounted there would
carry a wrong label for the life of its history, because the label is part of the key.

## Behaviour

One row = one test. Anchors: `spec: disk-sensor.md#<heading>`.

### Enumeration

| Machine state | Collected |
|---|---|
| a mounted volume whose type is in the allow-list | `disk.free_bytes` and `disk.free_pct` for it |
| a volume whose type is not in the allow-list (`devfs`, `tmpfs`, `overlay`, `nfs`) | nothing: it is skipped |
| a volume reporting zero total blocks | nothing: a percentage of nothing is not a number |
| a volume that vanishes between enumeration and reading | nothing for it; every other volume is still collected |
| a volume the agent may not read | nothing for it; every other volume is still collected |
| macOS refuses the available-capacity question, or answers zero | the `statfs` value is reported instead: system volumes and backup targets answer zero by design |
| the mount table cannot be read at all | no measurements and an error the agent logs |
| no volume passes the allow-list | no measurements; the request itself still carries the heartbeat |

### Labels

| Machine state | Labels |
|---|---|
| an internal volume | `mount`, `fs`, `removable: "false"` |
| a volume the OS reports as removable | the same, with `removable: "true"` |
| the same volume on the next collection | byte-identical labels, so the series continues |

### Applicability

| Machine | Manifest entry |
|---|---|
| any supported platform | `disk`, applicable — every machine has at least one volume |

## Invariants

- One unreadable volume never costs the others: collection returns what it could read.
- Collection is read-only and cheap — one `statfs` per mount, no directory walking.
- The sensor holds no state between collections: the same machine and the same allow-list
  produce the same measurements.
- The allow-list arrives from the hub; the sensor has no built-in list of its own
  ([0010](../decisions/0010-agent-configuration.md)).

## Edge cases

- **Several volumes of one APFS container** (`/`, `/System/Volumes/Data`) report the same
  free space and move together. They are separate series; the evaluation engine sees
  correlated volumes, not a bug.
- **An unplugged external drive** simply stops producing measurements. `removable: "true"`
  is what lets the hub tell that from a vanished internal disk (stage 2).
- **A mount point with spaces or non-ASCII characters** is carried verbatim; labels are
  not sanitised, because the label is what identifies the volume.
- **Purgeable space on macOS** is counted as available, so a Mac reports what Finder
  shows rather than what `df` does. The panel and the operating system agree, and a
  threshold is not spent on space the system reclaims on its own.
- **A volume remounted at a different path** starts a new series: the mount point is part
  of the identity, and the alternative — matching by device — breaks when a disk is
  reformatted.

## Out of scope

- Thresholds, roles and health for a volume → evaluation spec (stage 2),
  [0012](../decisions/0012-threshold-model.md).
- Scheduling: when the sensor runs and how often → agent spec,
  [0010](../decisions/0010-agent-configuration.md).
- Inode exhaustion, SMART health, IO latency — later sensors, not this one.
- Windows volumes: the POC covers macOS and Debian.

## Open questions

None.
