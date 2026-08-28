# POC — Free Disk Space

## Goal

See one end-to-end scenario working on real data and fix the architectural skeleton that
everything later grows from without a rewrite:

**agent on a node → ingest on the hub → storage → web page → Telegram alert.**

Scope: laptop-class nodes (macOS) and server-class nodes (Debian). Windows comes later,
but cross-platform support is a stack requirement from day one.

Metrics: free space (bytes and percent) per mounted volume per node. The agent's
liveness needs no metric of its own: the authenticated ingest request itself is the
heartbeat — every accepted request advances the node's last-seen
([ingest spec](specs/ingest.md)).

## Terminology

- **Agent** — the single application on a node that collects and sends measurements.
- **Sensor** — a module inside the agent that takes one kind of reading (disk, cpu,
  battery). Chosen over "driver": an agent has a set of sensors, like a robot.
- **Hub** — the server application: ingest, storage, evaluation, API, web.
- **Notifier** — a delivery channel for notifications (Telegram in the POC).

## Decisions this POC builds on

The reasoning and the rejected alternatives are in the ADRs; the POC only applies them.

- [0002](decisions/0002-push-not-pull.md) — push, not pull
- [0003](decisions/0003-sensors-are-modules.md) — sensors are modules
- [0004](decisions/0004-two-binaries-monorepo.md) — two artifacts, one repository
- [0005](decisions/0005-poc-stack.md) — the stack
- [0006](decisions/0006-alerting-rules.md) — alerting rules
- [0007](decisions/0007-public-repository.md) — what may not enter this repository
- [0010](decisions/0010-agent-configuration.md) — where the agent's configuration comes from
- [0011](decisions/0011-quality-gates.md) — the quality gates
- [0012](decisions/0012-threshold-model.md) — the disk threshold model
- [0013](decisions/0013-relative-hysteresis.md) — how a state recovers

## Wire format

The ingest payload is the seed of the general measurement schema, so it is versioned from
the first commit.

```json
POST /api/v1/ingest
{
  "node": "laptop-a",
  "agent_version": "0.1.0",
  "config_version": "7",
  "ts": "2026-08-28T10:00:00Z",
  "measurements": [
    { "metric": "disk.free_bytes", "labels": {"mount": "/", "fs": "apfs"}, "value": 123456789 },
    { "metric": "disk.free_pct",   "labels": {"mount": "/"}, "value": 34.2 }
  ]
}
```

`config_version` is the configuration the agent currently holds; when it differs from the
hub's, the response carries the newer one ([0010](decisions/0010-agent-configuration.md)).
The shape of that response belongs in the ingest spec.

Thresholds and node classes are declared in the hub's YAML configuration, never hard-coded
in the evaluation logic; product defaults apply where that file says nothing
([0007](decisions/0007-public-repository.md)). The deployment file itself lives on the server
and is not part of this repository, which ships only an example:

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
- [ ] Agent: disk sensor, local configuration limited to the hub url and its token, push loop
- [ ] Hub: `/api/v1/ingest`, write to SQLite, page `/` with the latest values
- [ ] Collection intervals travel in the ingest response, keyed off `config_version`
      ([0010](decisions/0010-agent-configuration.md))
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

## Questions, answered

All eight are settled. The architectural ones are owned by the ADR named against them; the
rest are recorded here, which is where they belong.

1. **Language** — Go for the agent and the hub; skins stay TypeScript over the State API.
   → [0005](decisions/0005-poc-stack.md), with the quality gates that made the trade sound
   in [0011](decisions/0011-quality-gates.md).
2. **Where the hub lives** — on one of the Debian servers, behind the nginx and certificate
   already on that host; the hub binds to localhost. The host name is a deployment setting
   and stays out of this repository ([0007](decisions/0007-public-repository.md)).
3. **Intervals** — per sensor, configured centrally, layered, delivered in the ingest
   response. → [0010](decisions/0010-agent-configuration.md). Starting values: disk every 15m
   on servers and 1h on laptops, above a 5m base tick.
4. **Thresholds** — a floor plus a proportional band, with backup volumes declared by role.
   → [0012](decisions/0012-threshold-model.md).
5. **Volumes and sensors** — an allow-list of filesystem types (`apfs`, `ext4`, `xfs`,
   `btrfs`, `zfs`, `ntfs`) delivered with the configuration; external drives are collected
   and flagged removable so an unplugged one is not read as a volume that vanished. Which
   sensors run is chosen by profile plus applicability.
   → [0010](decisions/0010-agent-configuration.md).
6. **Telegram** — a private chat, one recipient. The bot works both ways (manual input,
   summaries on request) and a channel has no dialogue. The recipient is one configuration
   value behind the `Notifier` interface.
7. **Retention** — keep every raw point; no downsampling is written. Tens of megabytes a year
   grows slower than storage gets cheaper, aggregation is irreversible, and a long series is
   what makes trends work. When it does become a problem it gets its own ADR. A daily
   `sqlite3 monitor.db ".backup /path/backup.db"` to the second server guards the history
   against the likelier accident — a plain `cp` is not equivalent, because in WAL mode the
   most recent transactions live in the `-wal` file and copying the database alone can lose
   them or corrupt the copy.
8. **Storage** — SQLite behind the `Storage` interface, not the MySQL already on that host.
   → [0005](decisions/0005-poc-stack.md).
