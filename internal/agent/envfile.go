package agent

import (
	"fmt"
	"os"
	"strings"
)

// ReadEnvFile reads a KEY=VALUE file as inert data, close to what systemd's EnvironmentFile=
// accepts: blank lines and lines starting with # are skipped, one matching pair of
// surrounding quotes is stripped, and everything else in a value is literal. Nothing is
// expanded, substituted or executed — a shell sourcing the same file would run the tail of a
// token that had a space pasted into it
// (docs/decisions/0020-agent-reads-its-environment-file.md). Where systemd warns and carries
// on, this refuses: a line it cannot read is an error, not a value quietly left wrong.
func ReadEnvFile(path string) (map[string]string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read the environment file: %w", err)
	}

	values := map[string]string{}
	for number, line := range strings.Split(newlines.Replace(string(body)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// An error quotes no part of the line. Everything after the = is a secret or a
		// deployment setting, and the text before it is the secret too when a token was
		// pasted on a line of its own: a base64 token ends in =. This error reaches a log
		// the operator forwards.
		key, value, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("%s line %d: want KEY=VALUE, found no =", path, number+1)
		}
		if key = strings.TrimSpace(key); !isVariableName(key) {
			return nil, fmt.Errorf("%s line %d: what stands before the = is not a variable name", path, number+1)
		}
		value, closed := unquote(strings.TrimSpace(value))
		if !closed {
			return nil, fmt.Errorf("%s line %d: the value opens with a quote that is never closed", path, number+1)
		}
		values[key] = value
	}
	return values, nil
}

// isVariableName rejects a line a shell would have accepted and this parser must not guess
// at, such as `export MONITOR_TOKEN=…`.
func isVariableName(key string) bool {
	for i, r := range key {
		letter := r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z'
		digit := i > 0 && r >= '0' && r <= '9'
		if letter || digit {
			continue
		}
		return false
	}
	return key != ""
}

// newlines makes a line end where any system ends one, so that a lone carriage return does
// not run two records together and carry the second into the first one's value.
var newlines = strings.NewReplacer("\r\n", "\n", "\r", "\n")

// unquote strips one matching pair of surrounding quotes. What is inside them stays exactly
// as it was written: the quotes say where the value ends, nothing more. A value that opens
// with a quote and never closes it is reported rather than guessed at — the alternative is a
// token silently carrying a quote and a node that authenticates nowhere.
func unquote(value string) (string, bool) {
	quote := byte(0)
	if len(value) > 0 && (value[0] == '\'' || value[0] == '"') {
		quote = value[0]
	}
	if quote == 0 {
		return value, true
	}
	if len(value) < 2 || value[len(value)-1] != quote {
		return "", false
	}
	return value[1 : len(value)-1], true
}
