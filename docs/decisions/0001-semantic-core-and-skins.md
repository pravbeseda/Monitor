# 0001. Meaning lives in the core; skins are dumb renderers

- **Status:** accepted
- **Date:** 2026-08-28
- **Source:** [concept](../concept.md), [design notes](../log/2026-08-28-concept.md)

## Context

The requirement that shaped the whole architecture: several visual concepts (mission
control, city, organism, advisors, Telegram) must coexist and be switchable. If each
concept decides for itself whether a value is bad, the evaluation logic is duplicated
across views and drifts apart.

## Decision

The core computes semantic state once — health (0–100), status (ok / warning / critical),
trend, anomaly rank, freshness, forecasts. Skins receive that state through the State API
and only translate it into visuals.

A skin is a manifest (what it can display) + a mapping (metric → slot, defaulted from
metric metadata) + a renderer.

## Consequences

- The first skin is a plain debug table: it proves the State API is sufficient and remains
  the standard debugging mode.
- Shared services live outside skins: drill-down (`openMetric(id)`), time travel (`at=`),
  event stream (WebSocket/SSE).
- **Prohibited:** skin-specific fields in the State API contract ("smoke level", "building
  height"). Semantics only.
- Adding a skin requires no change to the core.

## Alternatives

Evaluation logic inside each view — rejected: duplicated rules and diverging verdicts
between skins.
