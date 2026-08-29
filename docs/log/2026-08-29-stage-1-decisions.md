# 2026-08-29 — decisions taken while building stage 1

Everything here was settled while writing the specs and the code of POC stage 1. What
belongs to a subsystem is recorded in its spec; this note keeps the reasoning that has no
other home, and the alternatives that were rejected.

## Locale comes from the request, not from the installation

The panel picks its language per request: a supported `?lang=` wins, otherwise the
browser's `Accept-Language`, otherwise English. A `?lang=` naming a language the panel does
not speak is ignored, not honoured: a broken link passed around should not cost the reader
the language the browser already asks for. Rejected:

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

## Fifteen volumes, two of which are volumes

The first Mac the agent ran on reported fifteen volumes: seven of one APFS container that
all show the same free space, three of the external drive's container, two simulator
images, and the two an operator would name. Stage 2 would have sent seven identical alerts
for one full disk.

The sensor now keeps one volume per container — the shortest mount point, which is the one
a person recognises — and skips the mount prefixes the hub's `skip_mounts` names, a product
default of `/System/Volumes/` and `/Library/Developer/CoreSimulator/`. Rejected:

- **The skip list alone** — one setting instead of two, but the three volumes of the
  external drive stay three rows, and `/System/Volumes/Data` would have to be skipped by
  hand, which is one typo away from losing the volume that actually holds the data.
- **Collapsing containers alone** — no configuration at all, but the simulator images are
  their own containers and would survive.
- **Collecting everything and hiding it on the page** — the history stays complete and the
  choice reversible, at the price of seven times the rows for one fact, and stage 2 would
  still evaluate all of them.

## Deferred deliberately

- **Threshold sections in the hub's configuration** (`metrics`, volume roles) stay unknown
  keys until the evaluation spec introduces them in stage 2, so no code ships without a
  consumer.
- **Manifest-based filtering** — the hub delivers a sensor even to a node whose manifest
  lacks it; the agent ignores what it cannot run. Filtering belongs with the configuration
  editor in stage 3, where the manifest is what fills the form.
