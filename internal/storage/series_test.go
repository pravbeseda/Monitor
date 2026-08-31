package storage

import (
	"context"
	"testing"
	"time"
)

func pct(mount string, at time.Time, value float64) Measurement {
	return Measurement{
		Metric: "disk.free_pct",
		Labels: map[string]string{"mount": mount},
		Value:  value,
		TS:     at,
	}
}

func store(t *testing.T, db *SQLite, node string, ms ...Measurement) {
	t.Helper()
	if err := db.SaveIngest(context.Background(), ingest(node, collected, ms...)); err != nil {
		t.Fatalf("SaveIngest: %v", err)
	}
}

// spec: history.md#selection — series come ordered by node, then by labels; points oldest first.
func TestPointsOrdersSeriesByNodeThenLabels(t *testing.T) {
	db := open(t)
	store(t, db, "server-b", pct("/data", collected, 50), pct("/", collected.Add(time.Minute), 41), pct("/", collected, 42))
	store(t, db, "laptop-a", pct("/", collected, 80))

	got, err := db.Points(context.Background(), Selection{Metric: "disk.free_pct"}, collected.Add(-time.Hour))
	if err != nil {
		t.Fatalf("Points: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("series = %d, want 3", len(got))
	}
	if got[0].Node != "laptop-a" || got[1].Labels["mount"] != "/" || got[2].Labels["mount"] != "/data" {
		t.Fatalf("order = %+v, want laptop-a, then server-b / then server-b /data", got)
	}
	if len(got[1].Points) != 2 || got[1].Points[0].Value != 42 || got[1].Points[1].Value != 41 {
		t.Fatalf("points of server-b / = %+v, want 42 then 41", got[1].Points)
	}
}

// spec: history.md#selection — node given: only that node's series.
func TestPointsSelectsOneNode(t *testing.T) {
	db := open(t)
	store(t, db, "server-b", pct("/", collected, 42))
	store(t, db, "laptop-a", pct("/", collected, 80))

	got, err := db.Points(context.Background(), Selection{Metric: "disk.free_pct", Node: "laptop-a"}, collected.Add(-time.Hour))
	if err != nil {
		t.Fatalf("Points: %v", err)
	}
	if len(got) != 1 || got[0].Node != "laptop-a" {
		t.Fatalf("series = %+v, want laptop-a alone", got)
	}
}

// spec: history.md#window — a point stored before the window is not read.
func TestPointsSkipsWhatIsOlderThanTheWindow(t *testing.T) {
	db := open(t)
	store(t, db, "server-b", pct("/", collected.Add(-2*time.Hour), 90), pct("/", collected, 42))

	got, err := db.Points(context.Background(), Selection{Metric: "disk.free_pct"}, collected.Add(-time.Hour))
	if err != nil {
		t.Fatalf("Points: %v", err)
	}
	if len(got) != 1 || len(got[0].Points) != 1 || got[0].Points[0].Value != 42 {
		t.Fatalf("points = %+v, want the one inside the window", got)
	}
}

// spec: history.md#selection — /api/v1/series lists series whose last point is older than any window.
func TestSeriesListsWhatExistsWithoutPoints(t *testing.T) {
	db := open(t)
	store(t, db, "server-b", pct("/", collected.Add(-365*24*time.Hour), 90), pct("/data", collected, 50))
	store(t, db, "server-b", free("/", 123))

	got, err := db.Series(context.Background(), Selection{Metric: "disk.free_pct"})
	if err != nil {
		t.Fatalf("Series: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("series = %+v, want both mounts of disk.free_pct", got)
	}
	if got[0].Labels["mount"] != "/" || got[1].Labels["mount"] != "/data" {
		t.Fatalf("order = %+v, want / before /data", got)
	}
}

// spec: history.md#selection — series are ordered by the labels rendered as sorted
// key=value pairs, which is not the order their stored encoding compares in.
func TestSeriesOrdersByRenderedLabelsNotTheirEncoding(t *testing.T) {
	db := open(t)
	both := Measurement{
		Metric: "disk.free_pct",
		Labels: map[string]string{"mount": "/", "removable": "false"},
		Value:  50,
		TS:     collected,
	}
	store(t, db, "server-b", pct("/", collected, 42), both)

	got, err := db.Series(context.Background(), Selection{Metric: "disk.free_pct"})
	if err != nil {
		t.Fatalf("Series: %v", err)
	}
	// "mount=/" sorts before "mount=/,removable=false" as rendered, while their stored
	// JSON encodings compare the other way round: ',' beats '}' where they diverge.
	if len(got) != 2 || len(got[0].Labels) != 1 || len(got[1].Labels) != 2 {
		t.Fatalf("order = %+v, want the single-label series first", got)
	}
}
