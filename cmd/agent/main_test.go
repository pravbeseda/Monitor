package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

// spec: agent.md#local-configuration — all three values are deployment settings.
func TestSettingsHaveNoDefaults(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		token string
		want  string
	}{
		{"no hub", nil, "secret", "--hub"},
		{"no node", []string{"--hub", "https://hub.example"}, "secret", "--node"},
		{"no token", []string{"--hub", "https://hub.example", "--node", "laptop-a"}, "", "MONITOR_TOKEN"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tokenVariable, tc.token)

			_, err := settings(tc.args, io.Discard)

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to name %s", err, tc.want)
			}
		})
	}
}

// -h is a request, not a failure: it prints the flags and the caller exits without an error.
func TestSettingsAnswersHelpWithTheFlagList(t *testing.T) {
	var out bytes.Buffer

	_, err := settings([]string{"-h"}, &out)

	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want flag.ErrHelp", err)
	}
	for _, flagName := range []string{"-hub", "-node"} {
		if !strings.Contains(out.String(), flagName) {
			t.Errorf("usage = %q, want it to list %s", out.String(), flagName)
		}
	}
}
