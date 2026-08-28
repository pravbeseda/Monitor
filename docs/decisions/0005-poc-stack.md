# 0005. POC stack: Go, SQLite, server-side HTML

- **Status:** proposed (open question 1 in the [POC](../poc.md) — the language is not confirmed)
- **Date:** 2026-08-28
- **Source:** [POC](../poc.md)

## Context

The agent must install on macOS and Debian (Windows later) with no runtime and no package
dependencies. The hub serves a handful of nodes at minute intervals.

## Decision (pending confirmation)

- **Go** for both binaries: a single static binary, cross-compilation with one command, and
  `gopsutil` covering disks, CPU and memory on every target OS.
- **SQLite** for storage (one `measurements` table, WAL) behind a `Storage` interface.
- **Server-side HTML** (`html/template`), one page, no SPA and no frontend build.
- **HTTPS + JSON**, version in the path from the first commit (`/api/v1/ingest`).
- **Deployment**: systemd on Debian, launchd on macOS, Caddy for automatic TLS.

## Consequences

- Moving to Postgres or VictoriaMetrics is a new `Storage` implementation, not a hub
  rewrite. The interface must exist from the first commit.
- SPA and skins come after the POC, once the State API exists; the POC page is deliberately
  disposable.

## Alternatives

- **Rust** — more time for the same result at this scale.
- **Python** — painful to distribute an agent to several machines.
- The language stays open until explicitly confirmed: this is a personal project, and
  motivation outranks optimality.
