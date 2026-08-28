# 0007. Public repository from the first commit; everything personal stays out

- **Status:** accepted
- **Date:** 2026-08-28
- **Source:** [concept](../concept.md)

## Context

Monitor handles health, finance and infrastructure data. The project is nevertheless meant
to be open source, like the tools it sits next to (`node_exporter`, Home Assistant,
Grafana) — public tooling, private configuration.

Two risks are easy to confuse. Leaked **secrets** are loud and can be rotated in a minute.
Leaked **personal data** is quiet and cannot be recalled. The second is the greater risk
here, and it does not arrive through configuration files — it arrives through metric
schemas, test fixtures, screenshots, logs pasted into issues and commit messages.

Publishing a repository publishes its entire history, not its current state. Cleaning up
afterwards means rewriting history, rotating every secret it ever contained, and living
with forks and caches that keep the original.

## Decision

The repository is public from the first commit, so the discipline is never retrofitted.
Four levels, and only the first one is versioned here:

| Level | Contents | Where it lives |
|---|---|---|
| Tooling | agent, hub, evaluation engine, skins, general-purpose sensors | **this public repository** |
| Configuration | metric schemas, thresholds, node names and classes, domains, chat ids, skin mappings | on the server; a private repository once it outgrows a few files |
| Secrets | node tokens, bot token, TLS material | environment file (mode 600) or OS keychain, never in configuration files |
| Measurements | the SQLite database | on the server only, never beside a checkout |

**Never committed:** real node names, real thresholds, real domains or addresses, exported
health or finance data, fixtures built from real exports, database dumps, screenshots of a
live dashboard, log excerpts containing mount points or node names.

**Rules that make the split hold:**

1. **No configuration defaults in code.** The hub does not start with "reasonable values"
   when configuration is missing; it exits with a clear error. A default is how a personal
   setting leaks into the tool.
2. **Examples are synthetic.** `config.example.yaml` and every sample in the documentation
   use invented names (`laptop-a`, `server-b`) and round numbers.
3. **Fixtures are synthetic.** Test data is generated or hand-written, never a real export.
4. **A secret scanner runs before every commit**: `.githooks/pre-commit` blocks staged
   configuration, secrets and databases, then runs `gitleaks` over the staged diff. The
   hooks directory is versioned and enabled with `git config core.hooksPath .githooks`.
   Forge-side secret scanning with push protection is enabled as a second line.
5. **Security rests on secrets, not on obscurity.** A public repository publishes the
   threat model too: long random per-node tokens, rate limiting on ingest, and
   authentication on the web page before the hub gets a public address.
6. **A sensor whose code discloses private facts** — a parser for one specific broker's
   statement, an integration with one specific account — ships as an external sensor in the
   private configuration repository, through the interface from
   [0003](0003-sensors-are-modules.md). The privacy boundary is an existing architectural
   boundary.

## Consequences

- The separate private configuration repository is planned, not built: while configuration
  is a handful of files, it lives on the server. It is split out when the first private
  sensor appears, or when configuration needs its own history and review.
- Issues, pull requests and commit messages are subject to the same rules as code.
- Documentation examples cannot be copy-pasted into production; that is the intended cost.

## Alternatives

- **Private now, public after the POC** — rejected: the boundary between tool and
  configuration would be drawn after the fact, and the whole history would need an audit.
- **Two repositories from day one** — deferred, not rejected: two checkouts for thirty
  lines of configuration is overhead the POC does not need.
