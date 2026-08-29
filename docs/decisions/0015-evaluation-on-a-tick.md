# 0015. Evaluation runs on its own tick, never inside a request

- **Status:** accepted
- **Date:** 2026-08-29
- **Source:** [POC](../poc.md) stage 2, [evaluation spec](../specs/evaluation.md)

## Context

Stage 2 adds levels, an event log and alerts. Two places could compute them: the ingest
handler, right after it stores what an agent sent, or a scheduler on the hub.

Node silence settles half of it on its own. A silent node sends nothing, so the only thing
that can notice it is a clock on the hub. The daily digest and the once-a-day repeat of an
unresolved critical are the same kind of work. A periodic pass therefore exists whatever is
decided about measurements.

## Decision

**One tick evaluates everything.** Every minute — compiled in, with no key to change it — the hub
reads the latest stored values, computes the level of every subject, writes transitions to
the event log and hands them to the notifier. Silence, the digest and the repeat rule are
the same pass. `/api/v1/ingest` keeps doing what its spec says: validating shape, storing,
and answering with the agent's configuration.

## Consequences

- A transition is visible at most one tick late. Against collection intervals of 15m and 1h
  this is not measurable.
- There is exactly one writer of the event log, so no transition can be produced twice or
  race with itself.
- Notification delivery is off the agent's request path: a Telegram outage cannot turn into
  a 500 for an agent, and a slow send cannot hold an ingest connection open.
- A threshold edit takes effect on the next tick even for a node that is not reporting.
- The hub gains a background loop that `cmd/hub` must start and shut down cleanly.

## Alternatives

- **Evaluate synchronously in the ingest handler, tick only for silence** — rejected: two
  writers of one event log, alerting inside the agent's request, and thresholds that only
  apply when a node happens to report.
- **Ingest enqueues the node, a worker evaluates** — rejected as premature: it buys latency
  that the project's intervals cannot notice, at the price of a queue, deduplication and
  shutdown ordering.
