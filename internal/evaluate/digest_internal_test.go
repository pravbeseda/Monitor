package evaluate

import (
	"testing"
	"time"
)

// spec: evaluation.md#digest — an hour a DST change removes is normalised forward, so the
// day is not skipped.
func TestAnHourRemovedByADSTChangeIsNormalisedForward(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("this system carries no Europe/Berlin: %v", err)
	}
	// Clocks jump from 02:00 to 03:00 on 2026-03-29, so 02:30 does not exist that day.
	schedule := Schedule{Hour: 2, Minute: 30, Location: berlin}
	got := schedule.mostRecent(time.Date(2026, time.March, 29, 12, 0, 0, 0, berlin))

	want := time.Date(2026, time.March, 29, 3, 30, 0, 0, berlin)
	if !got.Equal(want) {
		t.Fatalf("the occurrence is %v, want %v: the day must not be skipped", got, want)
	}
}
