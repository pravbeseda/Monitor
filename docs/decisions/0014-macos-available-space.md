# 0014. On macOS, free space is what the system considers available

- **Status:** accepted
- **Date:** 2026-08-29
- **Source:** [stage 1 log](../log/2026-08-29-stage-1-decisions.md), the first end-to-end run

## Context

macOS reports two different numbers for the same volume. `statfs`, `df` and `getattrlist`
agree on one of them; Finder shows a larger one, because it adds **purgeable** space —
local Time Machine snapshots and caches that the system deletes by itself when a volume
fills up. On the machine the first run was made on the gap was 72 GB out of 995: 50 GB by
`statfs`, 122 GB by Finder.

Alerting on the smaller number means alerting on space the system would have reclaimed on
its own, and every such alert is argued with by the operating system's own display. The
larger number is only reachable through `kCFURLVolumeAvailableCapacityForImportantUsageKey`
in CoreFoundation: `getattrlist` was tried and returns the `statfs` value.

## Decision

The disk sensor reports, as `disk.free_bytes`, **the space the operating system says is
available for important use**: on macOS the CoreFoundation key, on Linux the blocks
`statfs` reports as available to an unprivileged user. `disk.free_pct` is that value over
the volume's total size, so both metrics keep describing the same thing.

Reaching the CoreFoundation key requires **cgo, in the darwin file of the sensor only**.
Every other file, both binaries on Linux, and the SQLite driver stay free of it
([0005](0005-poc-stack.md)); the platform boundary is the `Source` interface that
[the disk sensor spec](../specs/disk-sensor.md) already draws.

## Consequences

- The macOS agent is built on macOS: cgo cannot be cross-compiled from Linux, and the
  toolchain now needs the Xcode command line tools.
- CI builds and tests on macOS as well as Linux, or the darwin file would never be
  compiled by any check ([0011](0011-quality-gates.md)).
- Should the CoreFoundation call fail — or answer zero, which is what macOS says about
  volumes that may not hold important data, from `/System/Volumes/*` to a Time Machine
  target — the sensor falls back to the `statfs` value rather than reporting nothing or a
  zero that would read as a full disk.
- The two platforms report the same *meaning* — space usable without deleting anything by
  hand — rather than the same system call.

## Alternatives

- **Keep the `statfs` number everywhere** — rejected: it is 72 GB pessimistic on a Mac with
  local snapshots, so thresholds would fire on space macOS frees by itself, and the panel
  would contradict Finder on the machine the operator is looking at.
- **Report both, adding `disk.purgeable_bytes` parsed from `tmutil` and `diskutil`** —
  rejected: parsing the output of two CLI tools on every collection is the most fragile of
  the three, and it puts a macOS-shaped concept into the metric schema of every platform.
