# 0002. Agents push; the server never polls nodes

- **Status:** accepted
- **Date:** 2026-08-28
- **Source:** [POC](../poc.md)

## Context

The classic monitoring approach (Prometheus) has the server scrape nodes over the network.
Laptop-class nodes sit behind NAT, sleep, and change networks; they have no stable address.

## Decision

Agents send measurements themselves over HTTPS to `POST /api/v1/ingest`. Authentication is
a per-node static token in a header, so a single node can be revoked.

## Consequences

- Nodes need no open ports and no public address; they work from any network.
- "The node is quiet" is an expected state, not a failure. The silence detector must
  distinguish node classes: a laptop quiet for two hours is normal, a server quiet for ten
  minutes is an alert. This is a schema parameter (`silence_after` per node class), never a
  constant in code.
- A dashboard without a silence detector lies quietly, so the detector is part of the
  minimum POC scope.

## Alternatives

Pull over a VPN or tunnel to every laptop — rejected: infrastructure built to work around a
problem that push does not have.
