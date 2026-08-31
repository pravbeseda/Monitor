#!/bin/sh
# Install the monitor agent as a system service. Every path, mode and refusal here is
# docs/specs/deployment.md; nothing is configurable and no binary is built.
#
#   printf %s "$token" | sudo ./install-agent.sh --binary ./monitor-agent \
#       --hub https://hub.example.com --node laptop-a
#
# The token comes from MONITOR_TOKEN or from stdin, never from an argument: arguments are
# visible in `ps` to every local account and land in shell history.

set -eu

# Every file mode below is set explicitly, but a directory's is not: without this, a caller
# whose umask is 0 would leave /etc/monitor writable by anyone.
umask 022

program=$(basename "$0")
source_dir=$(dirname "$0")

# What has already reached the disk. A run that fails after its first write stops there and
# names this list; it neither continues nor rolls back (deployment.md#invariants).
written=

on_exit() {
	status=$?
	if [ "$status" -ne 0 ] && [ -n "$written" ]; then
		printf '%s: stopped after writing:\n%s' "$program" "$written" >&2
	fi
	exit "$status"
}
trap on_exit EXIT

usage() {
	cat >&2 <<EOF
usage: $program --binary <path> --hub <url> --node <name>

The token is read from MONITOR_TOKEN, or from stdin when that is unset — the route that
survives sudo resetting the environment:

  printf %s "\$token" | sudo ./$program --binary ./monitor-agent \\
      --hub https://hub.example.com --node laptop-a

DESTDIR stages the whole installation under a prefix and registers no service.
EOF
}

refuse() {
	printf '%s: %s\n' "$program" "$1" >&2
	exit 1
}

# Blanks around a line, around a key and around a value are what systemd's EnvironmentFile
# and the agent's own parser both ignore, so this reader ignores them too.
trim() {
	trimmed=$1
	while :; do
		case $trimmed in
		" "* | "	"*) trimmed=${trimmed#?} ;;
		*" " | *"	") trimmed=${trimmed%?} ;;
		*) return 0 ;;
		esac
	done
}

# Strip one matching pair of surrounding quotes, as the agent's parser does: the quotes say
# where the value ends, nothing more.
unquote() {
	unquoted=$1
	case $unquoted in
	"'"*"'" | '"'*'"')
		unquoted=${unquoted#?}
		unquoted=${unquoted%?}
		;;
	esac
}

# Split one line of the environment file into $key and $value by the agent's own rules
# (ADR 0020), and fail for a line that assigns nothing. A line the agent reads as a key has
# to be a line this script recognises and rewrites in place: one it read more narrowly would
# leave a hand-written token invisible on a re-run, and a revoked one on disk under a second
# MONITOR_TOKEN after a rotation (deployment.md#edge-cases).
env_line() {
	trim "$1"
	case $trimmed in
	*=*) assignment=$trimmed ;;
	*) return 1 ;;
	esac
	trim "${assignment%%=*}"
	key=$trimmed
	trim "${assignment#*=}"
	unquote "$trimmed"
	value=$unquoted
}

# Read one value out of the environment file without executing a line of it: the file holds
# a pasted secret, and POSIX `.` would run it (ADR 0020).
stored_value() {
	stored=
	line=
	if [ -f "$1" ]; then
		while IFS= read -r line || [ -n "$line" ]; do
			if env_line "$line" && [ "$key" = "$2" ]; then
				stored=$value
			fi
		done <"$1"
	fi
}

# Create an empty file at a path nothing else may have prepared. Unlinking first and then
# refusing to clobber (`set -C` opens with O_EXCL) is what stops a name another account
# planted from being written through: a symlink there would make root write into that
# account's file.
create_file() {
	rm -f "$1"
	(
		umask "$2"
		set -C
		: >"$1"
	)
}

# A temporary next to $1, under a name nothing can predict, in $temp. Every write after the
# creation re-opens the path by name and would follow a symlink dropped there in between, and
# the directory need not be root's — on a Homebrew Mac /usr/local/etc is not. mktemp creates
# it 0600, which is the environment file's own mode; a caller that needs another one chmods
# before the rename.
create_temp() {
	mkdir -p "$(dirname "$1")"
	temp=$(mktemp "$1.XXXXXXXX")
}

