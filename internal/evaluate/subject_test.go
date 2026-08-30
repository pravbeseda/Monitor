package evaluate_test

import (
	"testing"
	"time"

	"github.com/pravbeseda/monitor/internal/evaluate"
	"github.com/pravbeseda/monitor/internal/storage"
)

// tick is the instant every subject in this file is evaluated at.
var tick = time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)

// silenceAfter and interval give the node a 10m silence window and, at three collections
// missed, a 45m staleness window.
const (
	silenceAfter = 10 * time.Minute
	interval     = 15 * time.Minute
	staleAfter   = 3 * interval
)

// watching is a node the file lists, judging its volumes by the product defaults.
func watching(t *testing.T) evaluate.Target {
	t.Helper()
	return evaluate.Target{
		Node:         "server-b",
		SilenceAfter: silenceAfter,
		Intervals:    map[string]time.Duration{"disk": interval},
		Rules:        map[string]evaluate.Rule{"disk": disk(t).Default},
	}
}

// volume is the label set of one mounted volume, as the disk sensor reports it.
func volume(mount string) map[string]string {
	return map[string]string{"mount": mount, "fs": "ext4", "removable": "false"}
}

// reported is the pair of series of one volume, both collected age ago.
func reported(labels map[string]string, free, pct float64, age time.Duration) []storage.Value {
	return []storage.Value{
		{Metric: "disk.free_bytes", Labels: labels, Value: free, TS: tick.Add(-age)},
		{Metric: "disk.free_pct", Labels: labels, Value: pct, TS: tick.Add(-age)},
	}
}

// heard is a node last heard from age ago, carrying the given series.
func heard(age time.Duration, values ...storage.Value) storage.NodeState {
	return storage.NodeState{Node: "server-b", LastSeen: tick.Add(-age), Values: values}
}

func stored(rule string, labels map[string]string, level evaluate.Level, since time.Time) storage.State {
	return storage.State{
		Subject: storage.Subject{Node: "server-b", Rule: rule, Labels: labels},
		Level:   level.String(),
		Since:   since,
	}
}

// subjectsOf evaluates one node against one snapshot.
func subjectsOf(t *testing.T, target evaluate.Target, snap storage.Snapshot) []evaluate.Subject {
	t.Helper()
	return evaluate.Subjects([]evaluate.Target{target}, snap, tick)
}

// find returns the subject of one rule and one mount, or fails: an absent subject is a
// different assertion, made with count.
func find(t *testing.T, subjects []evaluate.Subject, rule, mount string) evaluate.Subject {
	t.Helper()
	for _, subject := range subjects {
		if subject.Rule == rule && subject.Mount() == mount {
			return subject
		}
	}
	t.Fatalf("no %s subject for mount %q among %d subjects", rule, mount, len(subjects))
	return evaluate.Subject{}
}

func count(t *testing.T, subjects []evaluate.Subject, rule string) int {
	t.Helper()
	n := 0
	for _, subject := range subjects {
		if subject.Rule == rule {
			n++
		}
	}
	return n
}

