// This file drives the real install-agent.sh. Every run is staged under DESTDIR, so the
// machine running the test is never touched, and the layout asserted is the host's own —
// CI runs the suite on Debian and on macOS, which is what exercises both branches.
package deploy_test

import (
	"bytes"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/pravbeseda/monitor/internal/agent"
)

const script = "./install-agent.sh"

// Synthetic throughout (ADR 0007): no run here names a real host, node or token.
const (
	testHub   = "https://hub.example.com"
	testNode  = "laptop-a"
	testToken = "token-aaaa"
)

// installedFile is one file the script writes and the mode the layout table gives it.
type installedFile struct {
	path string // relative to DESTDIR
	mode os.FileMode
}

// nodeLayout is the spec's layout table for one operating system.
type nodeLayout struct {
	binary  installedFile
	env     installedFile
	service installedFile
	log     *installedFile // launchd only: nil on Debian
	status  string         // the command a successful run points at
}

func (l nodeLayout) files() []installedFile {
	files := []installedFile{l.binary, l.env, l.service}
	if l.log != nil {
		files = append(files, *l.log)
	}
	return files
}

// spec: deployment.md#where-things-live — the paths and modes, read for the system the test
// is running on.
func hostLayout() nodeLayout {
	if runtime.GOOS == "darwin" {
		log := installedFile{"var/log/monitor-agent.log", 0o600}
		return nodeLayout{
			binary:  installedFile{"usr/local/bin/monitor-agent", 0o755},
			env:     installedFile{"usr/local/etc/monitor/agent.env", 0o600},
			service: installedFile{"Library/LaunchDaemons/io.github.pravbeseda.monitor-agent.plist", 0o644},
			log:     &log,
			status:  "launchctl print system/io.github.pravbeseda.monitor-agent",
		}
	}
	return nodeLayout{
		binary:  installedFile{"usr/local/bin/monitor-agent", 0o755},
		env:     installedFile{"etc/monitor/agent.env", 0o600},
		service: installedFile{"etc/systemd/system/monitor-agent.service", 0o644},
		status:  "systemctl status monitor-agent.service",
	}
}

// run is one invocation of the script, staged under destDir.
type run struct {
	destDir string
	args    []string
	stdin   string // what the token arrives on when it comes that way
	token   string // MONITOR_TOKEN in the environment; empty leaves it unset
	pathDir string // prepended to PATH, to catch a service command being run
	onlyDir bool   // PATH is pathDir alone, so what is missing from it is really missing
	umask   string // the caller's umask, when the run has to survive a hostile one
}

func (r run) start(t *testing.T) (stdout, stderr string, err error) {
	t.Helper()
	path := os.Getenv("PATH")
	switch {
	case r.onlyDir:
		path = r.pathDir
	case r.pathDir != "":
		path = r.pathDir + string(os.PathListSeparator) + path
	}
	cmd := exec.Command(script, r.args...)
	if r.umask != "" {
		// exec keeps $0 as the script, so the arguments below still land on it.
		shell := []string{"-c", "umask " + r.umask + `; exec "$0" "$@"`, script}
		cmd = exec.Command("/bin/sh", append(shell, r.args...)...)
	}
	cmd.Env = []string{"PATH=" + path, "DESTDIR=" + r.destDir}
	if r.token != "" {
		cmd.Env = append(cmd.Env, "MONITOR_TOKEN="+r.token)
	}
	cmd.Stdin = strings.NewReader(r.stdin)
	var out, errs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errs
	err = cmd.Run()
	return out.String(), errs.String(), err
}

// mustRun fails the test unless the run succeeds.
func (r run) mustRun(t *testing.T) (stdout, stderr string) {
	t.Helper()
	stdout, stderr, err := r.start(t)
	if err != nil {
		t.Fatalf("install-agent.sh %v: %v\nstdout: %s\nstderr: %s", r.args, err, stdout, stderr)
	}
	return stdout, stderr
}

