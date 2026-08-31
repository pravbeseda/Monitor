# 0021. The shell the project ships is linted like its Go

- **Status:** accepted
- **Date:** 2026-08-31
- **Source:** [0011](0011-quality-gates.md), [deployment spec](../specs/deployment.md)
- **Extends:** [0011](0011-quality-gates.md); its four gates and its reasoning stand

## Context

[0011](0011-quality-gates.md) chose Go partly because its quality tooling is one toolchain
rather than four projects assembled by hand, and named four gates — all of them Go's.

Stage 3 ships `deploy/install-agent.sh`: a POSIX `sh` script that runs as root, writes a
secret and installs a service. It joins `.githooks/pre-commit`, the hook
[0011](0011-quality-gates.md) itself leans on — the two artifacts in the repository no Go gate
can see, and shell is where an unquoted expansion is a defect rather than a style opinion. The
installer's own Go test drives it, which catches behaviour; nothing catches the constructs a
shell accepts and misreads, and the hook had one: `gofmt -l $gofiles` split on every blank,
not only on the newlines that separate the staged paths.

## Decision

`shellcheck -s sh` runs in CI over the shell this repository ships — the installer and the
pre-commit hook — and a finding blocks the merge like any other gate. `-s sh` reads the script as the POSIX shell its shebang claims and
as Debian's `/bin/sh` — dash — actually is, rather than as bash.

It runs on the Ubuntu runner only: the lint does not vary by operating system, `shellcheck` is
part of that runner's image, and nothing promises it on the macOS one.

## Consequences

- The gate list of [0011](0011-quality-gates.md) is five, and this ADR is where the fifth
  lives; that ADR keeps its four and its reasoning.
- A contributor needs `shellcheck` locally to reproduce the check (`brew install shellcheck`,
  `apt install shellcheck`). The pre-commit hook does not run it: the hook is the fast pair,
  and the shell changes rarely.
- Shell added anywhere else in the repository is expected under the same gate, which is one
  more reason for there to be very little of it.

## Alternatives

- **Leave the script to its Go test.** Rejected: the test asserts what the script does on the
  paths it exercises; `shellcheck` reads the paths nobody thought to exercise, which is the
  half a root-run installer cannot afford to leave to attention ([0011](0011-quality-gates.md)
  is the same argument).
- **Run it on both runners.** Rejected: the same file, the same linter, the same answer — and
  a `brew install` on the macOS runner to reach it.
- **Add it to the pre-commit hook.** Rejected: the hook is deliberately the fast pair, and a
  hook that needs a tool a fresh clone may not have blocks a commit that has nothing to do
  with shell.
- **Rewrite the installer in Go.** Rejected: a static binary that installs a static binary and
  then talks to systemd or launchd buys nothing a 200-line script does not already do, and it
  would need building on the node — which [0019](0019-deployment-layout.md) exists to avoid.
