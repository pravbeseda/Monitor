# Spec: Agent

- **Status:** approved
- **Owns:** `cmd/agent` and `internal/agent`: the tick loop, sensor scheduling, delivery
  and configuration application
- **Decisions:** [0002](../decisions/0002-push-not-pull.md),
  [0003](../decisions/0003-sensors-are-modules.md),
  [0007](../decisions/0007-public-repository.md),
  [0010](../decisions/0010-agent-configuration.md)

## Purpose

The agent is the only thing running on a node. It ticks, asks each sensor whose interval
has elapsed for measurements, posts them to the hub, and applies whatever configuration
the response carries. It decides nothing about meaning: no thresholds, no alerts, no
history.

It holds three local values and no more — the hub URL, its node name, and its token — so
that a node is configured once and never edited again
([0010](../decisions/0010-agent-configuration.md)).

## Local configuration

| Value | Source | Default |
|---|---|---|
| hub URL | `--hub` | none: a deployment setting |
| node name | `--node` | none: it must match the name the hub's token belongs to |
| token | `MONITOR_TOKEN` | none: a secret never lives in a file in the tree |
| bootstrap tick | compiled in | 5m, until the first configuration arrives |

## Behaviour

One row = one test. Anchors: `spec: agent.md#<heading>`.

### Ticking

| State | Event | Behaviour |
|---|---|---|
| started, no configuration yet | first tick | posts an empty batch with an empty `config_version` and the manifest |
| a sensor whose interval has elapsed | tick | it collects, and its measurements ride this tick's request |
| a sensor whose interval has not elapsed | tick | it is not called |
| a sensor disabled by the configuration | tick | it is not called, whatever its interval |
| no sensor is due | tick | a request with an empty batch — the heartbeat of [0002](../decisions/0002-push-not-pull.md) |
| a sensor returns an error | tick | the error is logged, the other sensors still post |
| a sensor's interval changes | next tick | it is measured from the sensor's last collection, not from the change |

### Delivering

What the hub did not accept is kept in memory and resent with the next tick, capped at
10 000 measurements — days of disk readings, and a bounded appetite on a laptop that
spends a week off the network. Measurements carry their collection time, so a late batch
lands in the right place in history.

| State | Hub answer | Behaviour |
|---|---|---|
| a batch was sent | 200 | the batch is dropped: it is stored |
| a batch was sent | 4xx | logged and dropped — a rejected shape will not become valid by waiting |
| a batch was sent | 429 | kept, and retried on the next tick |
| a batch was sent | 5xx, timeout or no network | kept, and retried on the next tick |
| a batch is being kept | the buffer is full (10 000 measurements) | the oldest are dropped first |
| the agent restarts | — | the buffer is lost: it lives in memory only |

### Applying configuration

| State | Response | Behaviour |
|---|---|---|
| any | body `{}` | the configuration in use is kept |
| any | `config` with a different `config_version` | it replaces the current one wholesale and takes effect from the next tick |
| any | a response field the agent does not know | ignored |
| any | a configuration it cannot parse | the working one is kept and the failure logged |
| the configuration changes the base tick | applied | the new tick starts after the current one ends |
| the configuration disables the only sensor | applied | the agent keeps ticking: the heartbeat is the point |

## Invariants

- The agent merges nothing: the hub sends a resolved configuration and the agent replaces
  what it holds ([0010](../decisions/0010-agent-configuration.md)).
- Every tick sends exactly one request, whether or not any sensor had something to say.
- A sensor never blocks the tick: collection runs under a timeout shorter than the tick.
- The token is never logged, and never leaves the `Authorization` header.
- Measurements carry the collection time, so a delayed batch keeps its own history.

## Edge cases

- **The hub is unreachable for hours**: measurements accumulate up to the buffer cap,
  the oldest going first; the node's silence is the hub's business to detect (stage 2).
- **The clock jumps** (sleep, NTP correction): a sensor is due when its interval has
  elapsed by the monotonic clock, so a jump neither floods the hub nor stalls collection.
- **A laptop sleeps and wakes**: the first tick after waking finds every sensor due and
  posts one request with all of them.
- **The hub answers 403** (the node does not match the token): logged as a configuration
  error, and the agent keeps ticking — the operator fixes the hub, not the node.

## Out of scope

- What the hub does with the measurements → [ingest](ingest.md).
- Where the configuration comes from and how it is resolved → [hub-config](hub-config.md).
- How a sensor reads its values → [disk-sensor](disk-sensor.md).
- Install, service units and updates → stage 3.

## Open questions

None.
