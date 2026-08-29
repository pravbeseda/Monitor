package config_test

import (
	"strings"
	"testing"

	"github.com/pravbeseda/monitor/internal/config"
	"github.com/pravbeseda/monitor/internal/evaluate"
)

func rule(t *testing.T, cfg *config.Config, nodeName, ruleName, mount string) evaluate.Rule {
	t.Helper()
	found, ok := node(t, cfg, nodeName).Rule(ruleName, mount)
	if !ok {
		t.Fatalf("node %s has no %s rule for %q", nodeName, ruleName, mount)
	}
	return found
}

// volumes builds a file whose only node declares one volume.
func volumes(body string) string {
	return `
nodes:
  laptop-a:
    class: laptop
    token_env: MONITOR_TOKEN_LAPTOP_A
    volumes:
` + body
}

// spec: evaluation.md#startup-validation — what a layer says on its own terms is checked at
// every layer, and every error names the key that has to be fixed.
func TestRulesReject(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "a rule name no rule reads",
			body: "rules:\n  disc:\n    warning: { floor: 10GB, ratio: 15, ceiling: 100GB }\n" + minimal,
			want: "disc",
		},
		{
			name: "a size with no unit",
			body: "rules:\n  disk:\n    warning: { floor: 10000000000 }\n" + minimal,
			want: "floor",
		},
		{
			name: "a size with an unknown unit",
			body: "rules:\n  disk:\n    warning: { floor: 10GiB }\n" + minimal,
			want: "10GiB",
		},
		{
			name: "a size spelled as a Go literal",
			body: "rules:\n  disk:\n    warning: { floor: 0x1p3GB }\n" + minimal,
			want: "0x1p3GB",
		},
		{
			name: "a size with a digit separator",
			body: "rules:\n  disk:\n    warning: { floor: 1_0GB }\n" + minimal,
			want: "1_0GB",
		},
		{
			name: "a negative zero size",
			body: "rules:\n  disk:\n    warning: { floor: -0GB }\n" + minimal,
			want: "floor",
		},
		{
			name: "a size written empty",
			body: "rules:\n  disk:\n    warning: { floor: '' }\n" + minimal,
			want: "size",
		},
		{
			name: "a ratio above a hundred",
			body: "rules:\n  disk:\n    warning: { ratio: 140 }\n" + minimal,
			want: "ratio",
		},
		{
			name: "a ratio that is not a number",
			body: "rules:\n  disk:\n    warning: { ratio: .nan }\n" + minimal,
			want: "ratio",
		},
		{
			name: "a ceiling without a ratio",
			body: "rules:\n  disk:\n    warning: { ratio: 0, ceiling: 90GB }\n" + minimal,
			want: "band",
		},
		{
			name: "a ratio under a backup branch",
			body: "rules:\n  disk:\n    backup: { warning: { floor: 50GB, ratio: 15 } }\n" + minimal,
			want: "backup",
		},
		{
			name: "a critical floor above the warning floor",
			body: "rules:\n  disk:\n    critical: { floor: 40GB, ratio: 7, ceiling: 40GB }\n" + minimal,
			want: "floor",
		},
		{
			name: "a critical ratio above the warning ratio",
			body: "rules:\n  disk:\n    critical: { floor: 4GB, ratio: 20, ceiling: 40GB }\n" + minimal,
			want: "ratio",
		},
		{
			name: "a critical ceiling above the warning ceiling",
			body: "rules:\n  disk:\n    critical: { floor: 4GB, ratio: 7, ceiling: 200GB }\n" + minimal,
			want: "ceiling",
		},
		{
			name: "an inversion no single layer carries",
			body: "classes:\n  laptop:\n    rules:\n      disk:\n        warning: { ratio: 10 }\n" +
				minimal + "    rules:\n      disk:\n        critical: { ratio: 12 }\n",
			want: "ratio",
		},
		{
			name: "a bad rule on a class no node uses",
			body: "classes:\n  spare:\n    silence_after: 10m\n    rules:\n      disk:\n        warning: { floor: 10GiB }\n" + minimal,
			want: "spare",
		},
		{
			name: "a role that is not backup",
			body: volumes("      \"/data\": { role: archive }\n"),
			want: "archive",
		},
		{
			name: "a bad size on a volume",
			body: volumes("      \"/data\":\n        rules:\n          disk:\n            warning: { floor: 10GiB }\n"),
			want: "/data",
		},
		{
			name: "an unknown rule on a volume",
			body: volumes("      \"/data\":\n        rules:\n          disc:\n            warning: { floor: 10GB }\n"),
			want: "disc",
		},
		{
			name: "an inversion introduced by a volume",
			body: volumes("      \"/data\":\n        rules:\n          disk:\n            warning: { floor: 1GB }\n"),
			want: "critical floor",
		},
		{
			name: "a backup branch on a volume",
			body: volumes("      \"/data\":\n        rules:\n          disk:\n            backup: { warning: { floor: 1GB } }\n"),
			want: "writes thresholds directly",
		},
		{
			name: "a band on a volume that is a backup",
			body: volumes("      \"/data\":\n        role: backup\n        rules:\n          disk:\n            warning: { ratio: 15, ceiling: 100GB }\n            critical: { ratio: 7, ceiling: 40GB }\n"),
			want: "a backup rule is a floor",
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(tokenEnv, token)
			_, err := config.Load(write(t, c.body))
			if err == nil {
				t.Fatal("the file was accepted")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("the error does not name %q: %v", c.want, err)
			}
		})
	}
}

