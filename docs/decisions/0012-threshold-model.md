# 0012. Thresholds fire on whichever comes first — percentage or absolute headroom

- **Status:** accepted
- **Date:** 2026-08-28
- **Source:** [POC](../poc.md) open question 4

## Context

Percentages misjudge both ends of the size range: 10% of 8 TB needs no attention, 10% of
128 GB is nearly full. Absolute limits fail the other way — 20 GB free is comfortable on a
laptop and alarming on a backup array. Backup and Time Machine volumes break both rules:
they sit nearly full by design and would alert forever.

## Decision

A disk threshold is a pair, and the state changes when **either** side is crossed:

| Level | Condition |
|---|---|
| warning | free below 15% **or** below 20 GB |
| critical | free below 7% **or** below 5 GB |

**Volume roles carry their own rule.** A volume declared `role: backup` drops percentages
entirely and keeps absolute headroom (warning below 50 GB, critical below 10 GB), because a
backup volume being full is its normal condition, not an incident.

**Overrides use the layering of [0010](0010-agent-configuration.md)** down to one volume:
sensor default → node class → node → single volume, keyed by node and mount point. A volume
the hub has not seen takes the defaults until it is given its own.

## Consequences

- The evaluation engine must support a disjunction of conditions per level; this is the
  shape every future threshold takes, not a special case for disks.
- Both `disk.free_pct` and `disk.free_bytes` are collected, as the wire format already has
  them — neither can be dropped as redundant.
- Hysteresis from [0006](0006-alerting-rules.md) applies per condition, so a volume sitting
  on either boundary cannot flap.
- Forecast-based alerting ("full in ~12 days") is a later addition on top of these
  thresholds, once history is long enough — not a replacement.

## Alternatives

- **Percentages only, with a manual exception per large volume** — rejected: it needs a hand
  written rule for nearly every volume, which is a patch, not a model.
- **Forecasts instead of thresholds** — deferred: no history exists at stage 2, and a short
  series forecasts badly.