// spec: evaluation.md#freezing — stale values are never re-evaluated: a frozen subject
// keeps the level and the `since` it had.
func TestFreezing(t *testing.T) {
	t.Run("the node is silent", func(t *testing.T) {
		snap := storage.Snapshot{Nodes: []storage.NodeState{
			heard(silenceAfter+time.Minute, reported(volume("/"), 40e9, 31.25, time.Minute)...),
		}}
		subjects := subjectsOf(t, watching(t), snap)
		if got := find(t, subjects, "disk", "/"); !got.Frozen {
			t.Fatal("a silent node's volume was evaluated, so stale values decided its level")
		}
		if got := find(t, subjects, "silence", ""); got.Frozen {
			t.Fatal("the silence subject froze itself, so the node could never recover")
		}
	})

	t.Run("the older series is older than stale_after", func(t *testing.T) {
		values := append(reported(volume("/data"), 5e9, 3.91, staleAfter+time.Minute),
			reported(volume("/"), 40e9, 31.25, time.Minute)...)
		subjects := subjectsOf(t, watching(t), storage.Snapshot{Nodes: []storage.NodeState{heard(0, values...)}})
		if got := find(t, subjects, "disk", "/data"); !got.Frozen {
			t.Fatal("a stale volume was evaluated")
		}
		if got := find(t, subjects, "disk", "/"); got.Frozen || got.Level != evaluate.OK {
			t.Fatalf("the fresh volume of the same node came out frozen=%v level=%v", got.Frozen, got.Level)
		}
	})

	t.Run("one series stale and the other fresh", func(t *testing.T) {
		labels := volume("/")
		values := []storage.Value{
			{Metric: "disk.free_bytes", Labels: labels, Value: 5e9, TS: tick.Add(-staleAfter - time.Minute)},
			{Metric: "disk.free_pct", Labels: labels, Value: 3.91, TS: tick.Add(-time.Minute)},
		}
		subjects := subjectsOf(t, watching(t), storage.Snapshot{Nodes: []storage.NodeState{heard(0, values...)}})
		if got := find(t, subjects, "disk", "/"); !got.Frozen {
			t.Fatal("the join was judged as fresh as its younger half")
		}
	})

	t.Run("the second series has never arrived", func(t *testing.T) {
		values := []storage.Value{{Metric: "disk.free_bytes", Labels: volume("/"), Value: 5e9, TS: tick}}
		subjects := subjectsOf(t, watching(t), storage.Snapshot{Nodes: []storage.NodeState{heard(0, values...)}})
		if got := count(t, subjects, "disk"); got != 0 {
			t.Fatalf("an incomplete join produced %d subjects, want none", got)
		}
	})

	t.Run("a removable volume is unplugged", func(t *testing.T) {
		labels := volume("/mnt/usb")
		labels["removable"] = "true"
		values := reported(labels, 5e9, 3.91, staleAfter+time.Minute)
		snap := storage.Snapshot{
			Nodes:  []storage.NodeState{heard(0, values...)},
			States: []storage.State{stored("disk", labels, evaluate.Critical, tick.Add(-time.Hour))},
		}
		got := find(t, subjectsOf(t, watching(t), snap), "disk", "/mnt/usb")
		if !got.Frozen || got.Level != evaluate.Critical || !got.Since.Equal(tick.Add(-time.Hour)) {
			t.Fatalf("an unplugged volume came out frozen=%v level=%v since=%v", got.Frozen, got.Level, got.Since)
		}
	})

	t.Run("the sensor of the rule does not run on this node", func(t *testing.T) {
		target := watching(t)
		target.Intervals = map[string]time.Duration{}
		values := reported(volume("/"), 5e9, 3.91, time.Minute)
		subjects := subjectsOf(t, target, storage.Snapshot{Nodes: []storage.NodeState{heard(0, values...)}})
		if got := count(t, subjects, "disk"); got != 0 {
			t.Fatalf("a rule whose sensor nothing collects produced %d subjects", got)
		}
		find(t, subjects, "silence", "")
	})

	t.Run("a volume reappears under different labels", func(t *testing.T) {
		was, now := volume("/data"), volume("/data")
		now["fs"] = "xfs"
		values := append(reported(was, 5e9, 3.91, staleAfter+time.Minute),
			reported(now, 5e9, 3.91, time.Minute)...)
		snap := storage.Snapshot{
			Nodes:  []storage.NodeState{heard(0, values...)},
			States: []storage.State{stored("disk", was, evaluate.Critical, tick.Add(-time.Hour))},
		}
		subjects := subjectsOf(t, watching(t), snap)
		if got := count(t, subjects, "disk"); got != 2 {
			t.Fatalf("relabelling gave %d subjects, want the old one and the new", got)
		}
		for _, subject := range subjects {
			if subject.Rule != "disk" {
				continue
			}
			if subject.Labels["fs"] == "ext4" && (!subject.Frozen || subject.Level != evaluate.Critical) {
				t.Fatalf("the old subject came out frozen=%v level=%v", subject.Frozen, subject.Level)
			}
			if subject.Labels["fs"] == "xfs" && (subject.Frozen || subject.Previous != evaluate.OK) {
				t.Fatalf("the new subject started at %v, want a new subject at ok", subject.Previous)
			}
		}
	})
}

// spec: evaluation.md#node-silence — the node is a subject too, with the window its class
// resolves to.
func TestNodeSilence(t *testing.T) {
	t.Run("silent past the window", func(t *testing.T) {
		snap := storage.Snapshot{Nodes: []storage.NodeState{heard(silenceAfter + time.Second)}}
		if got := find(t, subjectsOf(t, watching(t), snap), "silence", ""); got.Level != evaluate.Critical {
			t.Fatalf("a node past its silence window is %v, want critical", got.Level)
		}
	})

	t.Run("heard from inside the window", func(t *testing.T) {
		snap := storage.Snapshot{
			Nodes:  []storage.NodeState{heard(silenceAfter - time.Second)},
			States: []storage.State{stored("silence", nil, evaluate.Critical, tick.Add(-time.Hour))},
		}
		got := find(t, subjectsOf(t, watching(t), snap), "silence", "")
		if got.Level != evaluate.OK || got.Previous != evaluate.Critical {
			t.Fatalf("a node heard from again is %v from %v, want ok from critical", got.Level, got.Previous)
		}
	})

	t.Run("still inside the window", func(t *testing.T) {
		snap := storage.Snapshot{Nodes: []storage.NodeState{heard(time.Minute)}}
		if got := find(t, subjectsOf(t, watching(t), snap), "silence", ""); got.Level != evaluate.OK || got.Changed() {
			t.Fatalf("a healthy node changed to %v", got.Level)
		}
	})

	t.Run("a node that has never reported", func(t *testing.T) {
		if got := evaluate.Subjects([]evaluate.Target{watching(t)}, storage.Snapshot{}, tick); len(got) != 0 {
			t.Fatalf("an uninstalled agent produced %d subjects, want none", len(got))
		}
	})
}

// spec: evaluation.md#the-tick — messages and digest entries come out by node name, then by
// the subject's mount label.
func TestSubjectsComeOutInAStableOrder(t *testing.T) {
	values := append(reported(volume("/data"), 40e9, 31.25, time.Minute),
		reported(volume("/"), 40e9, 31.25, time.Minute)...)
	first, second := watching(t), watching(t)
	second.Node = "laptop-a"
	snap := storage.Snapshot{Nodes: []storage.NodeState{
		heard(0, values...),
		{Node: "laptop-a", LastSeen: tick},
	}}

	got := evaluate.Subjects([]evaluate.Target{first, second}, snap, tick)
	want := []string{"laptop-a/silence/", "server-b/silence/", "server-b/disk//", "server-b/disk//data"}
	if len(got) != len(want) {
		t.Fatalf("got %d subjects, want %d", len(got), len(want))
	}
	for i, subject := range got {
		if key := subject.Node + "/" + subject.Rule + "/" + subject.Mount(); key != want[i] {
			t.Fatalf("subject %d is %s, want %s", i, key, want[i])
		}
	}
}
