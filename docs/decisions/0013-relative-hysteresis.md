# 0013. Hysteresis is a relative margin, not a fixed number of points

- **Status:** accepted
- **Date:** 2026-08-28
- **Supersedes:** the recovery margin in [0006](0006-alerting-rules.md); the rest of that
  decision stands

## Context

[0006](0006-alerting-rules.md) defines recovery as `warn_below + 3` percentage points. That
is only meaningful for a threshold expressed in percent. [0012](0012-threshold-model.md)
introduced absolute conditions — a floor in gigabytes — where "plus three points" says
nothing, and absolute headroom is exactly what flaps: logs rotate, caches grow and shrink,
and a volume can cross 10 GB several times an hour while its percentage never moves.

Future metrics make this worse, not better: steps, seconds, currency. A table of "unit →
margin" would have to grow with every metric.

## Decision

**A state returns to a lower severity only once the value clears the threshold by 20% of
that threshold.**

| Threshold | Fires below | Returns to ok at |
|---|---|---|
| 15% free | 15% | 18% |
| 10 GB free | 10 GB | 12 GB |
| 4 GB free | 4 GB | 4.8 GB |

The margin is a property of the threshold, not of the unit, so it applies unchanged to every
metric the system will ever carry. For percentage thresholds it reproduces what 0006
prescribed, so nothing about existing behaviour changes.

**Recovery is the negation of the entry rule, with every comparison shifted by the margin.**
A compound rule ([0012](0012-threshold-model.md)) is not cleared condition by condition: the
state ends when the rule itself stops holding, and negating a conjunction gives a
disjunction.

```
enters when   free < 10 GB   or  (free% < 15  and  free < 100 GB)
leaves when   free >= 12 GB  and (free% >= 18  or  free >= 120 GB)
```

Every comparison keeps its own 20% margin, so nothing can flap, and the disjunction is what
prevents latching:

| Volume | Enters warning at | Leaves at | Why |
|---|---|---|---|
| 128 GB | 19 GB free (14.8%) | 23 GB free | the ratio clears at 18% |
| 8 TB | 99 GB free (1.2%) | 120 GB free | the ceiling comparison clears first |
| any | below 10 GB free | 12 GB free | the floor clears |

Reading it as "every condition that put the state there must clear" is what latches a
volume: a 128 GB disk would have to reach 120 GB free to clear a 100 GB ceiling, which is an
almost empty disk.

## Consequences

- The metric schema needs no per-unit hysteresis setting.
- 20% is a product default and may be overridden per metric through the layering of
  [0010](0010-agent-configuration.md), for a metric that is inherently noisy.
- The evaluation engine computes recovery from the threshold rather than storing a second
  number beside it.

## Alternatives

- **A fixed margin per unit** (`+3` points, `+2 GB`) — rejected: it needs a new entry for
  every unit the project ever adds.
- **Time-based hysteresis** (recover only after N minutes below the threshold) — deferred:
  it is a different mechanism, orthogonal to this one, and deserves its own decision if
  value-based hysteresis proves insufficient.
