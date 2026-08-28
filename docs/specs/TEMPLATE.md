# Spec: <subsystem>

- **Status:** draft | approved | superseded
- **Owns:** the packages this spec governs, e.g. `internal/eval`
- **Decisions:** ADRs this behaviour follows from

## Purpose

One paragraph: what this subsystem is responsible for, and what it explicitly is not.

## Behaviour

The table is the contract. One row = one test. Keep rows observable: an input the caller
can produce, an outcome the caller can see.

| State | Event | New state | Side effect |
|---|---|---|---|
| ok | free_pct 14 | warning | queued into digest |
| warning | free_pct 6 | critical | notify immediately |
| critical | free_pct 9 | critical | none (hysteresis) |
| critical | free_pct 18 | ok | notify recovery |

Anchor each group of rows with a heading so tests can cite it (`spec: thresholds.md#hysteresis`).

## Invariants

Statements that must hold after every operation, whatever the sequence of events.

- A transition is emitted at most once per state change.
- Restarting the hub does not re-emit a transition already delivered.

## Edge cases

The situations that are easy to leave undecided: missing measurements, restarts, clock
skew, configuration changed while a metric is in a non-ok state, first-ever measurement.

## Out of scope

What this subsystem deliberately does not handle, and which spec handles it instead.

## Open questions

Decisions the author cannot make alone. Empty before the spec is approved.
