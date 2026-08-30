package config_test

import (
	"testing"

	"github.com/pravbeseda/monitor/internal/config"
	"github.com/pravbeseda/monitor/internal/evaluate"
)

func target(t *testing.T, cfg *config.Config, name string) evaluate.Target {
	t.Helper()
	return node(t, cfg, name).Target()
}

// spec: evaluation.md#freezing — a sensor the node resolves as disabled collects nothing,
// so the rule that reads it has no subjects on that node.
func TestASwitchedOffSensorLeavesNoInterval(t *testing.T) {
	got := target(t, load(t, `
nodes:
  laptop-a:
    class: laptop
    token_env: MONITOR_TOKEN_LAPTOP_A
    sensors: { disk: { enabled: false } }
`), "laptop-a")
	if interval, runs := got.Intervals["disk"]; runs {
		t.Fatalf("a disabled sensor resolved an interval of %v, so its rule would still have subjects", interval)
	}
}

// spec: evaluation.md#freezing — a sensor no layer delivers is not collected either, and
// the rule that reads it has no subjects for the same reason.
func TestAnUndeliveredSensorLeavesNoInterval(t *testing.T) {
	got := target(t, load(t, minimal), "laptop-a")
	if interval, runs := got.Intervals["nosuchsensor"]; runs {
		t.Fatalf("a sensor no layer mentions resolved an interval of %v", interval)
	}
	if _, runs := got.Intervals["disk"]; !runs {
		t.Fatal("the disk sensor of the laptop profile resolved no interval")
	}
}

// spec: evaluation.md#configuration — a target carries the thresholds of the node and of
// every volume it names, which is what evaluation judges a mount by.
func TestATargetCarriesTheNodesWindowAndRules(t *testing.T) {
	cfg := load(t, `
classes:
  laptop: { silence_after: 48h }
nodes:
  laptop-a:
    class: laptop
    token_env: MONITOR_TOKEN_LAPTOP_A
    volumes:
      "/data/backup": { role: backup }
`)
	got := target(t, cfg, "laptop-a")
	if got.Node != "laptop-a" || got.SilenceAfter != node(t, cfg, "laptop-a").SilenceAfter {
		t.Fatalf("target %s carries a silence window of %v", got.Node, got.SilenceAfter)
	}
	backup, ok := got.Rule("disk", "/data/backup")
	if !ok || backup.Warning.Ratio != 0 {
		t.Fatalf("the backup volume resolved %+v, want the band-less rule", backup)
	}
	plain, ok := got.Rule("disk", "/")
	if !ok || plain.Warning.Ratio == 0 {
		t.Fatalf("an unnamed volume resolved %+v, want the node's own rule", plain)
	}
}