// spec: hub-config.md#invariants — no check compares one layer with another: a layer that
// looks inconsistent on its own may be made whole by the layer above it.
func TestALayerNeedNotStandAlone(t *testing.T) {
	cfg := load(t, `
rules:
  disk:
    critical: { floor: 50GB, ratio: 7, ceiling: 40GB }
nodes:
  laptop-a:
    class: laptop
    token_env: MONITOR_TOKEN_LAPTOP_A
    rules:
      disk:
        warning: { floor: 60GB }
`)
	if got := rule(t, cfg, "laptop-a", "disk", "/"); got.Critical.Floor != 50e9 || got.Warning.Floor != 60e9 {
		t.Fatalf("the resolved rule is %+v", got)
	}
}

// A top-level mistake names the top-level key, not whichever class happens to sort first.
func TestATopLevelMistakeNamesItsOwnKey(t *testing.T) {
	t.Setenv(tokenEnv, token)
	_, err := config.Load(write(t, "rules:\n  disk:\n    warning: { floor: 10GiB }\n"+minimal))
	if err == nil {
		t.Fatal("the file was accepted")
	}
	if strings.Contains(err.Error(), "class") {
		t.Fatalf("a top-level mistake was blamed on a class: %v", err)
	}
}

// spec: evaluation.md#levels — with nothing declared a node judges its volumes by the
// product defaults of ADR 0012.
func TestRulesDefaultToTheProductRule(t *testing.T) {
	got := rule(t, load(t, minimal), "laptop-a", "disk", "/")
	if got.Warning != (evaluate.Threshold{Floor: 10e9, Ratio: 15, Ceiling: 100e9}) {
		t.Fatalf("warning threshold is %+v", got.Warning)
	}
	if got.Critical != (evaluate.Threshold{Floor: 4e9, Ratio: 7, Ceiling: 40e9}) {
		t.Fatalf("critical threshold is %+v", got.Critical)
	}
}

