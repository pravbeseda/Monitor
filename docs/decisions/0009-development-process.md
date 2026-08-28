# 0009. Specs for behaviour, plans for work, ADRs for decisions

- **Status:** accepted
- **Date:** 2026-08-28

## Context

Agents write code well and guess intent badly. The industry answer in 2026 is
spec-driven development, but its documented failure mode is overhead: full
spec → plan → tasks pipelines turn small work into markdown theatre and slow
projects down more than they help. The reported break-even is narrow — specs pay off when
the work spans several sessions, when the agent makes decisions that shape the
architecture, and when someone will review or continue the work later.

This project has three document kinds with three different lifetimes, and mixing them is
what makes documentation rot: a plan is a consumable that dies at merge, a spec is an asset
that lives as long as the code, an ADR is history that is never edited.

## Decision

**A spec is mandatory** when any of these holds:

1. the work spans more than one session;
2. it touches a contract — wire format, State API, configuration schema, sensor interface;
3. it is an algorithm with state — threshold engine, silence detector, health scoring.

**A spec is not written** for sensors that only wrap a library call, for skins and visual
experiments, for the disposable POC page, or for infrastructure chores. Premature
specification of something not yet understood is a cost, not a safeguard.

**Every spec carries a behaviour table** — states, events, resulting state, side effects.
The table is the point of the document: it is what a human reviews in minutes and what
tests are derived from one row at a time. Prose around it stays minimal.

**Rows and tests cite each other.** Every row of a behaviour table has a test, and that test
carries an anchor to the row (`spec: thresholds.md#hysteresis`), so a row without a test or a
test without a row is visible. Spec-exempt work has no rows, and therefore no anchors.

**Division of labour:**

| Kind | Answers | Lifetime | Location |
|---|---|---|---|
| Spec | how the system behaves | as long as the code | `docs/specs/` |
| Plan | what to do, in what order | until merge | `docs/plans/` for stages; a checklist in the task for anything smaller |
| ADR | why it was chosen | permanent, never edited | `docs/decisions/` |

A plan never restates behaviour and a spec never lists work steps; whichever document owns
a statement is the only one that carries it.

## Consequences

- Reviewing behaviour before implementation replaces reviewing intent: the behaviour table
  is what gets approved, not a list of my steps.
- Changing behaviour means changing the spec in the same commit as the code. The pre-commit
  hook warns when guarded packages change without a spec change.
- `docs/plans/` holds only stage-sized plans; small plans live in the task and die with it.
- If a spec is ever skipped, behaviour still lives in tests — degraded, not broken.

## Alternatives

- **A full SDD pipeline** (Spec Kit, Kiro style) — rejected: the overhead the 2026 reports
  describe, on a project whose first milestone is one end-to-end thread.
- **Compiling specs into code** (CodeSpeak) — rejected for now: per-API billing on top of an
  existing subscription, a toolchain still changing its own philosophy, and a core whose
  value is heuristics that will be tuned against real data rather than generated once.
- **No specs at all, behaviour only in tests** — rejected: the edge cases an agent silently
  decides before writing a test are exactly the ones worth deciding on purpose.
