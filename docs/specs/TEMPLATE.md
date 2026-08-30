# Spec: <subsystem>

- **Status:** draft | approved (reviewed and in force) | superseded
- **Owns:** the packages this spec governs, e.g. `internal/eval`
- **Decisions:** ADRs this behaviour follows from

## Purpose

One paragraph: what this subsystem is responsible for, and what it explicitly is not.

## Behaviour

The table is the contract. One row = one test. Keep rows observable: an input the caller
can produce, an outcome the caller can see. A row that names a column, a package, a
goroutine, a timeout or a database flag is a test, not a spec
([0017](../decisions/0017-one-spec-and-decision-gates.md)); such constraints belong in the
checklist and in the tests.

| State | Event | New state | Side effect |
|---|---|---|---|
| ok | free_pct 14 | warning | queued into digest |
| warning | free_pct 6 | critical | notify immediately |
| critical | free_pct 8 | critical | none (hysteresis: 7% clears at 8.4%) |
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

Questions of the three kinds in
[0017](../decisions/0017-one-spec-and-decision-gates.md); everything else the author decides
and records. Empty in a spec that is in force.
