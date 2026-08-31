// Package deploy_test reads the shipped service definitions as data. They are constants —
// everything that varies between installations is in an environment file — so what can be
// tested is what they name.
package deploy_test

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	agentUnit  = "systemd/monitor-agent.service"
	hubUnit    = "systemd/monitor-hub.service"
	agentPlist = "launchd/io.github.pravbeseda.monitor-agent.plist"
	agentEnv   = "agent.env.example"
	hubEnv     = "hub.env.example"
)

// exampleHub is the only URL any file here may carry.
const exampleHub = "https://hub.example.com"

// plistDoctype is Apple's fixed boilerplate. It is removed before the scan for hosts and
// addresses, so that scan judges only the lines we wrote.
const plistDoctype = `<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">`

// shipped is every file this directory installs on a node.
var shipped = []string{agentUnit, hubUnit, agentPlist, agentEnv, hubEnv}

// layout is the spec's table read from the other side: the paths each unit has to name.
var layout = []struct {
	name  string
	file  string
	paths []string
	// extra are paths the unit needs that the layout table does not fix.
	extra []string
}{
	{
		name:  "agent unit",
		file:  agentUnit,
		paths: []string{"/usr/local/bin/monitor-agent", "/etc/monitor/agent.env"},
	},
	{
		name: "hub unit",
		file: hubUnit,
		paths: []string{
			"/usr/local/bin/monitor-hub",
			"/etc/monitor/hub.env",
			"/etc/monitor/hub.yaml",
			"/var/lib/monitor/monitor.db",
		},
	},
	{
		name: "agent plist",
		file: agentPlist,
		paths: []string{
			"/usr/local/bin/monitor-agent",
			"/usr/local/etc/monitor/agent.env",
			"/var/log/monitor-agent.log",
		},
		extra: []string{"/bin/sh"}, // launchd has no EnvironmentFile; a shell sources it.
	},
}

// allowedNames is every dotted name a shipped file may contain. Anything else is a host, a
// domain or a secret that someone pasted in.
var allowedNames = map[string]bool{
	"agent.env":                          true,
	"hub.env":                            true,
	"hub.yaml":                           true,
	"monitor.db":                         true,
	"monitor-agent.log":                  true,
	"network-online.target":              true,
	"multi-user.target":                  true,
	"io.github.pravbeseda.monitor-agent": true,
	"hub.example.com":                    true,
	"deployment.md":                      true,
	"config.example.yaml":                true,
}

var (
	// absolutePath captures the character in front of the path as well, so that a relative
	// path in a comment (docs/specs/deployment.md) is not read as an absolute one.
	absolutePath = regexp.MustCompile(`(^|[^A-Za-z0-9._/-])(/[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)+)`)
	// The value stops at the closing XML tag as well, so the plist's flags read the same
	// way the unit's do.
	flagValue  = regexp.MustCompile(`--(hub|node)[ \t]+([^\s<]+)`)
	assignment = regexp.MustCompile(`(?m)^([A-Z][A-Z0-9_]*)=(.*)$`)
	urlLiteral = regexp.MustCompile(`[a-z][a-z0-9+.-]*://[^\s"'<]+`)
	ipAddress  = regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}\b`)
	dottedName = regexp.MustCompile(`[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)+`)
	letter     = regexp.MustCompile(`[A-Za-z]`)
)

func read(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(body)
}

// spec: deployment.md#where-things-live — every path is fixed there, which is what lets a
// unit be a constant.
func TestUnitsNameThePathsTheSpecFixes(t *testing.T) {
	for _, unit := range layout {
		t.Run(unit.name, func(t *testing.T) {
			body := read(t, unit.file)
			for _, want := range unit.paths {
				if !strings.Contains(body, want) {
					t.Errorf("%s does not name %s", unit.file, want)
				}
			}
		})
	}
}

// spec: deployment.md#where-things-live — a path outside the table is one the install script
// and the guide do not know about.
func TestUnitsNameNoPathTheSpecDoesNotFix(t *testing.T) {
	for _, unit := range layout {
		t.Run(unit.name, func(t *testing.T) {
			allowed := map[string]bool{}
			for _, path := range append(append([]string{}, unit.paths...), unit.extra...) {
				allowed[path] = true
				allowed[filepath.Dir(path)] = true // the directory a fixed path lives in
			}
			for _, match := range absolutePath.FindAllStringSubmatch(read(t, unit.file), -1) {
				if !allowed[match[2]] {
					t.Errorf("%s names %s, which the layout table does not fix", unit.file, match[2])
				}
			}
		})
	}
}

// spec: deployment.md#the-environment-files — the hub URL and the node name are deployment
// settings: the service expands them from the environment file and never spells them out.
func TestAgentTakesHubAndNodeFromTheEnvironmentFile(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		envFile string
		hub     string
		node    string
	}{
		{"agent unit", agentUnit, "/etc/monitor/agent.env", "${MONITOR_HUB}", "${MONITOR_NODE}"},
		{"agent plist", agentPlist, "/usr/local/etc/monitor/agent.env", `"$MONITOR_HUB"`, `"$MONITOR_NODE"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := read(t, tc.file)
			if !strings.Contains(body, tc.envFile) {
				t.Fatalf("%s does not read %s", tc.file, tc.envFile)
			}
			want := map[string]string{"hub": tc.hub, "node": tc.node}
			seen := map[string]bool{}
			for _, match := range flagValue.FindAllStringSubmatch(body, -1) {
				seen[match[1]] = true
				if match[2] != want[match[1]] {
					t.Errorf("%s passes --%s %s, want %s from the environment file",
						tc.file, match[1], match[2], want[match[1]])
				}
			}
			for flag := range want {
				if !seen[flag] {
					t.Errorf("%s passes no --%s", tc.file, flag)
				}
			}
		})
	}
}

