# Munin plugin on the hub host — proposal

*Status: proposal, not reviewed. The decision point is in the [POC plan](../poc.md),
stage 3: decide before any charting is built by hand.*

## Proposal

Serve metric history graphs through munin instead of building charting into the web page,
for as long as the product has no drill-down of its own. One munin plugin on the hub's
host reads the hub's SQLite database and reports every metric of every node; munin's RRD
storage then draws day/week/month/year graphs for free.

## Why this fits

- The project's declared value is the normalization and prioritization layer, not storage
  or charting ([2026-08-28 note](2026-08-28-concept.md)). Graphs are exactly the layer
  worth taking off the shelf.
- The concept promises "depth and history" ([concept.md](../concept.md)), but the
  product's own drill-down and time travel arrive only with the semantic engine (roadmap
  stage 3). Munin bridges that gap for roughly an evening of work.
- The push architecture stays intact: munin polls only the hub's own host, never the
  nodes, so [ADR 0002](../decisions/0002-push-not-pull.md) is untouched.

## Constraints

- Munin's own warning/critical thresholds stay disabled: meaning lives in the semantic
  core alone ([ADR 0001](../decisions/0001-semantic-core-and-skins.md)), and a second
  source of "good/bad" would contradict it.
- The plugin itself is generic tooling and may ship in this repository; anything in its
  configuration that reveals installation facts belongs with the private configuration,
  per [ADR 0007](../decisions/0007-public-repository.md).

## Rejected along the way

- **munin-node on every node** — reintroduces pull to laptops behind NAT, the exact
  problem [ADR 0002](../decisions/0002-push-not-pull.md) exists to avoid.
- **The munin plugin protocol as the external-sensor format** — an executable printing
  `key.value` lines is almost exactly the external sensor of
  [ADR 0003](../decisions/0003-sensors-are-modules.md), and adopting the protocol would
  unlock the existing ecosystem of munin plugins as sensors. Deferred, not refused:
  ADR 0003 itself postpones external sensors until one is actually needed. Worth
  revisiting then.
