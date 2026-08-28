# Monitor — Documentation Map

A personal control panel for infrastructure, finance and health metrics. This file is the
entry point to every document in the project.

## Product and plan

| Document | Contents |
|---|---|
| [concept.md](concept.md) | Idea, architectural principles, planned skins, domains, roadmap |
| [poc.md](poc.md) | POC spec: scope, terminology, wire format, work plan, open questions |

## Architecture decisions

One decision per file, each stating what was rejected and why. New records start from
[TEMPLATE.md](decisions/TEMPLATE.md) with the next free number.

| # | Decision | Status |
|---|---|---|
| [0001](decisions/0001-semantic-core-and-skins.md) | Meaning lives in the core; skins are dumb renderers | accepted |
| [0002](decisions/0002-push-not-pull.md) | Agents push; the server never polls nodes | accepted |
| [0003](decisions/0003-sensors-are-modules.md) | Sensors are in-process modules of the agent | accepted |
| [0004](decisions/0004-two-binaries-monorepo.md) | Two artifacts — agent and monolithic hub — in one repository | accepted |
| [0005](decisions/0005-poc-stack.md) | POC stack: Go, SQLite, server-side HTML | proposed |
| [0006](decisions/0006-alerting-rules.md) | Alert on transitions; only critical is instant | accepted |
| [0007](decisions/0007-public-repository.md) | Public repository from the first commit; nothing personal in it | accepted |
| [0008](decisions/0008-english-repo-bilingual-ui.md) | The repository is English; the interface is bilingual | accepted |
| [0009](decisions/0009-development-process.md) | Specs for behaviour, plans for work, ADRs for decisions | accepted |

## Behaviour specs

How subsystems behave, one file per subsystem, each built around a behaviour table that
tests are derived from. Required for contracts, stateful algorithms and multi-session work
(see [ADR 0009](decisions/0009-development-process.md)); new specs start from
[TEMPLATE.md](specs/TEMPLATE.md).

| Spec | Owns | Status |
|---|---|---|
| _none yet_ | | |

## Design notes

Reasoning from working sessions, including options that were rejected.

| Date | Topic |
|---|---|
| [2026-08-28](log/2026-08-28-concept.md) | Project start: visual concepts, architecture, naming |

## Not written yet

- `docs/architecture/` — subsystem documents arrive with the code. Until then the
  architecture fits in [concept.md](concept.md), the ADRs and the specs, and duplicating it
  would create a second source of truth.
- `docs/plans/` — stage-sized plans; created with the first one.
- Install and deployment guide — after POC stage 1.

## Open questions

Kept at the end of [poc.md](poc.md). When one is answered, that document is updated in the
same session, and the answer becomes an ADR if it is architectural.
