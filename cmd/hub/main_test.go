package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// spec: hub-config.md#startup — the configuration path has no default.
func TestConfigPathRequired(t *testing.T) {
	_, err := configPath(nil)

	if err == nil || !strings.Contains(err.Error(), "--config") {
		t.Fatalf("error = %v, want it to name the missing flag", err)
	}
}

func TestRunReportsConfiguredNodes(t *testing.T) {
	t.Setenv("MONITOR_TOKEN_LAPTOP_A", strings.Repeat("synthetic-", 4))
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := "nodes:\n  laptop-a:\n    class: laptop\n    token_env: MONITOR_TOKEN_LAPTOP_A\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var out bytes.Buffer
	if err := run([]string{"--config", path}, &out); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !strings.Contains(out.String(), "1 node") {
		t.Errorf("output = %q, want the number of configured nodes", out.String())
	}
}
