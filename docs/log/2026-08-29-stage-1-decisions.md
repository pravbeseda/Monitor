# 2026-08-29 — decisions taken while building stage 1

Everything here was settled while writing the specs and the code of POC stage 1. What
belongs to a subsystem is recorded in its spec; this note keeps the reasoning that has no
other home, and the alternatives that were rejected.

## Locale comes from the request, not from the installation

The panel picks its language per request: an explicit `?lang=` wins, otherwise the
browser's `Accept-Language`, otherwise English. Rejected:

- **`?lang=` only, English by default** — predictable and trivial to test, but the panel is
  opened from a phone bookmark and asking for Russian every time is a tax on the one user.
- **One language per installation, set in the hub's configuration** — no per-request logic,
  but [0008](../decisions/0008-english-repo-bilingual-ui.md) asks for a bilingual interface,
  not a configurable one, and it cannot be switched while looking at a page.

Remembering the choice in a cookie waits for stage 3, when the page gets authentication and
therefore a session to remember it in.

## The wire format is one package, not two definitions

`internal/api` holds the request and response types, and both binaries import it. The
alternative — the agent describing the format it writes and the hub the format it reads —
was rejected the moment the second copy would have been written: two definitions of one
contract drift silently, and the drift shows up as a 400 in production rather than as a
failing test.

## What the end-to-end run caught

The first live run refused every heartbeat with `400: measurements is required`. A tick
with nothing to send marshalled its empty batch as `null`, which the hub reads as an absent
field. Every unit test passed, because they inspected the request struct rather than the
bytes: a pointer to a nil slice is non-nil.

The lesson is recorded here rather than in a spec: where a contract is about bytes, at
least one test has to look at the bytes.

## Deferred deliberately

- **Threshold sections in the hub's configuration** (`metrics`, volume roles) stay unknown
  keys until the evaluation spec introduces them in stage 2, so no code ships without a
  consumer.
- **Manifest-based filtering** — the hub delivers a sensor even to a node whose manifest
  lacks it; the agent ignores what it cannot run. Filtering belongs with the configuration
  editor in stage 3, where the manifest is what fills the form.
