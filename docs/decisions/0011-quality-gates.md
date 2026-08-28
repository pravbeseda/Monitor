# 0011. Quality is enforced by tooling, not by attention

- **Status:** accepted
- **Date:** 2026-08-28
- **Source:** [POC](../poc.md) question 1

## Context

Go was chosen over a stack the author already reads fluently
([0005](0005-poc-stack.md)). That trade is only sound if the author can tell good code from
bad without reading Go closely — which means the checks have to be machine-run and
mandatory, not a matter of reviewer diligence.

## Decision

Four checks gate every change. **CI runs all four and a red one blocks the merge**; the
pre-commit hook runs the fast pair (`gofmt`, `go vet`) so the local loop stays quick:

| Check | Where | Catches |
|---|---|---|
| `gofmt -l` (empty output required) | hook + CI | formatting drift — Go has one canonical style, so this is not a preference |
| `go vet` | hook + CI | constructs that compile and misbehave |
| `golangci-lint` | CI | `errcheck`, `staticcheck`, `ineffassign`, `gocritic`, `revive` |
| `go test -race -cover` | CI | failing tests and data races in the agent's concurrent collection |

The guarantee belongs at the boundary of `main`, not at the boundary of a local commit: a
branch may hold broken work, a merge may not. A hook slow enough to invite `--no-verify` is
worse than a fast one, because a bypassed hook is invisible while a red pipeline is not.

`errcheck` earns its place explicitly: an ignored error is the most common way to write code
that fails silently, which is the failure mode this project exists to detect in others.

## Consequences

- The author's review is the behaviour table and a green pipeline, not the syntax.
- A pull request is not ready while any check is red; nothing is merged past it.
- Committing to a branch with a failing test is allowed; merging with one is not.
- These gates arrive with the first Go code, not before it: there is nothing to check yet.

## Alternatives

- **Rely on review** — rejected: it puts the burden on the person least equipped to carry it
  here, and it is exactly the guarantee tooling gives for free.
- **Warnings instead of blocking** — rejected: a warning nobody must act on is a warning
  nobody acts on.