// agentBinary is what the script copies: a small executable file, not a built agent. Its
// mode is deliberately not the 0755 the layout table asks for, so that a run which merely
// copied the source's mode across would fail the layout assertion.
func agentBinary(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "monitor-agent")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("write test binary: %v", err)
	}
	return path
}

// install is the ordinary invocation: the three flags, the token on stdin.
func install(t *testing.T, destDir, binary, hub, node, token string) (stdout, stderr string) {
	t.Helper()
	return run{
		destDir: destDir,
		args:    []string{"--binary", binary, "--hub", hub, "--node", node},
		stdin:   token,
	}.mustRun(t)
}

// tree is every file below dir, keyed by its path relative to dir.
func tree(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	found := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		found[rel] = body
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return found
}

// assertLayout checks that the run produced exactly the layout table's files, each with the
// mode the table gives it.
func assertLayout(t *testing.T, destDir string) {
	t.Helper()
	want := hostLayout().files()
	got := tree(t, destDir)
	if len(got) != len(want) {
		t.Errorf("installed %d files, want %d: %v", len(got), len(want), sortedKeys(got))
	}
	for _, file := range want {
		if _, ok := got[file.path]; !ok {
			t.Errorf("%s was not installed; the tree holds %v", file.path, sortedKeys(got))
			continue
		}
		info, err := os.Stat(filepath.Join(destDir, file.path))
		if err != nil {
			t.Fatalf("stat %s: %v", file.path, err)
		}
		if info.Mode().Perm() != file.mode {
			t.Errorf("%s has mode %04o, want %04o", file.path, info.Mode().Perm(), file.mode)
		}
	}
}

func sortedKeys(files map[string][]byte) []string {
	return slices.Sorted(maps.Keys(files))
}

// installedEnv is the environment file the run wrote, through the parser the agent uses.
func installedEnv(t *testing.T, destDir string) map[string]string {
	t.Helper()
	values, err := agent.ReadEnvFile(filepath.Join(destDir, hostLayout().env.path))
	if err != nil {
		t.Fatalf("parse the installed environment file: %v", err)
	}
	return values
}

func assertEnv(t *testing.T, destDir, hub, node, token string) {
	t.Helper()
	values := installedEnv(t, destDir)
	for key, want := range map[string]string{
		"MONITOR_HUB":   hub,
		"MONITOR_NODE":  node,
		"MONITOR_TOKEN": token,
	} {
		if values[key] != want {
			t.Errorf("the environment file has %s=%q, want %q", key, values[key], want)
		}
	}
}

// spec: deployment.md#installing-on-a-fresh-node — the binary lands executable, the
// environment file holds the three values and is readable by its owner only, in the layout
// the host's own system fixes. The token arrives on stdin, and on MONITOR_TOKEN.
func TestAFreshInstallWritesTheLayoutTheSpecFixes(t *testing.T) {
	tests := []struct {
		name  string
		stdin string
		token string
	}{
		{"token on stdin", testToken, ""},
		{"token in the environment", "", testToken},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			destDir, binary := t.TempDir(), agentBinary(t)
			stdout, stderr := run{
				destDir: destDir,
				args:    []string{"--binary", binary, "--hub", testHub, "--node", testNode},
				stdin:   tc.stdin,
				token:   tc.token,
			}.mustRun(t)

			assertLayout(t, destDir)
			assertEnv(t, destDir, testHub, testNode, testToken)

			shipped, err := os.ReadFile(binary)
			if err != nil {
				t.Fatalf("read the source binary: %v", err)
			}
			if installed := tree(t, destDir)[hostLayout().binary.path]; !bytes.Equal(installed, shipped) {
				t.Errorf("the installed binary differs from the one --binary named")
			}
			assertNoToken(t, stdout, stderr)
		})
	}
}

// spec: deployment.md#installing-on-a-fresh-node — the token appears in no message. It is
// not an argument either, which is why the script offers no flag for it.
func assertNoToken(t *testing.T, streams ...string) {
	t.Helper()
	for _, stream := range streams {
		if strings.Contains(stream, testToken) {
			t.Errorf("a stream carries the token: %s", stream)
		}
	}
}

