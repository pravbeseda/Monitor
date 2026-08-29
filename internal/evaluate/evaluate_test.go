package evaluate_test

import (
	"testing"

	"github.com/pravbeseda/monitor/internal/evaluate"
)

// A rule names the sensor its metrics come from, so staleness is computable: a subject is
// frozen once its values are older than a multiple of that sensor's interval.
func TestDiskRuleNamesItsSensorAndSeries(t *testing.T) {
	found := disk(t)
	if found.Sensor != "disk" {
		t.Fatalf("the disk rule reads sensor %q, want disk", found.Sensor)
	}
	if found.Free != "disk.free_bytes" || found.Pct != "disk.free_pct" {
		t.Fatalf("the disk rule reads %q and %q", found.Free, found.Pct)
	}
}

// Silence is a subject, not a configurable rule: it has no thresholds, only the window its
// node class carries.
func TestSilenceIsNotAConfigurableRule(t *testing.T) {
	if _, ok := evaluate.Lookup("silence"); ok {
		t.Fatal("silence is configurable, which would give it thresholds it has no use for")
	}
}
