# 0019. Install into system paths, with one environment file per binary

- **Status:** accepted; the launchd bullet is amended by
  [0020](0020-agent-reads-its-environment-file.md), which has the agent read the environment
  file itself instead of a shell sourcing it
- **Date:** 2026-08-31
- **Source:** [POC](../poc.md) stage 3, [deployment spec](../specs/deployment.md)

## Context

[0005](0005-poc-stack.md) fixed systemd on Debian and launchd on macOS but said nothing
about where anything goes. Three things then have to agree: the unit files, the install
script and the guide — and [0007](0007-public-repository.md) forbids any of them from
carrying a host name, a node name, a URL or a token, all of which a running service needs.

systemd and launchd do not offer the same tools. systemd reads an environment file natively;
launchd has no equivalent, and a daemon plist in `/Library/LaunchDaemons` is conventionally
world-readable, so a token placed in its `EnvironmentVariables` is a token every local user
can read.

## Decision

- **System paths, not a private tree**: `/usr/local/bin/monitor-{agent,hub}`,
  `/etc/monitor/` (`/usr/local/etc/monitor/` on macOS), `/var/lib/monitor/monitor.db`, and
  the service definition where each system keeps them. The service runs system-wide, not as
  a login agent, so a node that nobody has logged into still reports.
- **One environment file per binary carries everything that varies** — for the agent,
  `MONITOR_HUB`, `MONITOR_NODE` and `MONITOR_TOKEN`; for the hub, its notifier credentials
  and the per-node tokens `hub.yaml` names. Mode `0600`, owned by the account that service
  runs as.
- **The agent runs as root and the hub does not.** The agent stats every mounted volume and
  a launchd daemon is a root process anyway; the hub only listens on a socket, so it runs as
  an unprivileged `monitor` account created once when the host is set up. That account owns
  `/var/lib/monitor` and `hub.env`.
- **The unit files are therefore constants.** systemd expands `${MONITOR_HUB}` from the
  environment file in `ExecStart`; the launchd plist runs
  `/bin/sh -c '. …/agent.env && exec monitor-agent …'`, which is what gives launchd the same
  ability. The plist keeps no secret, so it may stay world-readable like every other daemon.
- **`install-agent.sh` installs a binary that already exists** (`--binary`), never builds
  one, and never takes the token as an argument — it reads `MONITOR_TOKEN`, or stdin, which
  is the route that survives `sudo` resetting the environment.
- **`DESTDIR` stages an install under a prefix** and suppresses every service command, which
  is how the script is tested.

## Consequences

- A node is upgraded and a token rotated by re-running the same script; the run is
  idempotent, and it keeps a stored token when a new one is not supplied.
- Cross-compiling is a manual step until artifacts are published
  ([#16](https://github.com/pravbeseda/monitor/issues/16)). The darwin binary is built on a
  Mac, because its disk sensor is cgo ([0014](0014-macos-available-space.md)).
- The agent gains no configuration file: [0010](0010-agent-configuration.md) still holds, and
  the environment file holds exactly the three values that ADR allows a node to know.
- Moving a path later means editing the unit, the script and the guide together — the reason
  the spec fixes them in one table.

## Alternatives

- **A self-contained `/opt/monitor` tree** — rejected: the systemd unit lives in `/etc`
  regardless, `/opt` has no meaning on macOS, and the single benefit (one directory to
  delete) is one line in the guide instead.
- **A user-level service** (`systemd --user`, a launchd LaunchAgent) — rejected: it stops
  when the user logs out, and on macOS it meets TCC restrictions on some volumes. A monitor
  that quietly stops measuring is worse than one that needs `sudo` once.
- **The token inside the launchd plist** — rejected: world-readable by convention, and it
  would mean two secret formats and two rotation procedures.
- **The macOS keychain** — rejected for the POC: a third storage mechanism, a system-keychain
  dance for a root daemon, and rotation stops being "edit one line".
- **Placeholders substituted into the unit at install time** (`__HUB__`) — rejected: it makes
  the installed unit differ from the shipped one, so an upgrade has to re-render it and a
  changed URL means editing a systemd unit rather than an environment file.
- **Building on the node** — rejected: a Go toolchain on every machine to produce a binary
  the developer's machine can cross-compile in one command.
- **A `--token` flag** — rejected: arguments are visible in `ps` to every user on the box and
  land in shell history. `MONITOR_TOKEN` covers a root shell, stdin covers `sudo`, and
  between them no invocation needs the secret on the command line.
- **Running the hub as root as well** — rejected: it needs no privilege, and one account
  created by hand is cheaper than a network listener with none.
- **A `--prefix` flag, or a fakeroot/chroot test harness, instead of `DESTDIR`** — rejected:
  `DESTDIR` is the convention every packager already knows, it needs no privileges and no
  extra tool, and it keeps one code path for the real install and the staged one.
