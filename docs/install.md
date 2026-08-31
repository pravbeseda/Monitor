# Installing Monitor

The operator's guide: which commands to type to get a hub host and a node running. What the
result is guaranteed to look like — every path, mode and refusal — is
[specs/deployment.md](specs/deployment.md), and why it looks that way is
[ADR 0019](decisions/0019-deployment-layout.md) and
[ADR 0020](decisions/0020-agent-reads-its-environment-file.md). This file does not repeat
either.

Every name below is synthetic ([ADR 0007](decisions/0007-public-repository.md)):
`hub.example.com` is the hub, `server-b` a Debian node, `laptop-a` a macOS one. Substitute
your own — and keep them out of this repository.

What you need: a checkout of this repository and the Go toolchain `go.mod` names on the
machine you build on, plus `ssh`/`sudo` access to each node. One Debian host runs the hub;
every node, Debian or macOS, runs the agent.

## 1. Build the binaries

No binaries are published yet — [issue #16](https://github.com/pravbeseda/monitor/issues/16)
tracks that — so this is a manual step for now.

From the checkout, cross-compile for a Debian node:

```sh
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o monitor-agent ./cmd/agent
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o monitor-hub ./cmd/hub
```

Use `GOARCH=arm64` for an arm server. The hub cross-compiles because its SQLite driver needs
no cgo ([ADR 0005](decisions/0005-poc-stack.md)).

**The macOS agent must be built on a Mac.** Its disk sensor is cgo
([ADR 0014](decisions/0014-macos-available-space.md)), so `GOOS=darwin` from Linux is not an
option:

```sh
go build -o monitor-agent ./cmd/agent
```

## 2. Set up the hub host

The hub host is set up once, by hand: there is no hub install script
([specs/deployment.md](specs/deployment.md#out-of-scope)). Copy what the host needs:

```sh
scp monitor-hub config.example.yaml deploy/hub.env.example \
    deploy/systemd/monitor-hub.service hub.example.com:
```

Then, on the host, create the unprivileged account the hub runs as and its directories:

```sh
sudo adduser --system --group --no-create-home monitor
sudo mkdir -p /etc/monitor /var/lib/monitor
sudo chown monitor:monitor /var/lib/monitor
sudo chmod 0750 /var/lib/monitor
```

`/etc/monitor` stays owned by root — if this host is also a node, `install-agent.sh` refuses
to write into a directory that is not. The files inside carry the ownership instead:

```sh
sudo install -o root -g root -m 0755 monitor-hub /usr/local/bin/monitor-hub
sudo install -o monitor -g monitor -m 0640 config.example.yaml /etc/monitor/hub.yaml
sudo install -o monitor -g monitor -m 0600 hub.env.example /etc/monitor/hub.env
```

`agent.env` and `hub.env` look alike and are not read alike: systemd reads `hub.env` and
ignores a line it cannot parse, `;` comments included, while the agent reads `agent.env`
itself and refuses the whole file over one such line
([ADR 0020](decisions/0020-agent-reads-its-environment-file.md)). Edit `agent.env` as plain
`KEY=VALUE` lines and `#` comments only — an `export` prefix works in neither file.

Now edit both. `hub.yaml` is the product configuration — nodes, classes, thresholds,
digest, notifier ([specs/hub-config.md](specs/hub-config.md)). `hub.env` holds only secrets:
one token per node, named by that node's `token_env`, plus the Telegram credentials when the
channel is `telegram`. Generate a token per node, long and random:

```sh
openssl rand -base64 32
```

Install the unit and start the service:

```sh
sudo install -o root -g root -m 0644 monitor-hub.service \
    /etc/systemd/system/monitor-hub.service
sudo systemctl daemon-reload
sudo systemctl enable --now monitor-hub.service
systemctl status monitor-hub.service
```

A healthy start writes one line to the journal —
`monitor-hub <version> listening on 127.0.0.1:8080 (nodes: 2, notify: log)` — and the page
answers on the host itself: `curl -s localhost:8080/ | head`. The hub binds to loopback only,
so nothing reaches it from outside until the nginx vhost exists (see
[What is not covered yet](#what-is-not-covered-yet)).

## 3. Install a node

`install-agent.sh` reads the service definition from beside itself, so copy the whole
`deploy/` directory along with the binary:

```sh
scp -r monitor-agent deploy server-b:
```

The token goes in on **stdin**, because `sudo` resets the environment by default and
`MONITOR_TOKEN` would not survive the call; as an argument it would be visible in `ps` to
every local account and would land in shell history, so the script does not accept one there.
On the node:

```sh
read -rs token        # paste the node's token; bash and zsh keep it off the screen
printf %s "$token" | sudo ./deploy/install-agent.sh \
    --binary ./monitor-agent --hub https://hub.example.com --node server-b
unset token
```

Copy the binary and `deploy/` together: the service definitions pass `--env-file`, which an
agent built before them does not know, and the install would report success on a service that
exits every time it starts.

The macOS node is the same command with its own name (`--node laptop-a`); the script picks
systemd or launchd from the system it is running on. It prints every path it wrote and the
command that shows the service's state.

To see what a run would write without touching the machine, stage it under a prefix — this
registers no service and needs no root:

```sh
DESTDIR=/tmp/staged ./deploy/install-agent.sh \
    --binary ./monitor-agent --hub https://hub.example.com --node server-b
```

## 4. Verify

On Debian:

```sh
systemctl status monitor-agent.service
journalctl -u monitor-agent.service -f
```

On macOS, where launchd has no journal, the log is a file readable by root only:

```sh
sudo launchctl print system/io.github.pravbeseda.monitor-agent
sudo tail -f /var/log/monitor-agent.log
```

A healthy first minute: the service is active, the log opens with
`monitor-agent <version>: node server-b reporting to https://hub.example.com`, the first tick
runs immediately rather than after an interval, and no `tick failed` line repeats. Within a
base tick the node and its volumes appear on the hub's page.

Two failures look different from a service problem and are worth knowing:

- **No token in the environment file** — the agent exits at startup naming `MONITOR_TOKEN`,
  and the supervisor keeps restarting it, so the message keeps coming — every five seconds on
  Debian, every ten or so on macOS, where launchd sets the throttle. Re-run the install with
  the token.
- **A wrong token** — the service stays happily up. The hub refuses the batches, so the node
  never appears with fresh values and eventually turns silent. Check `hub.env` on the hub
  host against what you installed on the node.

## 5. Upgrade a node, rotate its token

Both are a re-run of the same script, and `--hub` and `--node` are given every time.

An upgrade — build a new binary, copy it over, and run without a token:

```sh
sudo ./deploy/install-agent.sh \
    --binary ./monitor-agent --hub https://hub.example.com --node server-b
```

A rotation is the same run with the new token piped in, exactly as in step 3.

A re-run **replaces** the binary, the service definition, `MONITOR_HUB` and `MONITOR_NODE`
from the flags, and the token when one is supplied. It **keeps** the stored token when none
is, and every other line of a hand-edited environment file. It ends by restarting the
service, so the run is done when the service is back up.

Rotating a token is two-sided: the hub reads `hub.env` when it starts, so change the node's
variable there and restart it too.

```sh
sudo systemctl restart monitor-hub.service
```

## 6. Uninstall

Deliberately not a mode of the script — it is these commands
([specs/deployment.md](specs/deployment.md#out-of-scope)).

A Debian node:

```sh
sudo systemctl disable --now monitor-agent.service
sudo rm /etc/systemd/system/monitor-agent.service /usr/local/bin/monitor-agent \
    /etc/monitor/agent.env
sudo systemctl daemon-reload
```

A macOS node:

```sh
sudo launchctl bootout system/io.github.pravbeseda.monitor-agent
sudo rm /Library/LaunchDaemons/io.github.pravbeseda.monitor-agent.plist \
    /usr/local/bin/monitor-agent /usr/local/etc/monitor/agent.env /var/log/monitor-agent.log
```

The hub host, the same way:

```sh
sudo systemctl disable --now monitor-hub.service
sudo rm /etc/systemd/system/monitor-hub.service /usr/local/bin/monitor-hub
sudo systemctl daemon-reload
```

That leaves the configuration and the measurements — `/etc/monitor/hub.yaml`,
`/etc/monitor/hub.env` and `/var/lib/monitor` — standing. Removing them is a separate,
deliberate step: the database is the whole history.

## What is not covered yet

The other half of the stage-3 bullet in [poc.md](poc.md) does not exist yet, and nothing
above works around it: the nginx vhost and TLS in front of the hub, per-node token issuance,
and authentication on the web page. Until they land, the hub is reachable on its own host
only, over loopback.

Published binaries are [issue #16](https://github.com/pravbeseda/monitor/issues/16); until
then step 1 is a manual build.
