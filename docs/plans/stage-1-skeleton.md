# Plan: POC Stage 1 — skeleton

- **Status:** approved
- **Source:** [poc.md](../poc.md) stage 1
- **Specs:** [ingest.md](../specs/ingest.md) (approved)

One end-to-end thread: disk sensor on a node → `/api/v1/ingest` → SQLite → web page.
No evaluation, no alerts, no services — those are stages 2 and 3.

Each step is test-first (red → green → refactor) and ends green on CI. Ingest tests cite
their behaviour-table rows with `spec: ingest.md#<anchor>`.

## Steps

1. **Scaffolding and quality gates.**
   `go.mod`, `cmd/agent`, `cmd/hub` (hello-world binaries), `internal/`; CI workflow with
   `gofmt`, `go vet`, `golangci-lint`, `go test -race -cover` per
   [ADR 0011](../decisions/0011-quality-gates.md); pre-commit extended with the fast pair.
   → verify: CI green on a trivial test.

2. **Storage.**
   `Storage` interface + SQLite implementation (`modernc.org/sqlite`, WAL). Tables:
   measurements (unique over node, metric, labels, ts), nodes (last-seen, config version,
   manifest). → verify: unit tests incl. duplicate-insert idempotence.

3. **Hub configuration.**
   Load the YAML (node classes, nodes, tokens reference, sensor intervals, filesystem
   allow-list); resolve the layers into one flat per-node config with a version. Missing
   deployment values are a startup error, never a default
   ([ADR 0007](../decisions/0007-public-repository.md)). Ship `config.example.yaml` with
   synthetic names. → verify: unit tests for layering and startup failure.

4. **Ingest endpoint.**
   Implement every behaviour-table row of [ingest.md](../specs/ingest.md): auth,
   validation, storage, config delivery, limits. → verify: one test per row, anchors in
   place.

5. **Disk sensor.**
   Short behaviour table in `docs/specs/disk-sensor.md` first (mount enumeration,
   allow-list filtering, removable flag, labels) — it defines the label contract. Then the
   sensor for macOS and Debian behind the `Sensor` interface
   ([ADR 0003](../decisions/0003-sensors-are-modules.md)). → **gate:** table approved;
   verify: unit tests with a fake mount source.

6. **Agent loop.**
   Local config = hub URL + token (env/flags only); base tick, per-sensor intervals from
   the ingest response, config application on version change
   ([ADR 0010](../decisions/0010-agent-configuration.md)). → verify: loop tests against a
   fake hub.

7. **Web page `/`.**
   Latest values and last-seen per node, `html/template`, strings from the en/ru catalogue
   from the first string ([ADR 0008](../decisions/0008-english-repo-bilingual-ui.md)).
   → verify: handler test; page renders in both locales.

8. **Manual end-to-end run.**
   Hub on localhost, agent on this laptop and one server, real disks. → verify: values
   visible on the page, last-seen advances, config change picked up within one tick.

## Out of scope

Threshold evaluation, hysteresis, silence alerts, Telegram, systemd/launchd, nginx,
install scripts (stages 2–3). Windows.

## Done when

All steps green on CI, the page shows live values from two real nodes, and the
documentation (specs, index) matches what was built.
