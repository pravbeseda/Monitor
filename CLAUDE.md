# Monitor

A personal control panel for life: metrics from infrastructure, finance and health flow
into one semantic core that computes their *meaning* (health 0–100, status, trend, anomaly
rank, freshness) and is rendered by interchangeable skins.

## Commands

```
go test -race -cover ./...   # tests
gofmt -l . && go vet ./...   # the fast pair, also run by the pre-commit hook
golangci-lint run            # the full linter set, also run by CI

MONITOR_TOKEN_LAPTOP_A=<token> go run ./cmd/hub --config config.yaml --db monitor.db
MONITOR_TOKEN=<token> go run ./cmd/agent --hub http://127.0.0.1:8080 --node laptop-a
```

**Where the project stands is not written here.** The work plan in
[docs/poc.md](docs/poc.md) and the stage plans in [docs/plans/](docs/plans/) own that, and
a second copy of it in this file is a copy that goes stale.

## Start here

**Read `docs/index.md` first.** It maps every document. The architecture is recorded as
ADRs in `docs/decisions/` — each one states what was rejected and why, so read the ADR
before re-opening a settled question.

```
docs/index.md              map of all documentation — the entry point
docs/concept.md            product concept, principles, roadmap
docs/poc.md                POC spec, wire format, work plan, answered questions
docs/decisions/NNNN-*.md   ADRs, one decision per file (TEMPLATE.md is the skeleton)
docs/specs/<subsystem>.md  behaviour specs: the table each test derives from
docs/plans/                stage-sized plans only; smaller plans live in the task
docs/log/YYYY-MM-DD-*.md   design notes: reasoning that did not fit the final documents
```

Documents are cross-linked with **relative Markdown links** (`[poc.md]` + `(../poc.md)`), never
Obsidian `[[wikilinks]]`: relative paths resolve identically in Obsidian, on the forge, and
for an agent reading files directly.

## This repository is public — hard rules

Per [ADR 0007](docs/decisions/0007-public-repository.md), the repository is public from the
first commit and contains **tooling only**. Configuration, secrets and measurements live on
the server. Publishing history is irreversible, so these rules bind every commit:

1. **Never commit** real node names, real domains or addresses, thresholds tied to a named
   host or volume, exported health or finance data, fixtures built from real exports,
   database dumps, screenshots of a live dashboard, or log excerpts containing mount points
   or node names. The same rule covers commit messages, issues and pull requests.
2. **Product defaults may ship; deployment settings may not.** Thresholds, intervals and
   sensor profiles describe how the tool behaves out of the box and belong in the repository.
   Hub address, tokens, node names and classes, chat ids and any host- or volume-specific
   override have no defaults: missing configuration is a clear startup error, never a
   "reasonable value". The test is whether the value says something about *an installation*.
3. **Examples and fixtures are synthetic** — invented names (`laptop-a`, `server-b`), round
   numbers, generated data. Never a real export, not even temporarily.
4. **Secrets never enter a file in the tree**, including example configs: they come from an
   environment file or the OS keychain.
5. **Security rests on secrets, not obscurity**: long random per-node tokens, rate limiting
   on ingest, authentication on the web page before the hub gets a public address.
6. If a sensor's own code would disclose a private fact (one specific broker, one specific
   account), it belongs in the private configuration repository as an external sensor —
   see [ADR 0003](docs/decisions/0003-sensors-are-modules.md).

When unsure whether something is personal, leave it out and say so in the summary.

**The scanner enforces this, not attention.** `.githooks/pre-commit` refuses a commit that
stages configuration, secrets or a database, and runs `gitleaks` over the staged diff. Hooks
are versioned, so a fresh clone must enable them once:

```
git config core.hooksPath .githooks   # required after cloning
brew install gitleaks                 # macOS; see the gitleaks README on Debian
brew install go golangci-lint         # the toolchain the hook and CI run
```

## Language

Per [ADR 0008](docs/decisions/0008-english-repo-bilingual-ui.md):

- **The repository is English** — code, identifiers, comments, documentation, config keys,
  metric ids, commit messages, CLI output, issue and PR text. No exceptions.
- **The interface is bilingual (en/ru)** from the first user-facing string: web, Telegram
  messages, alerts, digests, user-facing errors. No user-facing string is hard-coded at its
  usage site; strings come from a catalogue keyed by an English identifier, and locale
  governs number, date and byte-size formatting too.
- Log lines and CLI output stay English: they are diagnostic and end up in bug reports.

## Development process (mandatory)

Per [ADR 0009](docs/decisions/0009-development-process.md). Every unit of work runs through
these stages in order. Stages marked **gate** stop and wait for the user; the rest are
reported and continued without asking.

1. **Scope.** Restate the task, name the assumptions, ask the blocking questions one at a
   time. → **gate:** no unanswered blocking question remains.
2. **Spec.** Write or update `docs/specs/<subsystem>.md` when the work spans more than one
   session, touches a contract, or is an algorithm with state. Skip it, saying so, when none
   of those hold. The gate is on writing the code, not on deciding the approach: an ADR may
   be accepted before its subsystem has a spec. → **gate:** the behaviour table is approved.
