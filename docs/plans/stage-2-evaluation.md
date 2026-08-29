# Plan: POC Stage 2 — evaluation and alerts

- **Status:** proposed
- **Source:** [poc.md](../poc.md) stage 2
- **Specs:** [evaluation.md](../specs/evaluation.md) (approved),
  [hub-config.md](../specs/hub-config.md) (approved)
- **Decisions:** [0006](../decisions/0006-alerting-rules.md),
  [0012](../decisions/0012-threshold-model.md),
  [0013](../decisions/0013-relative-hysteresis.md),
  [0015](../decisions/0015-evaluation-on-a-tick.md),
  [0016](../decisions/0016-leaving-critical-is-instant.md)

Measurements already arrive and are stored; this stage gives them meaning and turns a
change of meaning into a message. Nothing about the agent or the ingest contract changes.

Every step is test-first (red → green → refactor), ends green on CI, and every test that
covers a behaviour-table row cites it with `spec: evaluation.md#<anchor>`. Steps are
ordered so each one is testable without the ones after it.

## Constraints that hold for every step

- **Dependency direction:** `config → evaluate`, `config → i18n`, `evaluate → storage`.
  Neither `evaluate` nor `storage` imports `config`; `internal/evaluate` reaches
  configuration through a consumer-side interface of its own, exactly as it reaches
  storage. `internal/evaluate` owns `Level`, `Rule` and the rule names;
  `internal/storage` stores a level as a string and never imports `evaluate`.
