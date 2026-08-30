# 0016. Leaving critical is announced as instantly as entering it

- **Status:** accepted
- **Date:** 2026-08-29
- **Source:** [evaluation spec](../specs/evaluation.md), review of stage 2
- **Amends:** the delivery rule of [0006](0006-alerting-rules.md); the rest of that
  decision stands

## Context

[0006](0006-alerting-rules.md) splits delivery by level: critical is instant, warning waits
for the daily digest. It says nothing about the direction of travel, and the two readings
differ for one transition — `critical → warning`. Under the literal reading a volume that
eased overnight is reported the next morning, after the reader was woken by the critical it
is easing from.

Silence is the worst of the two states of ignorance: after a critical message, no news reads
as "still burning", so the reader checks by hand — which is the behaviour the project exists
to remove. The repeat rule makes it worse: an unresolved critical repeats once a day, so its
disappearance is itself information, delivered by nothing.

## Decision

**Instant delivery is a property of leaving or entering `critical`, not of a level.** A
subject that enters critical, eases from critical to warning, or recovers from critical to
ok is reported at once. Everything else — `ok → warning`, `warning → ok` — still waits for
the digest, exactly as [0006](0006-alerting-rules.md) says.

## Consequences

- The notification rule is one sentence with no exceptions, which is what the evaluation
  spec's table encodes.
- A critical episode is bracketed by two instant messages, so the reader never has to infer
  the end of one from the absence of a repeat.
- No new configuration: the rule is derived from the transition, not from a setting.

## Alternatives

- **Keep 0006 literally: `critical → warning` waits for the digest** — rejected: it leaves
  the reader with an unexplained silence after an alarm, for a saving of one message per
  incident.
- **Report every transition instantly** — rejected by [0006](0006-alerting-rules.md)
  already: a guaranteed mute.
- **Record this as a clarification inside 0006** — rejected: it changes behaviour rather
  than fixing an illustration, and a behaviour change belongs in its own record.
