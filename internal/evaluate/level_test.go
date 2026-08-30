package evaluate_test

import (
	"testing"

	"github.com/pravbeseda/monitor/internal/evaluate"
)

// Sizes are decimal, the way the specs and the interface write them.
func gb(n float64) float64 { return n * 1e9 }
func tb(n float64) float64 { return n * 1e12 }

func disk(t *testing.T) evaluate.Definition {
	t.Helper()
	found, ok := evaluate.Lookup("disk")
	if !ok {
		t.Fatal("the hub has no disk rule")
	}
	return found
}

// levelCase is one row of a behaviour table: what the subject was, what the volume
// reports, and the level that follows.
type levelCase struct {
	name     string
	previous evaluate.Level
	free     float64
	pct      float64
	want     evaluate.Level
}

func run(t *testing.T, rule evaluate.Rule, cases []levelCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rule.Level(c.previous, c.free, c.pct); got != c.want {
				t.Fatalf("Level(%v, %g, %g) = %v, want %v", c.previous, c.free, c.pct, got, c.want)
			}
		})
	}
}

// spec: evaluation.md#levels — a 128 GB volume unless the row says otherwise.
func TestLevels(t *testing.T) {
	run(t, disk(t).Default, []levelCase{
		{"neither arm holds", evaluate.OK, gb(40), 31.25, evaluate.OK},
		{"the floor comparison is strict", evaluate.OK, gb(10), 25.00, evaluate.OK},
		{"band under the ratio and the ceiling", evaluate.OK, gb(19), 14.84, evaluate.Warning},
		{"the ratio comparison is strict", evaluate.OK, gb(19.2), 15.00, evaluate.OK},
		{"floor whatever the percentage", evaluate.OK, gb(9), 45.00, evaluate.Warning},
		{"band on a large volume", evaluate.OK, gb(99), 1.24, evaluate.Warning},
		{"the ceiling guards the band", evaluate.OK, tb(1.1), 13.75, evaluate.OK},
		{"the critical band alone", evaluate.OK, gb(5), 3.91, evaluate.Critical},
		{"the critical floor is strict", evaluate.OK, gb(4), 10.00, evaluate.Warning},
		{"above the critical ceiling", evaluate.OK, gb(45), 5.00, evaluate.Warning},
		{"the critical ceiling comparison is strict", evaluate.OK, gb(40), 4.00, evaluate.Warning},
		{"both critical arms hold", evaluate.OK, gb(3), 2.34, evaluate.Critical},
		{"the more severe level wins", evaluate.Warning, gb(3), 2.34, evaluate.Critical},
		{"a full volume", evaluate.OK, 0, 0, evaluate.Critical},
	})
}

// spec: evaluation.md#hysteresis — recovery is the negated entry rule with a 20% margin on
// every comparison, never a per-condition clearance.
func TestHysteresis(t *testing.T) {
	run(t, disk(t).Default, []levelCase{
		{"below the floor margin", evaluate.Warning, gb(11), 27.50, evaluate.Warning},
		{"clears the floor with its margin", evaluate.Warning, gb(12), 30.00, evaluate.OK},
		{"below the ratio margin", evaluate.Warning, gb(20), 15.63, evaluate.Warning},
		{"one step below the ratio margin", evaluate.Warning, gb(23), 17.97, evaluate.Warning},
		{"exactly the ratio margin", evaluate.Warning, gb(23.04), 18.00, evaluate.OK},
		{"clears both floor and ratio margins", evaluate.Warning, gb(24), 18.75, evaluate.OK},
		{"the rule re-enters at that size", evaluate.Warning, gb(12), 9.38, evaluate.Warning},
		{"the ceiling comparison clears first", evaluate.Warning, gb(120), 1.50, evaluate.OK},
		{"only the critical floor margin holds it", evaluate.Critical, gb(4.5), 10.00, evaluate.Critical},
		{"hysteresis never raises a level", evaluate.Warning, gb(4.5), 10.00, evaluate.Warning},
		{"clears critical, still under the warning floor", evaluate.Critical, gb(6), 30.00, evaluate.Warning},
		{"clears both levels in one tick", evaluate.Critical, gb(24), 18.75, evaluate.OK},
	})
}

