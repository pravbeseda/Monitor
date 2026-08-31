# Spec: Deployment

- **Status:** draft
- **Owns:** `deploy/`: the systemd units, the launchd daemon, the environment file and
  `install-agent.sh`
- **Decisions:** [0005](../decisions/0005-poc-stack.md),
  [0007](../decisions/0007-public-repository.md),
  [0010](../decisions/0010-agent-configuration.md),
  [0019](../decisions/0019-deployment-layout.md)

## Purpose

A node runs the agent as a system service that survives a reboot and a crash, and the host
runs the hub the same way. This spec owns what an installation looks like on disk, what the
service guarantees, and what `install-agent.sh` does to a node — nothing about what the
binaries then do.

It deliberately covers **one** half of the operational work: getting the processes running
and supervised. TLS, the nginx vhost, per-node token issuance and authentication on the web
page are the other half, and neither the units nor the script make any statement about them.

## Where things live

Every path is fixed by [0019](../decisions/0019-deployment-layout.md). Nothing here varies
per installation, which is what lets the unit files be constants.

| What | Debian | macOS |
|---|---|---|
| agent binary | `/usr/local/bin/monitor-agent` | `/usr/local/bin/monitor-agent` |
| agent settings and token | `/etc/monitor/agent.env` | `/usr/local/etc/monitor/agent.env` |
| agent service | `/etc/systemd/system/monitor-agent.service` | `/Library/LaunchDaemons/io.github.pravbeseda.monitor-agent.plist` |
| hub binary | `/usr/local/bin/monitor-hub` | — |
| hub settings and secrets | `/etc/monitor/hub.env` | — |
| hub configuration | `/etc/monitor/hub.yaml` | — |
| hub database | `/var/lib/monitor/monitor.db` | — |
| hub service | `/etc/systemd/system/monitor-hub.service` | — |

The hub is a Debian service only ([0005](../decisions/0005-poc-stack.md)); the agent runs on
both.

## The environment file

One file carries everything an installation knows, because everything an installation knows
is a secret or a deployment setting and none of it may live in this repository
([0007](../decisions/0007-public-repository.md)). The agent's three local values
([agent.md](agent.md)) are exactly its three keys.

| Key | In | Meaning |
|---|---|---|
| `MONITOR_HUB` | `agent.env` | base URL of the hub |
| `MONITOR_NODE` | `agent.env` | this node's name, as the hub knows it |
| `MONITOR_TOKEN` | `agent.env` | this node's token |
| `MONITOR_TELEGRAM_TOKEN`, `MONITOR_TELEGRAM_CHAT_ID` | `hub.env` | notifier credentials, when the channel is Telegram |
| `MONITOR_TOKEN_<NODE>` | `hub.env` | the token the named node presents |

## Behaviour

One row = one test. Anchors: `spec: deployment.md#<heading>`.

### Installing on a fresh node

The script installs a binary that was built elsewhere; it never builds one
([0019](../decisions/0019-deployment-layout.md)).

| State | Event | Behaviour |
|---|---|---|
| nothing installed | `install-agent.sh --binary B --hub U --node N`, token in `MONITOR_TOKEN` | the binary lands executable, the environment file holds the three values and is readable by its owner only, the service is registered and running |
| nothing installed | the same, with the token on stdin instead | identical: a token is never a command-line argument |
| nothing installed | any successful run | it prints every path it wrote and the command that shows the service's state |

### Refusing

A refusal writes nothing at all: a half-installed node is worse than an uninstalled one.

| State | Event | Behaviour |
|---|---|---|
| any | `--binary` names a file that is missing or not executable | refuses, naming the path |
| any | `--hub` or `--node` missing | refuses, naming the flag |
| any | no token in the environment and none on stdin | refuses, naming `MONITOR_TOKEN` |
| any | an unknown flag | refuses, printing the usage |
| any | the host has neither systemd nor launchd | refuses, naming what is supported |
| a real install | the invoking user is not root | refuses, saying to re-run under `sudo` |

### Re-running

Re-running is how a node is upgraded and how a token is rotated, so it is the ordinary case
rather than an error.

| State | Event | Behaviour |
|---|---|---|
| already installed | a run with a newer binary | the binary is replaced and the service restarted |
| already installed | a run with the same arguments and the same binary | every installed file ends byte-identical, and the service is running |
| already installed | a run with a different `--hub` or `--node` | the environment file carries the new value and the service is restarted |
| already installed | a run whose token differs | the environment file carries the new token, still readable by its owner only |
| already installed | a run with no token available, having installed one before | the stored token is kept and the run succeeds |

### The service

| State | Event | Behaviour |
|---|---|---|
| installed | the node reboots | the agent starts without anyone logging in |
| installed | the agent exits, whatever the status | it is restarted after a short delay |
| installed | the agent writes to stdout or stderr | the lines reach the system log |
| installed | the token is missing or wrong | the agent exits and is restarted; the failure is in the system log, not in the exit of the install |

### Staged installs

`DESTDIR` puts a whole installation under a prefix, as a package build would. It is what
makes the behaviour above testable without touching the machine running the test.

| State | Event | Behaviour |
|---|---|---|
| any | `DESTDIR` set | every path is written beneath it, with the same names and the same modes |
| any | `DESTDIR` set | no service is registered, started or restarted, and root is not required |

## Invariants

- No file this repository ships names a host, a node, a URL or a token: the unit files are
  the same on every installation, and everything that differs is in the environment file
  ([0007](../decisions/0007-public-repository.md)).
- The token is never an argument to a command, never printed, and never lands in a file
  another user can read.
- A failed install leaves the node as it was; a successful one leaves a service that starts
  on boot.

## Edge cases

- **A second install while the service is running** — the binary is replaced under a running
  process, which is why the run ends with a restart rather than a reload.
- **A token that is already installed and not supplied again** — kept. The alternative, an
  empty token written over a working one, breaks a node during a routine upgrade.
- **The environment file edited by hand** — supported; the script rewrites only the keys it
  was given values for.
- **A binary for the wrong operating system** — not detected. It installs, the service fails
  to start, and the system log says so.

## Out of scope

- The nginx vhost, TLS, web authentication and per-node token issuance — the other half of
  the stage-3 bullet in [poc.md](../poc.md).
- Building or shipping the binary: the script takes one that exists
  ([issue #16](https://github.com/pravbeseda/monitor/issues/16) tracks published artifacts).
- Uninstalling: two documented commands in the install guide, not a mode of the script.
- Installing the hub. Its unit ships here, and the guide says where to put it; a host that
  runs the hub is set up once and by hand.

## Open questions

None.