// spec: deployment.md#installing-on-a-fresh-node — any successful run prints every path it
// wrote and the command that shows the service's state.
func TestASuccessfulRunPrintsWhatItWroteAndHowToSeeTheService(t *testing.T) {
	destDir := t.TempDir()
	stdout, _ := install(t, destDir, agentBinary(t), testHub, testNode, testToken)

	for _, file := range hostLayout().files() {
		if !strings.Contains(stdout, filepath.Join(destDir, file.path)) {
			t.Errorf("the run does not print %s; it printed:\n%s", file.path, stdout)
		}
	}
	if !strings.Contains(stdout, hostLayout().status) {
		t.Errorf("the run does not print %q; it printed:\n%s", hostLayout().status, stdout)
	}
	// spec: deployment.md#staged-installs — and it says which of the two runs this was, so
	// that a stray DESTDIR cannot pass for an installed node.
	if !strings.Contains(stdout, "no service was registered") {
		t.Errorf("a staged run does not say so; it printed:\n%s", stdout)
	}
}

// spec: deployment.md#re-running — a run with the same arguments and the same binary leaves
// every installed file byte-identical. Compared by content, because a copy is what a
// timestamp would show changing.
func TestASecondRunWithTheSameInputsChangesNothing(t *testing.T) {
	destDir, binary := t.TempDir(), agentBinary(t)
	install(t, destDir, binary, testHub, testNode, testToken)
	before := tree(t, destDir)

	install(t, destDir, binary, testHub, testNode, testToken)

	after := tree(t, destDir)
	if len(before) != len(after) {
		t.Fatalf("the second run left %v, want %v", sortedKeys(after), sortedKeys(before))
	}
	for name, body := range before {
		if !bytes.Equal(after[name], body) {
			t.Errorf("%s changed on a second run with the same inputs", name)
		}
	}
	assertLayout(t, destDir)
}

// spec: deployment.md#re-running — a run with a newer binary replaces the installed one.
// The restart that follows is a service command, so a staged run cannot show it
// (deployment.md#staged-installs).
func TestARerunReplacesTheBinary(t *testing.T) {
	destDir := t.TempDir()
	install(t, destDir, agentBinary(t), testHub, testNode, testToken)

	newer := filepath.Join(t.TempDir(), "monitor-agent")
	if err := os.WriteFile(newer, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write the newer binary: %v", err)
	}
	install(t, destDir, newer, testHub, testNode, testToken)

	want, err := os.ReadFile(newer)
	if err != nil {
		t.Fatalf("read the newer binary: %v", err)
	}
	if got := tree(t, destDir)[hostLayout().binary.path]; !bytes.Equal(got, want) {
		t.Errorf("the re-run did not replace the installed binary")
	}
	assertLayout(t, destDir)
}

// spec: deployment.md#re-running — a run with a different --hub or --node carries the new
// value into the environment file.
func TestARerunWritesADifferentHubOrNodeThrough(t *testing.T) {
	destDir, binary := t.TempDir(), agentBinary(t)
	install(t, destDir, binary, testHub, testNode, testToken)

	install(t, destDir, binary, "https://other.example.com", "server-b", testToken)

	assertEnv(t, destDir, "https://other.example.com", "server-b", testToken)
	assertLayout(t, destDir)
}

// spec: deployment.md#re-running — a run whose token differs replaces the stored one, and
// the file stays readable by its owner only.
func TestARerunReplacesTheStoredToken(t *testing.T) {
	destDir, binary := t.TempDir(), agentBinary(t)
	install(t, destDir, binary, testHub, testNode, testToken)

	install(t, destDir, binary, testHub, testNode, "token-bbbb")

	assertEnv(t, destDir, testHub, testNode, "token-bbbb")
	env := string(tree(t, destDir)[hostLayout().env.path])
	if strings.Contains(env, testToken) {
		t.Errorf("the environment file still carries the replaced token")
	}
	assertLayout(t, destDir)
}

