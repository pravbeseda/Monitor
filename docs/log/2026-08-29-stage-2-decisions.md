# 2026-08-29 — Stage 2: what the evaluation design rejected

Notes from the session that produced [evaluation.md](../specs/evaluation.md),
[ADR 0015](../decisions/0015-evaluation-on-a-tick.md) and
[ADR 0016](../decisions/0016-leaving-critical-is-instant.md). The decisions live in those
documents; what is kept here is the reasoning that did not fit them.

## Seven questions, and what lost

1. **Where evaluation lives.** A tick won either way: silence, the digest and the daily
   repeat all need a clock, so the periodic pass exists whatever happens to measurements.
   Evaluating inside ingest would have added a second writer to one event log for a latency
   nobody can measure at 15m collection intervals. → [0015](../decisions/0015-evaluation-on-a-tick.md)
2. **What carries a level.** Rejected: a level per series. The rule of
   [0012](../decisions/0012-threshold-model.md) reads two metrics of one volume, so a level
   on `disk.free_bytes` would have had to name one of them "primary" and would have risked
   two alerts for one volume.
3. **How thresholds are written.** Rejected for now: an `and`/`or` tree in YAML. The engine
   builds that tree either way; what the file holds is three numbers per level, and the
   tree form can be added later as a second notation without touching the engine.
4. **How state is stored.** Rejected: deriving the current level from the event log alone.
   The log would then serve two masters — history and the scheduler's working memory — and
   every tick would run an aggregate over all history.
5. **When the digest goes out.** Rejected: the host's timezone, which makes the delivery
   hour depend on where the hub happens to run; and "every 24 hours from startup", which
   turns a daily digest into a random hour that a restart moves for good.
6. **What silence does.** Rejected: a fourth level (`unknown`) for a silent node's volumes.
   It would have propagated through states, page and catalogue, and `critical → unknown`
   reads as a recovery. Freezing gives the same alerting behaviour with no new level.
7. **How much Telegram.** Rejected for stage 2: long polling and inbound commands. Sending
   needs no polling loop, and commands are worth building when there is history to show.

## What three rounds of review changed

The tables were rewritten twice. The findings worth remembering:

- **A row can be right and prove nothing.** Several hysteresis rows stated the correct
  level, but at those volume sizes the entry rule re-fires before any exit is consulted, so
  an implementation with no hysteresis at all would pass them. Rows now name the predicate
  that decides them, and the sizes are chosen so that predicate is the deciding one.
- **The margin was unpinned from below.** Every row passed with any margin between 1.10 and
  1.20, so the 20% rule was not actually asserted. The 23 GB / 23.04 GB pair pins it.
- **Backup rows mostly did not test the backup rule.** Five of six gave the same level under
  the default rule, so `role: backup` could have been ignored entirely. One row now
  separates them.
- **`silence` froze itself.** "A silent node's subjects are frozen" plus "the node is a
  subject too" means a silent node never recovers. The subject is now exempt by rule.
- **A retry that could not happen.** Delivery keyed on this tick's transition cannot resend
  a failed `critical → warning`, because next tick nothing changed. Delivery is now keyed on
  the newest event against `last_notified_at`.
- **The ADRs carried an arithmetic slip.** 0012 and 0013 said a 128 GB volume leaves warning
  at 23 GB; 18% of 128 GB is 23.04 GB. Corrected in place: the decision did not change, only
  an illustration of it.

## Not decided here

Colouring the web page by level, per-recipient locale, a per-metric hysteresis margin, and
forecast alerting are all recorded as out of scope in
[evaluation.md](../specs/evaluation.md).