- **No `SELECT` inside a write transaction.** The tick reads its whole snapshot outside any
  transaction (the spec's tick, step 1); a write transaction opens, writes and commits.
  A deferred read-then-write transaction fails `SQLITE_BUSY` immediately, without waiting
  out `busy_timeout`.
- **No test sleeps and no wall clock.** The tick takes `now`; the loop takes its ticks from
  an injected channel. Anything a test observes across goroutines — the notifier recorder,
  the log — is guarded.
- **Secrets stay out of errors.** Nothing wraps an error that can carry a token.

## Steps

0. **Documentation.**
   The evaluation spec, ADRs 0015 and 0016, the log note, the cross-document edits and the
   stage-1 checkbox in [poc.md](../poc.md); pin `golangci-lint` in CI to the version the
   toolchain installs locally, so a release mid-stage cannot turn CI red on its own.
   → verify: no Go file in the commit; `docs/index.md` lists both ADRs, the spec, the log
   note and this plan.

1. **Threshold engine and rule vocabulary, pure.**
   `internal/evaluate`: `Level` and its order; the rule registry — for each rule name the
   sensor it reads and the metric ids under their value names (`disk` → sensor `disk`,
   `free`, `pct`); the threshold type and the compiled-in product defaults; entry and exit
   predicates; the selection loop written as the spec's loop, not an if-else chain; the
   absent-band convention. No storage, no clock, no i/o.
   → verify: one test per row of `#levels`, `#hysteresis`, `#backup-volumes`; a test that
   `10GB` and `500MB` parse decimally.

2. **Configuration: rules and volumes.**
   `internal/config`: `rules` at the top level, on a class, on a node and on a volume, with
   the `backup` branch merged separately; `volumes` with its `role`; sizes accepted both as
   `10GB` and as a bare integer, reported against their key the way durations already are;
   an accessor returning a subject's resolved `evaluate.Rule`.
   → verify: the `#startup-validation` rows about sizes, `ratio`, unknown rule names,
   critical-above-warning, `ratio`/`ceiling` under `backup` and `role`; the
   `#configuration-changes` row about a trailing slash; a test that a file differing only in
   these keys yields an identical `config_version`
   (`spec: hub-config.md#configuration-version`). The same commit removes the "refuses them
   as unknown" paragraph from [hub-config.md](../specs/hub-config.md).

3. **Configuration: digest and notify.**
   `digest.at` and `digest.timezone`, `notify.channel` and `notify.locale`, the Telegram
   environment variables read here beside the node tokens, `Digest()` and `Notify()`.
   `internal/i18n` exports a strict locale parser — the existing negotiation falls back to
   English, which is the opposite of what a startup check needs — and a regional tag in the
   file is an error.
   → verify: the remaining `#startup-validation` rows, including the unset-variable one,
   whose error names the variable and never its value.

4. **State and event storage.**
   `internal/storage`: `states`, `events` and `meta`, an index on `events(at)`, and a
   `PRAGMA user_version` migration list so a column added later in this stage is a new
   migration rather than a silent no-op on the dogfood database. Operations: load all
   states, apply one transition (state + event) in one transaction returning the record it
   wrote, the newest event per subject, events in a window, record a delivery, read and
   write `last_digest_at`. `last_notified_at` is empty until a delivery. The DSN gains
   `_txlock=immediate` and is built URL-safely, so a temp path containing `#` cannot
   truncate it. The tick reuses the existing `States` for its snapshot; `Storage` keeps its
   three methods and its three test doubles untouched.
   → verify: the writing rows of `#storage` — the transaction, `since` untouched, both
   first-evaluation rows — read-back after reopening, and a test that the new tables appear
   on a database created by stage 1.

5. **Subjects, freezing and silence.**
   Join series into subjects, resolve each subject's rule, compute `stale_after` from the
   node's resolved sensor interval, decide the `silence` subject first, freeze the rest;
   read the snapshot once in the spec's order and evaluate nodes by name, subjects by
   `mount`. Levels are computed and written; nothing is delivered yet. Every entry point
   takes `now`.
   → verify: rows of `#freezing`, the state columns of `#node-silence`,
   `#configuration-changes`, the `#storage` idempotence row, a test asserting the evaluation
   sequence over two nodes and two volumes, and one test that runs a tick against a real
   `*storage.SQLite` while another goroutine calls `SaveIngest` on the same handle.

6. **Messages and the log channel.**
   The `Notifier` interface and the message value of `#messages`; `internal/i18n` gains
   argument substitution and duration formatting, plus the alert, silence and digest keys in
   both languages; the `log` channel writes English whatever `notify.locale` says. The test
   recorder is guarded from the start, because step 10 calls it from the tick goroutine.
   → verify: rows of `#messages` except the Telegram one, which step 9 closes; a test that
   every new key exists in `en` and `ru` with the same number of substitutions.

7. **Delivery.**
   Instant versus digest by transition, the 24h repeat, delivery driven by the newest event
   against `last_notified_at`, the commit → send → record order, per-message failure and
   redelivery on the next tick. Every notifier call is made under a 10s context, whatever
   the channel.
   → verify: rows of `#notifications`, the side-effect column of `#node-silence`, the
   `#storage` rows about the two-phase write and about a restart re-notifying nothing, and a
   test that a notifier blocking past the timeout is recorded as a failed delivery while the
   tick continues.

8. **Digest.**
   The due predicate over `digest.at` in `digest.timezone`, the window from `events` plus
   the subjects currently in `warning`, `last_digest_at` set to the occurrence and seeded
   with the tick's `now` when the `meta` row is absent, failure leaving it untouched. Tests
   that name a real zone link `time/tzdata` themselves, so they pass on a host without
   `zoneinfo`.
   → verify: rows of `#digest`, including the never-digested database, restart, two-day
   outage and DST.

9. **Telegram channel.**
   `sendMessage` over HTTP behind the same `Notifier`, the token and chat id passed in from
   the configuration rather than read from the environment a second time, the timeout as a
   field so no test waits it out.
   → verify: the Telegram row of `#messages` against an `httptest` server; a test that a
   rejected request produces a failed delivery whose error and log line contain neither the
   token nor the chat id — the transport error is never wrapped, because the token lives in
   the URL.

10. **The evaluation loop.**
    `Run(ctx, ticks <-chan time.Time)` in `internal/evaluate`: one tick per received time,
    a tick arriving while the previous is still running is skipped and logged, and
    cancellation returns without starting another subject. The logger is injected.
    → verify: the `#storage` rows about an overlapping tick and about stopping mid-tick,
    driven by the channel with no sleep.

11. **Wiring in `cmd/hub`.**
    The notifier built from `notify.channel`, the 1m ticker feeding `Run`, the
    `start`/`run(ctx, …)` split that `cmd/agent` already uses, the tick goroutine joined
    before the database is closed, `http.ErrServerClosed` and `context.Canceled` treated as
    a clean stop, `time/tzdata` linked in.
    → verify: a test that the process stops cleanly on a cancelled context with the server
    closed and the database not closed under a running tick, binding `127.0.0.1:0`; a test
    that each `notify.channel` yields its notifier.

12. **Manual end-to-end run.**
    Hub on localhost with `channel: telegram`, one real node: fill a volume past the warning
    floor and then past the critical floor, free it again, then stop the agent past
    `silence_after`.
    → verify: instant critical and recovery messages arrive, the silence message arrives one
    tick after the window, the warning appears in the next digest, and nothing repeats
    inside 24h.

13. **Self-review and documentation.**
    Review the whole diff (simplicity, duplication, dead code, spec conformance), extend
    `config.example.yaml` with the new keys under synthetic names, and re-read every spec
    against what was built.
    → verify: a test in `internal/config` that loads `config.example.yaml` with the token
    variables set and resolves every node, so the shipped example cannot drift; a check that
    every `### ` heading of `evaluation.md` is cited by at least one `spec:` anchor in the
    tests and that every cited anchor exists; the stage-2 boxes in [poc.md](../poc.md)
    ticked.

## Out of scope

Colouring the web page by level and rendering the event log; inbound Telegram commands;
systemd and launchd units, nginx, install scripts, page authentication (stage 3); forecast
alerting; a per-metric hysteresis margin; a locale per recipient.

## Done when

Every behaviour-table row of [evaluation.md](../specs/evaluation.md) has a test citing it
and the anchor check in step 13 proves it; CI is green (`gofmt`, `go vet`,
`golangci-lint`, `go test -race -cover` on ubuntu and macos); the manual run of step 12
delivered every kind of message through Telegram; the self-review was run and its findings
fixed; and the specs, the ADRs and `docs/index.md` match what was built.
