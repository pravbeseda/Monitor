package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

// spec: hub-config.md#startup — the deployment paths have no defaults.
func TestParseFlagsRequiresDeploymentPaths(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no flags at all", nil, "--config"},
		{"a configuration but no database", []string{"--config", "config.yaml"}, "--db"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseFlags(tc.args, io.Discard)

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to name %s", err, tc.want)
			}
		})
	}
}

func TestParseFlagsDefaultsToLocalhost(t *testing.T) {
	opts, err := parseFlags([]string{"--config", "config.yaml", "--db", "monitor.db"}, io.Discard)
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}

	if !strings.HasPrefix(opts.listen, "127.0.0.1:") {
		t.Errorf("listen = %q, want the hub bound to localhost (ADR 0005)", opts.listen)
	}
}

// -h is a request, not a failure: it prints the flags and the caller exits without an error.
func TestParseFlagsAnswersHelpWithTheFlagList(t *testing.T) {
	var out bytes.Buffer

	_, err := parseFlags([]string{"-h"}, &out)

	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want flag.ErrHelp", err)
	}
	for _, flagName := range []string{"-config", "-db", "-listen"} {
		if !strings.Contains(out.String(), flagName) {
			t.Errorf("usage = %q, want it to list %s", out.String(), flagName)
		}
	}
}