# Copy one file into place with the mode the layout table gives it. The temporary and the
# rename are what let a running agent's binary be replaced under it.
install_file() {
	create_temp "$3"
	cp "$1" "$temp"
	chmod "$2" "$temp"
	mv "$temp" "$3"
	written="$written  $3
"
}

# Rewrite MONITOR_HUB, MONITOR_NODE and MONITOR_TOKEN, and leave every other line of an
# existing file alone — a file edited by hand is supported (deployment.md#edge-cases).
write_env_file() {
	create_temp "$1"

	has_hub=0
	has_node=0
	has_token=0
	line=
	if [ -f "$1" ]; then
		while IFS= read -r line || [ -n "$line" ]; do
			if env_line "$line"; then
				case $key in
				MONITOR_HUB)
					line="MONITOR_HUB=$hub"
					has_hub=1
					;;
				MONITOR_NODE)
					line="MONITOR_NODE=$node"
					has_node=1
					;;
				MONITOR_TOKEN)
					line="MONITOR_TOKEN=$token"
					has_token=1
					;;
				esac
			fi
			printf '%s\n' "$line"
		done <"$1" >>"$temp"
	fi
	[ "$has_hub" -eq 1 ] || printf 'MONITOR_HUB=%s\n' "$hub" >>"$temp"
	[ "$has_node" -eq 1 ] || printf 'MONITOR_NODE=%s\n' "$node" >>"$temp"
	[ "$has_token" -eq 1 ] || printf 'MONITOR_TOKEN=%s\n' "$token" >>"$temp"

	mv "$temp" "$1"
	written="$written  $1
"
}

binary=
hub=
node=

while [ "$#" -gt 0 ]; do
	case $1 in
	--binary)
		[ "$#" -ge 2 ] || refuse "$1 needs a value"
		binary=$2
		shift 2
		;;
	--hub)
		[ "$#" -ge 2 ] || refuse "$1 needs a value"
		hub=$2
		shift 2
		;;
	--node)
		[ "$#" -ge 2 ] || refuse "$1 needs a value"
		node=$2
		shift 2
		;;
	*)
		printf '%s: unknown option: %s\n' "$program" "$1" >&2
		usage
		exit 1
		;;
	esac
done

# Everything below refuses before the first write, so a rejected run leaves the node exactly
# as it was (deployment.md#refusing).
[ -n "$binary" ] || refuse "--binary is required"
[ -n "$hub" ] || refuse "--hub is required"
[ -n "$node" ] || refuse "--node is required"
[ -f "$binary" ] || refuse "--binary names no file: $binary"
[ -x "$binary" ] || refuse "--binary names a file that is not executable: $binary"

binary_file=/usr/local/bin/monitor-agent
case $(uname -s) in
Linux)
	env_file=/etc/monitor/agent.env
	service_source=$source_dir/systemd/monitor-agent.service
	service_file=/etc/systemd/system/monitor-agent.service
	log_file=
	init_tool=systemctl
	status_command='systemctl status monitor-agent.service'
	;;
Darwin)
	env_file=/usr/local/etc/monitor/agent.env
	service_source=$source_dir/launchd/io.github.pravbeseda.monitor-agent.plist
	service_file=/Library/LaunchDaemons/io.github.pravbeseda.monitor-agent.plist
	# launchd creates a missing StandardOutPath world-readable on first start, so the log is
	# a file this install owns (deployment.md#where-things-live).
	log_file=/var/log/monitor-agent.log
	init_tool=launchctl
	service_label=io.github.pravbeseda.monitor-agent
	status_command='launchctl print system/io.github.pravbeseda.monitor-agent'
	;;
*)
	refuse "unsupported system: the agent installs on Debian (systemd) and macOS (launchd)"
	;;
esac
[ -f "$service_source" ] || refuse "the service definition is missing: $service_source"

