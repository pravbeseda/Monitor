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

Each condition of a compound rule ([0012](0012-threshold-model.md)) carries its own margin;
the state leaves a severity only when every condition that put it there has cleared.

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
