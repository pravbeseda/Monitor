package agent_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pravbeseda/monitor/internal/agent"
)

// envFile writes a file in a temporary directory and returns its path.
func envFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent.env")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// spec: agent.md#local-configuration — the file is KEY=VALUE data: blank lines and comment
// lines are skipped, matching surrounding quotes are stripped, and a key the agent does not
// know is carried rather than refused.
func TestReadEnvFileParsesKeyValueLines(t *testing.T) {
	path := envFile(t, `# the hub this node reports to

MONITOR_HUB=https://hub.example.com
MONITOR_NODE="laptop-a"
MONITOR_TOKEN='replace-me'
   MONITOR_SPACED   =   padded
MONITOR_EMPTY=
MONITOR_UNKNOWN=ignored by the agent, kept by the parser
MONITOR_DUPLICATE=first
MONITOR_DUPLICATE=last
`)

	values, err := agent.ReadEnvFile(path)

	if err != nil {
		t.Fatalf("ReadEnvFile: %v", err)
	}
	want := map[string]string{
		"MONITOR_HUB":       "https://hub.example.com",
		"MONITOR_NODE":      "laptop-a",
		"MONITOR_TOKEN":     "replace-me",
		"MONITOR_SPACED":    "padded",
		"MONITOR_EMPTY":     "",
		"MONITOR_UNKNOWN":   "ignored by the agent, kept by the parser",
		"MONITOR_DUPLICATE": "last",
	}
	if len(values) != len(want) {
		t.Fatalf("ReadEnvFile = %v, want %v", values, want)
	}
	for key, wanted := range want {
		if values[key] != wanted {
			t.Errorf("%s = %q, want %q", key, values[key], wanted)
		}
	}
}

// spec: agent.md#local-configuration — nothing in the file is expanded, substituted or run.
// It is inert data, which is the whole reason the agent reads it instead of a shell: a `.`
// in a shell would execute the tail of a token pasted with a space in it.
func TestReadEnvFileTakesHostileValuesLiterally(t *testing.T) {
	hostile := []struct{ key, value string }{
		{"MONITOR_SUBSTITUTION", "$(id)"},
		{"MONITOR_BACKTICKS", "`id`"},
		{"MONITOR_SPACE", "abcd efgh"},
		{"MONITOR_HASH", "abcd#efgh"},
		{"MONITOR_VARIABLE", "$MONITOR_HUB"},
		{"MONITOR_SEQUENCE", "abcd; touch /tmp/pwned"},
	}
	var body strings.Builder
	for _, line := range hostile {
		body.WriteString(line.key + "=" + line.value + "\n")
	}
	path := envFile(t, body.String())

	values, err := agent.ReadEnvFile(path)

	if err != nil {
		t.Fatalf("ReadEnvFile: %v", err)
	}
	for _, line := range hostile {
		if values[line.key] != line.value {
			t.Errorf("%s = %q, want the literal %q", line.key, values[line.key], line.value)
		}
	}
}

// spec: agent.md#local-configuration — a line ends where any system ends one, so a file
// written on another platform does not smuggle a carriage return into a value.
func TestReadEnvFileEndsALineAtAnyLineEnding(t *testing.T) {
	for name, body := range map[string]string{
		"lf":   "MONITOR_NODE=laptop-a\nMONITOR_TOKEN=replace-me\n",
		"crlf": "MONITOR_NODE=laptop-a\r\nMONITOR_TOKEN=replace-me\r\n",
		"cr":   "MONITOR_NODE=laptop-a\rMONITOR_TOKEN=replace-me\r",
	} {
		t.Run(name, func(t *testing.T) {
			values, err := agent.ReadEnvFile(envFile(t, body))

			if err != nil {
				t.Fatalf("ReadEnvFile: %v", err)
			}
			if values["MONITOR_NODE"] != "laptop-a" || values["MONITOR_TOKEN"] != "replace-me" {
				t.Errorf("ReadEnvFile = %q, want the two values unrun together", values)
			}
		})
	}
}

// spec: agent.md#local-configuration — an error names the problem and the place: the path
// when the file cannot be read, the line number when a line is not KEY=VALUE. It never
// quotes the line, because this error goes to a log and the line may hold the token.
func TestReadEnvFileNamesTheProblemAndThePlace(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.env")
	tests := []struct {
		name     string
		path     string
		want     []string
		unwanted string
	}{
		{name: "missing file", path: missing, want: []string{missing}},
		{
			name:     "a line that is not an assignment",
			path:     envFile(t, "MONITOR_HUB=https://hub.example.com\nMONITOR_TOKEN replace-me\n"),
			want:     []string{"line 2"},
			unwanted: "replace-me",
		},
		{name: "a line with no key", path: envFile(t, "=laptop-a\n"), want: []string{"line 1"}},
		{
			name:     "a key that is not a variable name",
			path:     envFile(t, "\nexport MONITOR_TOKEN=replace-me\n"),
			want:     []string{"line 2"},
			unwanted: "replace-me",
		},
		{
			// A base64 token pasted on a line of its own ends in = , so the text before the
			// first = is the secret itself. Naming it would put it in the log.
			name:     "a token pasted as if it were a key",
			path:     envFile(t, "ab+cd/ef-replace-me=\n"),
			want:     []string{"line 1"},
			unwanted: "replace-me",
		},
		{
			name:     "a value whose quote is never closed",
			path:     envFile(t, "MONITOR_TOKEN=\"replace-me\n"),
			want:     []string{"line 1"},
			unwanted: "replace-me",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := agent.ReadEnvFile(tc.path)

			if err == nil {
				t.Fatalf("ReadEnvFile(%s) = nil error, want one", tc.path)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %v, want it to name %s", err, want)
				}
			}
			if tc.unwanted != "" && strings.Contains(err.Error(), tc.unwanted) {
				t.Errorf("error = %v, want it not to quote the value", err)
			}
		})
	}
}
