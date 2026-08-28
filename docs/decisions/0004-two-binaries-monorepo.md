# 0004. Two artifacts — agent and a monolithic hub — in one repository

- **Status:** accepted
- **Date:** 2026-08-28
- **Source:** [POC](../poc.md)

## Context

The hub combines ingest, storage, threshold evaluation, web and notification delivery. The
temptation is to split that into services immediately.

## Decision

Exactly two build artifacts from one monorepo:

1. **`cmd/agent`** — one binary per node (macOS arm64, Linux amd64; Windows later).
2. **`cmd/hub`** — a monolith: ingest + storage + evaluation + web + notifier.

Shared code lives in `internal/...`.

## Consequences

- One repository, one protocol version, one release cycle.
- Splitting into services stays possible later; at a scale of a few nodes it adds failure
  modes and deployment work and buys nothing.
- Components inside the hub are still separated by interfaces (`Storage`, notifier) so the
  cut can be made without a rewrite.

## Alternatives

Microservices from day one — rejected as disproportionate to the scale.