// spec: deployment.md#re-running — a run with no token available keeps the stored one and
// succeeds. Writing an empty token over a working one would break a node during a routine
// upgrade (deployment.md#edge-cases).
func TestARerunWithoutATokenKeepsTheStoredOne(t *testing.T) {
	destDir, binary := t.TempDir(), agentBinary(t)
	install(t, destDir, binary, testHub, testNode, testToken)

	install(t, destDir, binary, testHub, "server-b", "")

	assertEnv(t, destDir, testHub, "server-b", testToken)
	assertLayout(t, destDir)
}

// spec: deployment.md#edge-cases — the environment file edited by hand is supported: the run
// rewrites MONITOR_HUB and MONITOR_NODE and leaves every other line alone.
func TestARerunLeavesLinesItDoesNotOwnAlone(t *testing.T) {
	destDir, binary := t.TempDir(), agentBinary(t)
	install(t, destDir, binary, testHub, testNode, testToken)

	envFile := filepath.Join(destDir, hostLayout().env.path)
	body, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("read the environment file: %v", err)
	}
	if err := os.WriteFile(envFile, append([]byte("# hand-written note\n"), body...), 0o600); err != nil {
		t.Fatalf("edit the environment file: %v", err)
	}

	install(t, destDir, binary, "https://other.example.com", testNode, "")

	edited := string(tree(t, destDir)[hostLayout().env.path])
	if !strings.Contains(edited, "# hand-written note") {
		t.Errorf("the run dropped a line it does not own:\n%s", edited)
	}
	assertEnv(t, destDir, "https://other.example.com", testNode, testToken)
}

// spec: deployment.md#edge-cases — a hand-edited file is supported, and a hand-indented line
// is still that key: the agent's own parser ignores leading blanks (ADR 0020), so a token
// behind one is a stored token rather than a node that refuses its next upgrade.
func TestARerunReadsAnIndentedLineAsThatKey(t *testing.T) {
	destDir, binary := t.TempDir(), agentBinary(t)
	install(t, destDir, binary, testHub, testNode, testToken)

	envFile := filepath.Join(destDir, hostLayout().env.path)
	body, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("read the environment file: %v", err)
	}
	indented := regexp.MustCompile(`(?m)^`).ReplaceAllString(string(body), "  ")
	if err := os.WriteFile(envFile, []byte(indented), 0o600); err != nil {
		t.Fatalf("edit the environment file: %v", err)
	}

	install(t, destDir, binary, "https://other.example.com", testNode, "")

	assertEnv(t, destDir, "https://other.example.com", testNode, testToken)
}

