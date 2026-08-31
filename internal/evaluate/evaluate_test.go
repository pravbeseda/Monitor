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

// The hub's configuration resolves a rule for every implemented name, so the list is what
// says which rules exist at all.
func TestNamesListsTheImplementedRules(t *testing.T) {
	names := evaluate.Names()
	if len(names) != 1 || names[0] != "disk" {
		t.Fatalf("Names() = %v, want [disk]", names)
	}
}

// spec: history.md#gaps — a series ages by the interval of the sensor its metric comes from.
func TestSensorOfNamesTheSensorBehindAMetric(t *testing.T) {
	if sensor, known := evaluate.SensorOf("disk.free_pct"); !known || sensor != "disk" {
		t.Errorf("SensorOf(disk.free_pct) = %q, %v, want the disk sensor", sensor, known)
	}
	if _, known := evaluate.SensorOf("battery.charge_pct"); known {
		t.Error("SensorOf(battery.charge_pct) claims a sensor, want none: no rule declares it")
	}
}