// spec: deployment.md#the-environment-files — the token reaches the process through the
// environment file, so a service definition that named it could also hold it.
func TestNoServiceDefinitionNamesTheToken(t *testing.T) {
	for _, file := range []string{agentUnit, hubUnit, agentPlist} {
		t.Run(file, func(t *testing.T) {
			if strings.Contains(read(t, file), "TOKEN") {
				t.Errorf("%s names the token; it belongs in the environment file only", file)
			}
		})
	}
}

// spec: deployment.md#invariants — no file this repository ships names a host, a node, a URL
// or a token (ADR 0007).
func TestNoShippedFileCarriesAHostAddressOrSecret(t *testing.T) {
	for _, file := range shipped {
		t.Run(file, func(t *testing.T) {
			body := strings.ReplaceAll(read(t, file), plistDoctype, "")
			if found := ipAddress.FindString(body); found != "" {
				t.Errorf("%s carries the address %s", file, found)
			}
			for _, found := range urlLiteral.FindAllString(body, -1) {
				if found != exampleHub {
					t.Errorf("%s carries the URL %s, want %s or none", file, found, exampleHub)
				}
			}
			for _, found := range dottedName.FindAllString(body, -1) {
				if !letter.MatchString(found) || allowedNames[found] {
					continue
				}
				t.Errorf("%s names %s, which reads like a host or a secret", file, found)
			}
		})
	}
}

// spec: deployment.md#the-environment-files — the examples show the keys and none of the
// values, because every value is a secret or a deployment setting (ADR 0007).
func TestExampleEnvironmentFilesHoldPlaceholdersOnly(t *testing.T) {
	placeholders := map[string]bool{exampleHub: true, "laptop-a": true, "replace-me": true}
	tests := []struct {
		file string
		keys []string
	}{
		{agentEnv, []string{"MONITOR_HUB", "MONITOR_NODE", "MONITOR_TOKEN"}},
		{hubEnv, []string{"MONITOR_TELEGRAM_TOKEN", "MONITOR_TELEGRAM_CHAT_ID"}},
	}

	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			body := read(t, tc.file)
			set := map[string]bool{}
			for _, match := range assignment.FindAllStringSubmatch(body, -1) {
				set[match[1]] = true
				if !placeholders[match[2]] {
					t.Errorf("%s sets %s to %q, want a placeholder", tc.file, match[1], match[2])
				}
			}
			for _, key := range tc.keys {
				if !set[key] {
					t.Errorf("%s assigns no %s", tc.file, key)
				}
			}
		})
	}
}

// spec: deployment.md#where-things-live — the hub runs as the unprivileged monitor account;
// the agent runs as root, because it stats every mounted volume.
func TestOnlyTheHubRunsAsAnAccount(t *testing.T) {
	hub := read(t, hubUnit)
	for _, want := range []string{"User=monitor", "Group=monitor"} {
		if !strings.Contains(hub, want) {
			t.Errorf("%s has no %s", hubUnit, want)
		}
	}
	agent := read(t, agentUnit)
	for _, unwanted := range []string{"User=", "Group="} {
		if strings.Contains(agent, unwanted) {
			t.Errorf("%s sets %s; the agent runs as root", agentUnit, unwanted)
		}
	}
	if strings.Contains(read(t, agentPlist), "UserName") {
		t.Errorf("%s sets UserName; a launchd daemon runs as root", agentPlist)
	}
}

// spec: deployment.md#the-services — it is restarted after a short delay, and goes on being
// restarted however fast it keeps failing.
func TestBothUnitsRestartWithoutARateLimit(t *testing.T) {
	for _, file := range []string{agentUnit, hubUnit} {
		t.Run(file, func(t *testing.T) {
			body := read(t, file)
			for _, want := range []string{"Restart=always", "RestartSec=", "StartLimitIntervalSec=0"} {
				if !strings.Contains(body, want) {
					t.Errorf("%s has no %s", file, want)
				}
			}
		})
	}
}

// spec: deployment.md#the-services — launchd's half of the same guarantee: start at load,
// restart on exit.
func TestThePlistStartsAtLoadAndKeepsTheAgentAlive(t *testing.T) {
	body := read(t, agentPlist)
	for _, key := range []string{"RunAtLoad", "KeepAlive"} {
		enabled := regexp.MustCompile(`<key>` + key + `</key>\s*<true/>`)
		if !enabled.MatchString(body) {
			t.Errorf("%s does not set %s", agentPlist, key)
		}
	}
}

// spec: deployment.md#the-services — launchd supervises what ProgramArguments starts, so the
// shell that sources the environment file has to hand the process over rather than stay in
// the middle of it.
func TestThePlistExecsTheAgent(t *testing.T) {
	var doc struct {
		Args []string `xml:"dict>array>string"`
	}
	if err := xml.Unmarshal([]byte(read(t, agentPlist)), &doc); err != nil {
		t.Fatalf("parse %s: %v", agentPlist, err)
	}
	if len(doc.Args) != 3 {
		t.Fatalf("ProgramArguments = %q, want the shell, -c and one command", doc.Args)
	}
	if doc.Args[0] != "/bin/sh" || doc.Args[1] != "-c" {
		t.Fatalf("ProgramArguments starts with %q, want /bin/sh -c", doc.Args[:2])
	}
	steps := strings.Split(doc.Args[2], "&&")
	last := strings.TrimSpace(steps[len(steps)-1])
	if !strings.HasPrefix(last, "exec /usr/local/bin/monitor-agent") {
		t.Errorf("the command ends with %q, want it to exec the agent", last)
	}
}