// spec: deployment.md#refusing — every refusal is decided before the first file is written,
// so a rejected run leaves the node exactly as it was.
//
// The two refusals DESTDIR suppresses — no init system, not root — have no staged run to
// assert them from (deployment.md#staged-installs), so they are left to the real install.
func TestARefusalNamesItsCauseAndWritesNothing(t *testing.T) {
	binary := agentBinary(t)
	unreadable := filepath.Join(t.TempDir(), "not-executable")
	if err := os.WriteFile(unreadable, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("write the non-executable file: %v", err)
	}
	missing := filepath.Join(t.TempDir(), "absent")

	tests := []struct {
		name  string
		args  []string
		stdin string
		names string
	}{
		{
			name:  "the binary is missing",
			args:  []string{"--binary", missing, "--hub", testHub, "--node", testNode},
			stdin: testToken,
			names: missing,
		},
		{
			name:  "the binary is not executable",
			args:  []string{"--binary", unreadable, "--hub", testHub, "--node", testNode},
			stdin: testToken,
			names: unreadable,
		},
		{
			name:  "no --binary",
			args:  []string{"--hub", testHub, "--node", testNode},
			stdin: testToken,
			names: "--binary",
		},
		{
			name:  "no --hub",
			args:  []string{"--binary", binary, "--node", testNode},
			stdin: testToken,
			names: "--hub",
		},
		{
			name:  "no --node",
			args:  []string{"--binary", binary, "--hub", testHub},
			stdin: testToken,
			names: "--node",
		},
		{
			name:  "--hub without a value",
			args:  []string{"--binary", binary, "--hub"},
			stdin: testToken,
			names: "--hub",
		},
		{
			name:  "no token anywhere",
			args:  []string{"--binary", binary, "--hub", testHub, "--node", testNode},
			stdin: "",
			names: "MONITOR_TOKEN",
		},
		{
			// Not a row of the table: the environment file is one KEY=VALUE per line, so a
			// value carrying a newline would write a second line the agent reads as
			// configuration — a mistyped --node could override the stored token.
			name:  "a value carrying a newline",
			args:  []string{"--binary", binary, "--hub", testHub, "--node", "server-b\nMONITOR_TOKEN=x"},
			stdin: testToken,
			names: "newline",
		},
		{
			name:  "an unknown flag",
			args:  []string{"--binary", binary, "--hub", testHub, "--node", testNode, "--token", testToken},
			stdin: "",
			names: "usage",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			destDir := t.TempDir()
			stdout, stderr, err := run{destDir: destDir, args: tc.args, stdin: tc.stdin}.start(t)
			if err == nil {
				t.Fatalf("the run succeeded; it should have refused")
			}
			if !strings.Contains(stdout+stderr, tc.names) {
				t.Errorf("the refusal does not name %q; it said:\n%s%s", tc.names, stdout, stderr)
			}
			if written := tree(t, destDir); len(written) != 0 {
				t.Errorf("the refused run wrote %v", sortedKeys(written))
			}
			assertNoToken(t, stdout, stderr)
		})
	}
}

