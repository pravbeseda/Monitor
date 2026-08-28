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
- [0005](decisions/0005-poc-stack.md) — Go, SQLite, server-side HTML.
- [0010](decisions/0010-agent-configuration.md) — the agent's configuration comes from the
  hub in the ingest response.
- [0011](decisions/0011-quality-gates.md) — `gofmt`, `go vet`, `golangci-lint` and
  `go test -race -cover` gate every change.
- [0012](decisions/0012-threshold-model.md) — thresholds fire on percentage or absolute
  headroom, whichever comes first.
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
- [ ] nginx vhost for the hub, per-node tokens, authentication on the web page
- [ ] Roll out to every node, observe for a week, tune thresholds

**Done when**: filling a disk on a test node produces a Telegram alert within one interval,
the node turns red on the web page, and the value history is visible for the whole
observation period.

## Open questions

1. Language: is Go agreed, or is there a stack that is simply more pleasant to write in?
   (This is a personal project — motivation outranks optimality.)
   **Decision:** Go for the agent and the hub, with quality enforced by tooling — `gofmt`,
   `go vet`, `golangci-lint` and `go test -race -cover` required in CI and in the pre-commit
   hook. Chosen over Node + TypeScript because the agent ships as one dependency-free binary
   and because Go's checks are part of the language rather than a hand-assembled set. Skins
   stay TypeScript, as a separate frontend over the State API.
2. Where does the hub live, and does that host have a domain or a static address?
   **Decision:** on one of the Debian servers, reached through a dedicated subdomain with a
   certificate and nginx already in place. TLS terminates at nginx, so the hub binds to
   localhost and never faces the internet directly, and Caddy is dropped from the stack
   (ADR 0005). The host name is configuration and stays out of this repository (ADR 0007).
3. Polling interval: 5 minutes for servers, 15 for laptops?
   **Decision:** intervals are per sensor, not per node, and are configured centrally on the
   hub in three layers — sensor default, node class, single node. Starting defaults: disk
   15m on servers and 1h on laptops; sensors added later (cpu, swap) set their own, finer
   ones. The agent runs a cheap base tick (5m default) that carries the heartbeat and
   whatever the sensors have collected, and each sensor collects on its own schedule above
   that tick.
   **Delivery:** the agent holds nothing but the hub address and its token. It sends
   measurements and receives the current configuration in the response to the same request,
   whenever the version it reports differs from the hub's. The hub never initiates a
   connection, so a node behind NAT, VPN or a firewall needs no exception (ADR 0002).
   Editing that configuration through the hub's web page comes at stage 3, once the panel
   has authentication; until then the hub reads it from YAML on the server.
4. Thresholds: 15% / 7% free to start? Separate thresholds for backup volumes, which are
   always nearly full by design?
   **Decision:** whichever fires first — percentage or absolute headroom. Warning below 15%
   or 20 GB free, critical below 7% or 5 GB. Percentages alone misjudge both ends of the size
   range: 10% of 8 TB needs no attention, 10% of 128 GB is nearly full.
   **Overrides** follow the same layering as intervals, with one more level: sensor default →
   node class → node → single volume, keyed by node and mount point. A volume the hub has not
   seen before takes the defaults until it is given its own. Backup volumes are declared by
   role rather than patched: `role: backup` drops percentages entirely and keeps absolute
   headroom (warning below 50 GB, critical below 10 GB), because such volumes are nearly full
   by design.
   Forecast-based alerting ("full in ~12 days") stays a later addition on top of thresholds,
   once there is enough history for it — not a replacement.
5. Which volumes are excluded (APFS snapshots, `/boot/efi`, tmpfs, external drives)?
   **Decision:** an allow-list of filesystem types, delivered with the configuration —
   `apfs`, `ext4`, `xfs`, `btrfs`, `zfs`, `ntfs` are collected, everything else (`tmpfs`,
   `devfs`, `overlay`, `squashfs`, `autofs`) is not. Filtering by type rather than by mount
   path works the same on every OS and cannot silently admit a new system volume. External
   drives are collected and flagged as removable, so an unplugged one is not mistaken for a
   volume that vanished.
   **Which sensors run** follows the same layering: a node class carries a profile
   (`server` = disk, cpu, memory, load; `laptop` = disk, battery, memory), each sensor
   reports whether it is applicable on this machine (no battery, no Docker), and the
   configuration overrides per node and per sensor. The agent sends a manifest of the sensors
   it was built with and their applicability, so the hub — and later its web page — can offer
   what exists instead of asking for names to be typed correctly.
6. Telegram: a private chat or a channel? A single recipient?
   **Decision:** a private chat with the bot, one recipient. The bot is meant to work both
   ways — manual finance input and summaries on request — and a channel has no dialogue, so
   it would trade half the future functionality for sharing nobody needs yet. The recipient
   is one configuration value behind the `Notifier` interface, so turning it into a list is
   half an hour's work when a second recipient actually exists.
7. Retention of raw points — keep everything? At minute intervals that is megabytes a year.
   **Decision:** keep everything; no downsampling is written. At the chosen intervals the
   database grows by tens of megabytes a year, slower than storage gets cheaper, and
   aggregation is irreversible while a long series is what makes trends and forecasts work.
   Deferred, not rejected: when the database actually becomes a problem, retention gets its
   own ADR and its own field in the metric schema — reserving one now would be building
   ahead of need. A daily copy of the SQLite file to the second server guards the history
   against the likelier accident.
8. Storage: SQLite, or the MySQL already running on that server?
   **Decision:** SQLite, behind the `Storage` interface. The load is a single writer with
   tens of rows a minute, which is what SQLite is for and where MySQL could show none of its
   strengths. The deciding argument is independence: a system that watches a server must not
   go blind when that server's database does. Tests also run on a bare machine and in CI with
   no service to start. MySQL's two real advantages — it is already backed up, and its data
   is viewable with familiar tools — cost one cron line and the hub's own web page.