3. **Plan.** A checklist of steps in the task; a document in `docs/plans/` only for
   stage-sized work. Steps, never behaviour. → **gate:** the plan is approved.
4. **Branch.** Work on a branch off `main`. Never commit to `main`.
5. **Implement, step by step, test-first.** Red → green → refactor per step. Every row of a
   behaviour table the step touches gets a test, and that test cites the row with a `spec:`
   anchor; work with no spec has no rows and no anchors. Report after each step and keep
   going — never ask whether to continue.
6. **Self-review.** Review the diff before showing it: simplicity, duplication, dead code,
   conformance to the spec and the plan, contradictions with the documentation. Fix what it
   finds and review again.
7. **Documentation.** Update the spec, ADRs and `docs/index.md`; report which documents were
   touched.
8. **Pull request.** → **gate:** never opened without explicit permission for that PR.
9. **Merge.** → **gate:** only on the user's explicit instruction.

**Definition of done** for any unit of work: tests pass, every behaviour-table row that the
change touches has a test, the spec matches the code, the documentation was updated, and the
self-review was run.

### Holding the line

The user has asked to be kept on this process. When a step would skip a stage — coding
before an approved spec, committing to `main`, opening a PR unasked, leaving a spec stale:

1. Say plainly which stage is being skipped and what it protects, in one or two sentences.
2. Offer the shortest legitimate path — usually "the behaviour table takes five minutes,
   then I implement".
3. If the user confirms anyway, do it: it is their project. Record the skip in the task or
   the PR description, so it is visible rather than forgotten.

Say it once per occurrence. Repeating an objection the user has already overruled is noise,
not diligence.

## Documentation duties (mandatory)

Documentation is part of the work, not a follow-up task:

1. **Architectural decision taken → write an ADR** from `TEMPLATE.md` with the next free
   number and add its row to `docs/index.md`. A decision is architectural if reversing it
   later would mean rewriting code rather than editing it.
2. **Open question answered → update the document that poses it** in the same session, and
   promote the answer to an ADR when it is architectural.
3. **New document created → add it to `docs/index.md`.** An unlisted document does not exist.
4. **Behaviour changed → update the document that describes it.** Never leave code and
   documentation contradicting each other; if both cannot be fixed now, say so explicitly.
5. **Substantial discussion → leave a note in `docs/log/`** capturing what was rejected and
   why. That reasoning is lost otherwise when the session ends.

Keep documents lean: no duplication between them, no filler. An ADR is the single source of
truth for its decision — other documents link to it instead of restating the reasoning. An
accepted ADR is never rewritten and its number is never reused; a reversal is a new ADR that
marks the old one superseded.

Report at the end of a task which documents were touched.

## Architecture in one screen

Each line is an index into the ADR that owns it.

- **Two binaries, one monorepo**: `cmd/agent` per node, `cmd/hub` as a monolith (ingest +
  storage + evaluation + web + notifier), shared code in `internal/...`. → 0004
- **Push, not pull**: agents POST to `/api/v1/ingest` over HTTPS with a per-node token; node
  silence is an expected state configured per node class (`silence_after`). → 0002
- **Sensors are in-process modules** (`Sensor: collect() -> []Measurement`); the interface
  must not reveal whether an implementation is built in or external. → 0003
- **Metrics are declared, not coded**: id, domain, source, unit, direction, thresholds and
  expected interval come from the hub's YAML config. Health, anomaly and silence detection
  all derive from that schema.
- **The semantic core is the single source of truth**; the State API carries semantics only,
  never skin-specific fields. Skins are equal consumers of it, and drill-down, `at=` time
  travel and the event stream live outside them. → 0001
- **Alerts fire on transitions** with hysteresis; entering or leaving critical is instant,
  everything else batches into a daily digest, unresolved critical repeats at most once a
  day. → 0006, 0016
- **Evaluation runs on the hub's own tick**, never inside an ingest request: one writer of
  the event log, and silence, digest and repeat are the same pass. → 0015
- **Hysteresis is relative**: recovery is the negated entry rule with every comparison
  shifted by 20% of its threshold, whatever the unit. → 0013
- **Stack**: Go for both binaries, SQLite (`modernc.org/sqlite`, no CGO) behind a `Storage`
  interface, server-side `html/template`, systemd/launchd, TLS terminated by the host's
  nginx with the hub bound to localhost. → 0005
- **The agent carries no configuration** beyond the hub address and its token: which sensors
  run, how often and with which thresholds comes from the hub in the ingest response, layered
  sensor default → node class → node → sensor or volume. → 0010
- **Quality is machine-enforced**: CI runs `gofmt`, `go vet`, `golangci-lint` and
  `go test -race -cover`, and a red check blocks the merge; the pre-commit hook runs the fast
  pair (`gofmt`, `go vet`). → 0011
- **Disk thresholds are a floor plus a band**: alert below the floor, or below the ratio
  while absolute headroom is under the ceiling; `role: backup` volumes use headroom only.
  → 0012
- The versioned API prefix (`/api/v1/...`) is deliberate — keep it on every new endpoint.
- The project's value is the normalization and prioritization layer, not storage or
  charting; weigh new low-level work against what off-the-shelf tools already do.