// spec: evaluation.md#configuration — product default → top level → class → node → volume,
// merged field by field so a layer may move one number and keep the rest.
func TestRulesLayerMostSpecificLast(t *testing.T) {
	layered := func(t *testing.T, node, volume string) *config.Config {
		t.Helper()
		return load(t, `
rules:
  disk:
    warning: { floor: 20GB }
classes:
  laptop:
    rules:
      disk:
        warning: { floor: 30GB }
nodes:
  laptop-a:
    class: laptop
    token_env: MONITOR_TOKEN_LAPTOP_A
`+node+volume)
	}

	topLevelOnly := load(t, "rules:\n  disk:\n    warning: { floor: 20GB }\n"+minimal)
	if got := rule(t, topLevelOnly, "laptop-a", "disk", "/").Warning.Floor; got != 20e9 {
		t.Fatalf("the top level gave floor %g, want 20e9", got)
	}

	classOnly := layered(t, "", "")
	if got := rule(t, classOnly, "laptop-a", "disk", "/").Warning.Floor; got != 30e9 {
		t.Fatalf("the class did not win over the top level: floor %g", got)
	}

	withNode := layered(t, "    rules:\n      disk:\n        warning: { floor: 40GB }\n", "")
	if got := rule(t, withNode, "laptop-a", "disk", "/").Warning.Floor; got != 40e9 {
		t.Fatalf("the node did not win over the class: floor %g", got)
	}

	withVolume := layered(t,
		"    rules:\n      disk:\n        warning: { floor: 40GB }\n",
		"    volumes:\n      \"/data\":\n        rules:\n          disk:\n            warning: { floor: 50GB }\n")
	if got := rule(t, withVolume, "laptop-a", "disk", "/data").Warning; got.Floor != 50e9 {
		t.Fatalf("the volume did not win over the node: floor %g", got.Floor)
	} else if got.Ratio != 15 {
		t.Fatalf("a layer moving the floor dropped the ratio: %+v", got)
	}
}

// spec: evaluation.md#backup-volumes — role: backup selects a rule of its own, which keeps
// headroom and has no band; the branches never merge into one another.
func TestBackupRoleSelectsItsOwnBranch(t *testing.T) {
	cfg := load(t, volumes("      \"/data/backup\": { role: backup }\n"))
	got := rule(t, cfg, "laptop-a", "disk", "/data/backup")
	if got.Warning.Floor != 50e9 || got.Critical.Floor != 10e9 {
		t.Fatalf("backup thresholds are %+v / %+v", got.Warning, got.Critical)
	}
	if got.Warning.Ratio != 0 || got.Warning.Ceiling != 0 {
		t.Fatalf("the backup branch inherited a band: %+v", got.Warning)
	}
}

// A backup branch declared in the file layers onto the backup default, and reaches only the
// volumes whose role selects it.
func TestBackupBranchLayersOnItsOwnDefault(t *testing.T) {
	cfg := load(t, `
rules:
  disk:
    backup:
      warning: { floor: 80GB }
nodes:
  laptop-a:
    class: laptop
    token_env: MONITOR_TOKEN_LAPTOP_A
    volumes:
      "/data/backup": { role: backup }
      "/data": {}
`)
	if got := rule(t, cfg, "laptop-a", "disk", "/data/backup").Warning; got.Floor != 80e9 || got.Ceiling != 0 {
		t.Fatalf("the backup volume resolved to %+v", got)
	}
	if got := rule(t, cfg, "laptop-a", "disk", "/data").Warning; got.Floor != 10e9 {
		t.Fatalf("a plain volume took the backup branch: %+v", got)
	}
}

// spec: evaluation.md#configuration-changes — a volumes key matches a mount byte for byte,
// so a trailing slash is a different volume.
func TestVolumeKeysAreMatchedByteForByte(t *testing.T) {
	cfg := load(t, volumes("      \"/data/backup/\": { role: backup }\n"))
	if got := rule(t, cfg, "laptop-a", "disk", "/data/backup").Warning; got.Floor != 10e9 {
		t.Fatalf("a trailing slash matched a different mount: floor %g", got.Floor)
	}
}

// spec: hub-config.md#configuration-version — thresholds never reach an agent, so editing
// them cannot make one re-fetch its configuration.
func TestRulesDoNotChangeTheConfigurationVersion(t *testing.T) {
	plain := load(t, minimal)
	withThresholds := load(t, `
rules:
  disk:
    warning: { floor: 80GB, ratio: 40, ceiling: 900GB }
nodes:
  laptop-a:
    class: laptop
    token_env: MONITOR_TOKEN_LAPTOP_A
    volumes:
      "/data": { role: backup }
`)
	if a, b := node(t, plain, "laptop-a").Version, node(t, withThresholds, "laptop-a").Version; a != b {
		t.Fatalf("the version changed with a hub-only value: %s vs %s", a, b)
	}
}

// A rule the hub does not implement has no thresholds to hand out.
func TestUnknownRuleHasNoThresholds(t *testing.T) {
	if _, ok := node(t, load(t, minimal), "laptop-a").Rule("silence", "/"); ok {
		t.Fatal("silence resolved to a rule")
	}
}