// spec: evaluation.md#backup-volumes — a 2 TB volume declared role: backup, which drops the
// band and keeps absolute headroom.
func TestBackupVolumes(t *testing.T) {
	run(t, disk(t).Backup, []levelCase{
		{"under the warning floor", evaluate.OK, gb(40), 2.00, evaluate.Warning},
		{"below the warning margin", evaluate.Warning, gb(55), 2.75, evaluate.Warning},
		{"clears the warning floor with its margin", evaluate.Warning, gb(60), 3.00, evaluate.OK},
		{"the default rule would warn here", evaluate.OK, gb(70), 3.50, evaluate.OK},
		{"under the critical floor", evaluate.OK, gb(9), 0.45, evaluate.Critical},
		{"below the critical margin", evaluate.Critical, gb(11), 0.55, evaluate.Critical},
		{"clears the critical margin", evaluate.Critical, gb(12), 0.60, evaluate.Warning},
	})
}

// A margin lands exactly on a two-decimal percentage whenever the ratio is a multiple of
// 0.05, and such a value has to count as cleared however the product is computed.
func TestRatioMarginIsExactOnTwoDecimals(t *testing.T) {
	rule := evaluate.Rule{Warning: evaluate.Threshold{Floor: gb(10), Ratio: 6.7, Ceiling: gb(100)}}
	if got := rule.Level(evaluate.Warning, gb(50), 8.04); got != evaluate.OK {
		t.Fatalf("8.04%% is exactly 20%% above 6.7%%, so the level is %v, want ok", got)
	}
}

// A band needs both halves; the hub refuses a configuration carrying one of them
// (spec: evaluation.md#startup-validation), and the engine ignores what it cannot read as
// a band rather than half-applying it.
func TestHalfABandIsNoBand(t *testing.T) {
	rule := evaluate.Rule{Warning: evaluate.Threshold{Floor: gb(10), Ratio: 15}}
	if got := rule.Level(evaluate.OK, gb(19), 14.84); got != evaluate.OK {
		t.Fatalf("a ratio with no ceiling gave %v, want ok: the floor alone decides", got)
	}
}

// A level read back from storage may have been written by another build, so an unknown
// name is refused rather than guessed at.
func TestParseLevel(t *testing.T) {
	for _, want := range []evaluate.Level{evaluate.OK, evaluate.Warning, evaluate.Critical} {
		if got, ok := evaluate.ParseLevel(want.String()); !ok || got != want {
			t.Fatalf("ParseLevel(%q) = %v, %v; want %v, true", want.String(), got, ok, want)
		}
	}
	for _, text := range []string{"", "unknown", "OK", "warn"} {
		if got, ok := evaluate.ParseLevel(text); ok {
			t.Fatalf("ParseLevel(%q) = %v, true; want it refused", text, got)
		}
	}
}

// Storage keeps a level as its name, so the names are part of the stored form.
func TestLevelNames(t *testing.T) {
	for level, want := range map[evaluate.Level]string{
		evaluate.OK: "ok", evaluate.Warning: "warning", evaluate.Critical: "critical",
		evaluate.Level(9): "unknown",
	} {
		if got := level.String(); got != want {
			t.Fatalf("Level(%d).String() = %q, want %q", int(level), got, want)
		}
	}
}

// A configured ratio carries whatever the file wrote, so its margin is not quantised to the
// two decimals the sensor reports: 20% above 6.755 is 8.106, and 8.11% has cleared it.
func TestARatioFinerThanTwoDecimalsKeepsItsMargin(t *testing.T) {
	rule := evaluate.Rule{Warning: evaluate.Threshold{Floor: gb(10), Ratio: 6.755, Ceiling: gb(100)}}
	if got := rule.Level(evaluate.Warning, gb(50), 8.11); got != evaluate.OK {
		t.Fatalf("8.11%% is above the 8.106%% margin of 6.755%%, so the level is %v, want ok", got)
	}
	if got := rule.Level(evaluate.Warning, gb(50), 8.10); got != evaluate.Warning {
		t.Fatalf("8.10%% is below that margin, so the level is %v, want warning", got)
	}
}

// The margin of a ratio finer than a sensor step is compared at full precision in both
// directions: 20% above 6.759 is 8.1108, which 8.11% has not reached.
func TestARatioBetweenSensorStepsIsNotRoundedIntoTheMargin(t *testing.T) {
	rule := evaluate.Rule{Warning: evaluate.Threshold{Floor: gb(10), Ratio: 6.759, Ceiling: gb(100)}}
	if got := rule.Level(evaluate.Warning, gb(50), 8.11); got != evaluate.Warning {
		t.Fatalf("8.11%% is below the 8.1108%% margin of 6.759%%, so the level is %v, want warning", got)
	}
	if got := rule.Level(evaluate.Warning, gb(50), 8.12); got != evaluate.OK {
		t.Fatalf("8.12%% clears that margin, so the level is %v, want ok", got)
	}
}