// spec: deployment.md#staged-installs — no service is registered, started or restarted. The
// stand-ins on PATH record any call, so a run that reached for one is visible.
func TestAStagedRunCallsNoServiceCommand(t *testing.T) {
	pathDir := t.TempDir()
	marker := filepath.Join(pathDir, "called")
	for _, tool := range []string{"systemctl", "launchctl"} {
		stub := "#!/bin/sh\necho \"$0 $*\" >> " + marker + "\n"
		if err := os.WriteFile(filepath.Join(pathDir, tool), []byte(stub), 0o755); err != nil {
			t.Fatalf("write the %s stand-in: %v", tool, err)
		}
	}

	// Without this, a broken PATH would make the assertion below pass for the wrong reason.
	control := exec.Command("/bin/sh", "-c", "systemctl daemon-reload")
	control.Env = []string{"PATH=" + pathDir + string(os.PathListSeparator) + os.Getenv("PATH")}
	if err := control.Run(); err != nil {
		t.Fatalf("the stand-in is not reachable on PATH: %v", err)
	}
	if _, err := os.ReadFile(marker); err != nil {
		t.Fatalf("the stand-in records no call, so this test could not detect one: %v", err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatalf("clear the record: %v", err)
	}

	destDir := t.TempDir()
	run{
		destDir: destDir,
		args:    []string{"--binary", agentBinary(t), "--hub", testHub, "--node", testNode},
		stdin:   testToken,
		pathDir: pathDir,
	}.mustRun(t)

	if called, err := os.ReadFile(marker); err == nil {
		t.Errorf("a staged run called a service command: %s", called)
	}
	assertLayout(t, destDir)
}

// spec: deployment.md#staged-installs — neither root nor an init system is required. The run
// gets a PATH holding the ordinary tools and nothing else, so `systemctl` and `launchctl`
// are genuinely absent rather than merely unused.
func TestAStagedRunNeedsNoInitSystemOnPath(t *testing.T) {
	pathDir := t.TempDir()
	for _, tool := range []string{"basename", "dirname", "mkdir", "rm", "cp", "chmod", "mv", "uname", "id"} {
		found, err := exec.LookPath(tool)
		if err != nil {
			t.Fatalf("find %s: %v", tool, err)
		}
		if err := os.Symlink(found, filepath.Join(pathDir, tool)); err != nil {
			t.Fatalf("link %s: %v", tool, err)
		}
	}
	for _, tool := range []string{"systemctl", "launchctl"} {
		if _, err := exec.LookPath(tool); err == nil {
			t.Logf("%s exists on this host, and is deliberately left off the run's PATH", tool)
		}
	}

	destDir := t.TempDir()
	run{
		destDir: destDir,
		args:    []string{"--binary", agentBinary(t), "--hub", testHub, "--node", testNode},
		stdin:   testToken,
		pathDir: pathDir,
		onlyDir: true,
	}.mustRun(t)

	assertLayout(t, destDir)
	assertEnv(t, destDir, testHub, testNode, testToken)
}

// spec: deployment.md#where-things-live — the modes are the table's whatever umask the
// operator ran with. Under a permissive umask a run that only copied modes across would
// leave the environment file readable by every account on the node.
func TestTheModesDoNotFollowTheCallersUmask(t *testing.T) {
	destDir := t.TempDir()
	run{
		destDir: destDir,
		args:    []string{"--binary", agentBinary(t), "--hub", testHub, "--node", testNode},
		stdin:   testToken,
		umask:   "000",
	}.mustRun(t)

	assertLayout(t, destDir)
}

// spec: deployment.md#invariants — a run that fails after it has begun writing stops at that
// failure and names what it already wrote. Here the environment file's directory is a
// regular file, so the run dies with the binary already installed.
func TestARunThatFailsAfterWritingNamesWhatItWrote(t *testing.T) {
	destDir := t.TempDir()
	envDir := filepath.Dir(filepath.Join(destDir, hostLayout().env.path))
	if err := os.MkdirAll(filepath.Dir(envDir), 0o755); err != nil {
		t.Fatalf("prepare the staging tree: %v", err)
	}
	if err := os.WriteFile(envDir, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatalf("block the environment file's directory: %v", err)
	}

	stdout, stderr, err := run{
		destDir: destDir,
		args:    []string{"--binary", agentBinary(t), "--hub", testHub, "--node", testNode},
		stdin:   testToken,
	}.start(t)
	if err == nil {
		t.Fatalf("the run succeeded; it should have failed on the environment file")
	}

	binary := filepath.Join(destDir, hostLayout().binary.path)
	if !strings.Contains(stdout+stderr, binary) {
		t.Errorf("the failure does not name the binary it had already written:\n%s%s", stdout, stderr)
	}
	if _, err := os.Stat(binary); err != nil {
		t.Errorf("the run rolled back instead of stopping: %v", err)
	}
	assertNoToken(t, stdout, stderr)
}

// spec: deployment.md#where-things-live — the service definition installed is the one this
// repository ships, byte for byte: it is a constant, so nothing is rendered into it.
func TestTheInstalledServiceIsTheShippedFile(t *testing.T) {
	destDir := t.TempDir()
	install(t, destDir, agentBinary(t), testHub, testNode, testToken)

	source := agentUnit
	if runtime.GOOS == "darwin" {
		source = agentPlist
	}
	shipped, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read %s: %v", source, err)
	}
	if installed := tree(t, destDir)[hostLayout().service.path]; !bytes.Equal(installed, shipped) {
		t.Errorf("the installed service definition differs from %s", source)
	}
}

// spec: deployment.md#where-things-live — the agent log exists in the launchd layout only,
// and it is created by the install because launchd would otherwise create it world-readable
// on first start.
func TestTheAgentLogBelongsToTheLaunchdLayoutOnly(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the log file is a macOS row of the layout table")
	}
	destDir, binary := t.TempDir(), agentBinary(t)
	install(t, destDir, binary, testHub, testNode, testToken)

	logFile := filepath.Join(destDir, "var/log/monitor-agent.log")
	if err := os.WriteFile(logFile, []byte("a line the agent already wrote\n"), 0o600); err != nil {
		t.Fatalf("write the log: %v", err)
	}

	install(t, destDir, binary, testHub, testNode, testToken)

	kept, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read the log: %v", err)
	}
	if len(kept) == 0 {
		t.Errorf("the second run truncated the agent's log")
	}
}
