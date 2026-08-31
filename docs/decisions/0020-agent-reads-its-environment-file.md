# 0020. The agent reads its own environment file; no shell sources it

- **Status:** accepted
- **Date:** 2026-08-31
- **Source:** a security review of the stage-3 service definitions
- **Amends:** the launchd bullet of [0019](0019-deployment-layout.md); its paths and its
  one-environment-file-per-binary rule stand

## Context

[0019](0019-deployment-layout.md) gave each binary one environment file and made the unit
files constants. systemd reads such a file with its own parser, as inert data. launchd has
no equivalent, so the plist ran
`/bin/sh -c 'set -a && . /usr/local/etc/monitor/agent.env && … exec monitor-agent …'`.

POSIX `.` *executes* the file: it is a shell script, not a data format. A token pasted with
a space in it — `MONITOR_TOKEN=abcd efgh` — makes the shell run `efgh` and write
`sh: efgh: command not found` into the agent's log, which puts the tail of the secret in the
file an operator pastes into a bug report. A value containing `$(…)` or a backtick runs as
root before the agent starts. Quoting every value avoids both, but nothing enforces the
quoting and nothing announces its absence.

So one file had two parsers, and only the dangerous one held a secret.

## Decision

- **The agent takes `--env-file <path>` and reads the file itself.** Blank lines and lines
  starting with `#` are skipped; a line is `KEY=VALUE`; one matching pair of surrounding
  single or double quotes is stripped; the value is otherwise literal, with no expansion, no
  command substitution and no execution of any kind. Keys the agent does not know are
  ignored, as systemd ignores them. An unreadable file names the path in its error; a line
  that is not `KEY=VALUE`, or that opens a quote and never closes it, names its number and
  no part of its text — the error goes to a log, and any part of the line may be the token,
  the text before the first `=` included, since a base64 token ends in `=`.
- **Precedence is one rule: anything given explicitly wins over the file** — `--hub` over
  `MONITOR_HUB`, `--node` over `MONITOR_NODE`, and `MONITOR_TOKEN` in the process
  environment over the file's token. Under a service nothing else is set, so the file
  supplies all three.
- **Both service definitions become the same command.** The systemd agent unit drops
  `EnvironmentFile=` and its `${…}` expansions, the launchd plist drops the shell, and each
  starts `/usr/local/bin/monitor-agent --env-file` with the path [0019](0019-deployment-layout.md)
  fixes for its system.
- **The hub is unchanged**: systemd's parser is safe, and there is no launchd hub.

## Consequences

- One file, one parser, on both systems: a value that works on Debian works on macOS, and
  the rules it obeys are tested rather than documented.
- No deployment setting reaches a service's arguments any more. The hub URL and the node
  name used to sit in `ps` for every local account.
- `--hub` and `--node` stay, which is what keeps running the agent by hand a one-liner in
  development; the file is what a service uses.
- The parser is the agent's to maintain, and it deliberately accepts less than systemd does:
  no `;` comment lines, no line continuations, no `$` of any meaning. Where systemd warns and
  carries on, this refuses — an unclosed quote is an error rather than a token that silently
  keeps its quote and authenticates nowhere.
- Nothing about the install changes: the same file, at the same path, with the same mode.

## Alternatives

- **A safe shell loop in the plist** — `while IFS= read -r line; do export "$line"; done`
  instead of `.` — rejected: `export` still parses its argument, comments and quoting have
  to be handled by hand in `sh`, and the whole thing lives as one XML-escaped string that no
  test can reach. It is a parser either way; a parser in Go can be tested.
- **Keeping `.` and making the value safe** — restrict the log to root and require every
  value in the file to be shell-quoted — rejected: correctness becomes a property of what an
  operator types, the failure is silent until it is not, and root still executes a file whose
  only purpose is to hold a pasted secret.
- **The token in the plist's `EnvironmentVariables`** — rejected by
  [0019](0019-deployment-layout.md) already: a daemon plist is world-readable by convention.
- **Two mechanisms — `EnvironmentFile=` on systemd, `--env-file` on launchd** — rejected:
  the point is one file with one parser. Two would drift, and the systemd path would be the
  one nothing exercises.
