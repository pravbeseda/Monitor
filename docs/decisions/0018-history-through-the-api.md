# 0018. History is served by the hub's API; a chart is one of its consumers

- **Status:** accepted
- **Date:** 2026-08-31
- **Source:** [POC](../poc.md) stage 3, the
  [munin proposal](../log/2026-08-29-munin-hub-plugin.md) it decides against

## Context

Stage 3 asks how the history of a metric reaches a human, and says to decide it before any
charting is built by hand. The POC is done only when the value history of the observation
period is visible, and the hub already holds every raw point it ever accepted: retention
keeps them all ([poc.md](../poc.md), answered question 7).

Two shapes were on the table. Take charting off the shelf — a munin plugin on the hub's
host, RRD storage, day/week/month/year graphs for an evening of work — or serve the series
from the hub and draw them in the web page.

The off-the-shelf route buys real things: multi-resolution graphs, a zoom UI, several
series on one canvas. It also assumes a collector that polls at a fixed period and stores
what it read at the instant it read it. This hub is push
([0002](0002-push-not-pull.md)): a laptop reports once an hour, carries its own timestamps,
and goes quiet for a day at a time by design. A five-minute poll of the latest value would
draw twelve copies of one reading, and points that arrive late — the normal outcome of an
agent that was offline — can never be written into an RRD behind its last update. The
graphs would be smooth and wrong exactly where the product is supposed to be truthful.

## Decision

**The hub serves history as data, over `/api/v1/history`, and every renderer is a consumer
of that endpoint.** The POC ships one consumer: a drill-down page in the hub's own web skin
that draws an SVG chart server-side, with no client-side charting library
([0005](0005-poc-stack.md)).

The series every renderer receives are assembled by a package that knows nothing about SVG
or HTTP. The built-in page reads them in process; anything outside the hub — a munin plugin,
a Grafana datasource, a future skin ([0001](0001-semantic-core-and-skins.md)) — reads the
same series over the endpoint. That is what a replacement costs: a new consumer, not a
change to the reading. `/api/v1/series` ships with it, so a consumer can learn which nodes,
metrics and label sets exist instead of being configured with deployment facts
([0007](0007-public-repository.md)).

## Consequences

- Every renderer sees the same series, with the agent's own timestamps and the real gaps
  where a node was silent.
- The seam is the series and the endpoint that publishes them. No `Renderer` interface is
  introduced for a single implementation; a second way of drawing is a second consumer.
- The hub gains a second page, which [0005](0005-poc-stack.md) did not foresee when it said
  "one page". The constraint that mattered there stands: server-rendered HTML, no SPA, no
  frontend build.
- The endpoints publish every measurement the hub holds, so they cannot be exposed before
  the stage 3 authentication work, and that work has to issue a credential a program can
  send: a browser session alone would lock out every consumer named above.
- The hub owns the only copy of the history. No second store is kept in sync, and a chart
  cannot disagree with an alert about what a value was.
- `at=` time travel and the event stream of [0001](0001-semantic-core-and-skins.md) extend
  this endpoint later instead of being bolted onto a renderer.
- The chart is work this project has to maintain: axes, ticks and a window switcher are
  ours, and the multi-resolution graphs munin gives away are not free here.

## Alternatives

- **A munin plugin on the hub's host reading the SQLite database** — rejected: its poll
  model cannot represent push timestamps, late points or deliberate silence, and it would
  hold a second copy of the history behind a second URL with its own authentication. The
  proposal and what it was weighed against stay in the
  [design note](../log/2026-08-29-munin-hub-plugin.md).
- **Grafana over the SQLite file** — rejected for the POC: a heavy dependency on the hub's
  host, and a second surface where thresholds and "good/bad" can be declared, which
  [0001](0001-semantic-core-and-skins.md) reserves for the semantic core. It stays available
  as a consumer of the endpoint, which is the point of deciding this way.
- **A client-side charting library over the same endpoint** — rejected for the POC, not
  refused: it would vendor a JavaScript asset into a repository whose web skin is
  server-rendered HTML ([0005](0005-poc-stack.md)). The endpoint makes this a change of one
  consumer if interactive zoom is ever wanted.
