# 0003. Sensors are in-process modules of the agent

- **Status:** accepted
- **Date:** 2026-08-28
- **Source:** [POC](../poc.md)

## Context

The original sketch had the agent talking to separate driver applications over IPC. For a
POC with one kind of reading that is a whole inter-process layer to build, version, debug
and deploy on every node.

## Decision

Sensors are modules inside the agent behind `Sensor: collect() -> []Measurement`. An
external sensor — a separate binary writing JSON to stdout — is a future *implementation of
the same interface*, not a separate architecture.

## Consequences

- The sensor interface must not reveal whether an implementation is built in or external.
  That is the condition that makes this decision correct, not an implementation detail.
- External sensors arrive when a sensor is written in another language, by another author,
  or when its code would itself disclose private facts — see
  [0007](0007-public-repository.md). Not before.

## Alternatives

IPC from day one — rejected: paid for now, useful at an unknown later date.
