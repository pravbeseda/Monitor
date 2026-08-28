# POC — Free Disk Space

## Goal

See one end-to-end scenario working on real data and fix the architectural skeleton that
everything later grows from without a rewrite:

**agent on a node → ingest on the hub → storage → web page → Telegram alert.**

Scope: laptop-class nodes (macOS) and server-class nodes (Debian). Windows comes later,
but cross-platform support is a stack requirement from day one.

Metrics: free space (bytes and percent) per mounted volume per node, plus a `heartbeat`
metric that proves the agent is alive.

## Terminology

- **Agent** — the single application on a node that collects and sends measurements.
- **Sensor** — a module inside the agent that takes one kind of reading (disk, cpu,
  battery). Chosen over "driver": an agent has a set of sensors, like a robot.
- **Hub** — the server application: ingest, storage, evaluation, API, web.
- **Notifier** — a delivery channel for notifications (Telegram in the POC).

## Decisions this POC builds on

The reasoning and the rejected alternatives are in the ADRs; the POC only applies them.

- [0002](decisions/0002-push-not-pull.md) — agents push over HTTPS; node silence is an
  expected state, configured per node class.
- [0003](decisions/0003-sensors-are-modules.md) — sensors are in-process modules.
- [0004](decisions/0004-two-binaries-monorepo.md) — two build artifacts: `agent` and `hub`.
- [0005](decisions/0005-poc-stack.md) — Go, SQLite, server-side HTML *(proposed)*.
- [0006](decisions/0006-alerting-rules.md) — alert on transitions, critical only instantly.
- [0007](decisions/0007-public-repository.md) — no node names, thresholds or secrets in
  this repository.

## Wire format

The ingest payload is the seed of the general measurement schema, so it is versioned from
the first commit.

```json
POST /api/v1/ingest
{
  "node": "laptop-a",
  "agent_version": "0.1.0",
  "ts": "2026-08-28T10:00:00Z",
  "measurements": [
    { "metric": "disk.free_bytes", "labels": {"mount": "/", "fs": "apfs"}, "value": 123456789 },
    { "metric": "disk.free_pct",   "labels": {"mount": "/"}, "value": 34.2 }
  ]
}
```

Thresholds and node classes are declared in the hub's YAML configuration, never in code.
The real file lives on the server and is not part of this repository; the repository ships
only an example:

```yaml
metrics:
  disk.free_pct:
    direction: higher_is_better
    warn_below: 15
    crit_below: 7
nodes:
  laptop-a: { class: laptop, silence_after: 48h }
  server-b: { class: server, silence_after: 10m }
```

## Work plan

**Stage 1 — skeleton (one end-to-end thread)**
- [ ] Monorepo: `cmd/agent`, `cmd/hub`, `internal/...`
- [ ] Agent: disk sensor, configuration (url, token, interval), push loop
- [ ] Hub: `/api/v1/ingest`, write to SQLite, page `/` with the latest values
- [ ] Run by hand on one laptop and one server

**Stage 2 — evaluation and alerts**
- [ ] YAML config for metrics and nodes; threshold engine with hysteresis
- [ ] State and transition tables (event log)
- [ ] Telegram notifier and silence detector

**Stage 3 — operation**
- [ ] systemd and launchd units, agent install script
- [ ] Caddy with TLS, per-node tokens, authentication on the web page
- [ ] Roll out to every node, observe for a week, tune thresholds

**Done when**: filling a disk on a test node produces a Telegram alert within one interval,
the node turns red on the web page, and the value history is visible for the whole
observation period.

## Open questions

1. Language: is Go agreed, or is there a stack that is simply more pleasant to write in?
   (This is a personal project — motivation outranks optimality.)
2. Where does the hub live, and does that host have a domain or a static address?
3. Polling interval: 5 minutes for servers, 15 for laptops?
4. Thresholds: 15% / 7% free to start? Separate thresholds for backup volumes, which are
   always nearly full by design?
5. Which volumes are excluded (APFS snapshots, `/boot/efi`, tmpfs, external drives)?
6. Telegram: a private chat or a channel? A single recipient?
7. Retention of raw points — keep everything? At minute intervals that is megabytes a year.