# DESTDIR stages the whole installation under a prefix: no root, no init system, no service
# command (deployment.md#staged-installs).
destdir=${DESTDIR:-}
if [ -z "$destdir" ]; then
	[ "$(id -u)" -eq 0 ] || refuse "must run as root: re-run under sudo"
	# Nothing but this script creates the environment file's directory, so requiring root to
	# own it costs a legitimate installation nothing and closes both holes in a directory an
	# unprivileged account owns: a MONITOR_TOKEN line planted there for the read below to
	# adopt, and a name planted there for root to write the token through. `find -user` is
	# the ownership test both BSD and GNU have; `stat` is not.
	env_dir=$(dirname "$env_file")
	if [ -d "$env_dir" ] && [ -z "$(find "$env_dir" -maxdepth 0 -user root)" ]; then
		refuse "the environment file's directory is not owned by root: $env_dir"
	fi
	command -v "$init_tool" >/dev/null 2>&1 ||
		refuse "this host has neither systemd nor launchd; the agent installs on Debian and macOS"
fi

token=${MONITOR_TOKEN:-}
unset MONITOR_TOKEN # no child process needs it in its environment
if [ -z "$token" ] && [ ! -t 0 ]; then
	IFS= read -r token || :
fi
if [ -z "$token" ]; then
	# A token already installed and not supplied again is kept: writing an empty one over a
	# working one would break a node during a routine upgrade (deployment.md#re-running).
	stored_value "$destdir$env_file" MONITOR_TOKEN
	token=$stored
fi
[ -n "$token" ] || refuse "no token: set MONITOR_TOKEN or pipe the token in on stdin"

# A value the agent's parser cannot read back as itself is refused here, before anything is
# written. Each message names no value: one of them is the token.
newline='
'
carriage_return=$(printf '\r')
for checked in "$hub" "$node" "$token"; do
	# The environment file is one KEY=VALUE per line, and a lone carriage return ends a line
	# for the agent too (ADR 0020), so either one would write a second line the agent reads
	# as configuration — a hub URL the operator never passed, on a node that reports there
	# silently.
	case $checked in
	*"$newline"* | *"$carriage_return"*)
		refuse "a value contains a line break, and the environment file is one KEY=VALUE per line"
		;;
	esac
	# A value that opens with a quote and never closes it makes the agent refuse the whole
	# file, so the install would succeed on a node that never starts again.
	case $checked in
	"'"*"'" | '"'*'"') ;;
	"'"* | '"'*)
		refuse "a value opens with a quote it does not close, which the agent's parser refuses"
		;;
	esac
done

install_file "$binary" 0755 "$destdir$binary_file"
write_env_file "$destdir$env_file"
install_file "$service_source" 0644 "$destdir$service_file"
if [ -n "$log_file" ]; then
	mkdir -p "$(dirname "$destdir$log_file")"
	# An existing log is kept; a symlink standing where it should be is not a log.
	if [ ! -e "$destdir$log_file" ] || [ -L "$destdir$log_file" ]; then
		create_file "$destdir$log_file" 077
	fi
	chmod 0600 "$destdir$log_file"
	written="$written  $destdir$log_file
"
fi

if [ -z "$destdir" ]; then
	case $init_tool in
	systemctl)
		systemctl daemon-reload
		systemctl enable monitor-agent.service
		systemctl restart monitor-agent.service
		;;
	launchctl)
		launchctl bootout "system/$service_label" 2>/dev/null || :
		# `launchctl disable` writes an override that outlives bootout and a reboot, so a
		# label disabled once would bootstrap and never run. Clearing it is best effort:
		# there is nothing to clear on a label that was never disabled.
		launchctl enable "system/$service_label" 2>/dev/null || :
		# bootout returns before the daemon has finished going away, and bootstrap fails
		# while it is still there. Retrying is what makes an upgrade reliable.
		attempt=1
		while ! launchctl bootstrap system "$service_file" 2>/dev/null; do
			[ "$attempt" -lt 20 ] ||
				refuse "launchctl bootstrap failed and the agent is not running"
			attempt=$((attempt + 1))
			sleep 0.5
		done
		;;
	esac
fi

printf '%s: installed\n%s' "$program" "$written"
if [ -n "$destdir" ]; then
	printf 'Staged under %s: no service was registered.\n' "$destdir"
fi
printf 'The service state: %s\n' "$status_command"
