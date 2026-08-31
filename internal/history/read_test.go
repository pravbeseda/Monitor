package history_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pravbeseda/monitor/internal/history"
	"github.com/pravbeseda/monitor/internal/storage"
)

var now = time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)

// source answers with whatever a test put in it, and records the read it was asked for.
type source struct {
	series []storage.SeriesPoints
	err    error
	from   time.Time
	sel    storage.Selection
}

func (s *source) Series(_ context.Context, sel storage.Selection) ([]storage.SeriesRef, error) {
	s.sel = sel
	refs := make([]storage.SeriesRef, 0, len(s.series))
	for _, series := range s.series {
		refs = append(refs, series.SeriesRef)
	}
	return refs, s.err
}

func (s *source) Points(_ context.Context, sel storage.Selection, from time.Time) ([]storage.SeriesPoints, error) {
	s.from, s.sel = from, sel
	out := make([]storage.SeriesPoints, 0, len(s.series))
	for _, series := range s.series {
		if sel.Node != "" && series.Node != sel.Node {
			continue
		}
		kept := storage.SeriesPoints{SeriesRef: series.SeriesRef}
		for _, point := range series.Points {
			if !point.TS.Before(from) {
				kept.Points = append(kept.Points, point)
			}
		}
		out = append(out, kept)
	}
	return out, s.err
}

func series(node, mount string, points ...storage.Point) storage.SeriesPoints {
	return storage.SeriesPoints{
		SeriesRef: storage.SeriesRef{
			Node:   node,
			Metric: "disk.free_pct",
			Labels: map[string]string{"mount": mount, "fs": "ext4"},
		},
		Points: points,
	}
}

func at(offset time.Duration, value float64) storage.Point {
	return storage.Point{TS: now.Add(offset), Value: value}
}

func reader(src *source) history.Reader {
	return history.Reader{
		Source:   src,
		Interval: func(string, string) time.Duration { return 15 * time.Minute },
		Now:      func() time.Time { return now },
	}
}

func read(t *testing.T, src *source, q history.Query) history.Result {
	t.Helper()
	got, err := reader(src).Read(context.Background(), q)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	return got
}

func query() history.Query {
	return history.Query{Metric: "disk.free_pct", Window: 24 * time.Hour}
}

// spec: history.md#selection — a label filter keeps only the series that carry it.
func TestReadFiltersByLabel(t *testing.T) {
	src := &source{series: []storage.SeriesPoints{
		series("server-b", "/", at(-time.Hour, 42)),
		series("server-b", "/data", at(-time.Hour, 50)),
	}}

	q := query()
	q.Labels = map[string]string{"mount": "/"}
	got := read(t, src, q)

	if len(got.Series) != 1 || got.Series[0].Labels["mount"] != "/" {
		t.Fatalf("series = %+v, want the / mount alone", got.Series)
	}
}

// spec: history.md#selection — a filter naming a label no series carries matches nothing.
func TestReadFilterThatMatchesNothing(t *testing.T) {
	src := &source{series: []storage.SeriesPoints{series("server-b", "/", at(-time.Hour, 42))}}

	q := query()
	q.Labels = map[string]string{"role": "backup"}
	if got := read(t, src, q); len(got.Series) != 0 {
		t.Fatalf("series = %+v, want none", got.Series)
	}
}

// spec: history.md#selection — a series whose every point is outside the window is not returned.
func TestReadDropsSeriesWithoutPointsInTheWindow(t *testing.T) {
	src := &source{series: []storage.SeriesPoints{
		series("server-b", "/", at(-48*time.Hour, 42)),
		series("server-b", "/data", at(-time.Hour, 50)),
	}}

	if got := read(t, src, query()); len(got.Series) != 1 || got.Series[0].Labels["mount"] != "/data" {
		t.Fatalf("series = %+v, want /data alone", got.Series)
	}
}

// spec: history.md#window — no window: the 24 hours ending now, bounds included.
func TestReadWindowEndsNow(t *testing.T) {
	src := &source{series: []storage.SeriesPoints{
		series("server-b", "/", at(-24*time.Hour, 40), at(-24*time.Hour-time.Millisecond, 39), at(0, 42)),
	}}

	got := read(t, src, query())
	if !got.Window.To.Equal(now) || !got.Window.From.Equal(now.Add(-24*time.Hour)) {
		t.Fatalf("window = %+v, want the 24 hours ending %v", got.Window, now)
	}
	if points := got.Series[0].Points; len(points) != 2 || points[0].Value != 40 || points[1].Value != 42 {
		t.Fatalf("points = %+v, want the two inside the window", points)
	}
}

// spec: history.md#window — a point stamped after now ends the window instead.
func TestReadWindowEndsAtAPointFromTheFuture(t *testing.T) {
	ahead := 10 * time.Minute
	src := &source{series: []storage.SeriesPoints{series("server-b", "/", at(-time.Hour, 42), at(ahead, 41))}}

	got := read(t, src, query())
	if !got.Window.To.Equal(now.Add(ahead)) {
		t.Fatalf("window ends at %v, want the future point at %v", got.Window.To, now.Add(ahead))
	}
	last := got.Series[0].Points[len(got.Series[0].Points)-1]
	if last.Value != 41 {
		t.Fatalf("last point = %+v, want the future one", last)
	}
}

// spec: history.md#reduction — 1000 points or fewer come back as stored.
func TestReadKeepsSmallSeriesRaw(t *testing.T) {
	var points []storage.Point
	for i := range 1000 {
		points = append(points, at(-24*time.Hour+time.Duration(i)*time.Minute, float64(i)))
	}
	src := &source{series: []storage.SeriesPoints{series("server-b", "/", points...)}}

	got := read(t, src, query()).Series[0]
	if got.Reduced || len(got.Points) != 1000 || got.Stored != 1000 {
		t.Fatalf("series reduced=%v points=%d stored=%d, want 1000 raw points", got.Reduced, len(got.Points), got.Stored)
	}
}

// spec: history.md#reduction — over 1000 points: each bucket contributes its lowest and its
// highest point, and the newest point of the window survives whatever bucketing does.
func TestReadReducesToBothExtremes(t *testing.T) {
	var points []storage.Point
	step := 24 * time.Hour / 2000
	for i := range 2000 {
		value := float64(i % 10)
		points = append(points, at(-24*time.Hour+time.Duration(i)*step, value))
	}
	src := &source{series: []storage.SeriesPoints{series("server-b", "/", points...)}}

	got := read(t, src, query()).Series[0]
	if !got.Reduced || got.Stored != 2000 {
		t.Fatalf("series reduced=%v stored=%d, want a reduced series of 2000 stored points", got.Reduced, got.Stored)
	}
	if len(got.Points) > 1000 {
		t.Fatalf("points = %d, want at most 1000", len(got.Points))
	}
	var low, high bool
	for _, point := range got.Points {
		low = low || point.Value == 0
		high = high || point.Value == 9
	}
	if !low || !high {
		t.Fatalf("points hold low=%v high=%v, want both extremes of every bucket", low, high)
	}
	last := got.Points[len(got.Points)-1]
	if !last.TS.Equal(points[len(points)-1].TS) {
		t.Fatalf("last point at %v, want the newest stored point at %v", last.TS, points[len(points)-1].TS)
	}
	for i := 1; i < len(got.Points); i++ {
		if !got.Points[i].TS.After(got.Points[i-1].TS) {
			t.Fatalf("points %d and %d are out of order or share a timestamp", i-1, i)
		}
	}
}

// spec: history.md#selection — a selection matching more than 50 series is refused.
func TestReadRefusesTooManySeries(t *testing.T) {
	src := &source{}
	for i := range 51 {
		src.series = append(src.series, series("server-b", string(rune('a'+i%26))+string(rune('a'+i/26)), at(-time.Hour, 42)))
	}

	_, err := reader(src).Read(context.Background(), query())
	if refusal := new(history.Refusal); !errors.As(err, refusal) {
		t.Fatalf("Read of 51 series = %v, want a refusal", err)
	}
}

// spec: history.md — a series carries the unit its metric id declares and the interval its
// node resolves; a metric no rule declares has neither.
func TestReadCarriesUnitAndInterval(t *testing.T) {
	src := &source{series: []storage.SeriesPoints{series("server-b", "/", at(-time.Hour, 42))}}

	got := read(t, src, query()).Series[0]
	if got.Unit != history.Percent || got.Interval != 15*time.Minute {
		t.Fatalf("series unit=%q interval=%v, want percent and 15m", got.Unit, got.Interval)
	}
}

// spec: history.md#selection — the metric and the node reach storage; the labels do not,
// because no index can serve them.
func TestReadSelectsByMetricAndNode(t *testing.T) {
	src := &source{series: []storage.SeriesPoints{series("server-b", "/", at(-time.Hour, 42))}}

	q := query()
	q.Node = "server-b"
	q.Labels = map[string]string{"mount": "/"}
	read(t, src, q)

	if src.sel != (storage.Selection{Metric: "disk.free_pct", Node: "server-b"}) {
		t.Fatalf("storage was asked for %+v, want the metric and the node", src.sel)
	}
}

// spec: history.md#selection — two filters together keep only what matches both.
func TestReadFiltersByEveryLabelGiven(t *testing.T) {
	other := series("server-b", "/")
	other.Labels = map[string]string{"mount": "/", "fs": "xfs"}
	other.Points = []storage.Point{at(-time.Hour, 50)}
	src := &source{series: []storage.SeriesPoints{series("server-b", "/", at(-time.Hour, 42)), other}}

	q := query()
	q.Labels = map[string]string{"mount": "/", "fs": "ext4"}
	got := read(t, src, q)

	if len(got.Series) != 1 || got.Series[0].Labels["fs"] != "ext4" {
		t.Fatalf("series = %+v, want the ext4 volume alone", got.Series)
	}
}

// spec: history.md#window — a point further ahead than the window is long does not move it.
func TestReadIgnoresATimestampBeyondTheWindow(t *testing.T) {
	src := &source{series: []storage.SeriesPoints{
		series("laptop-a", "/", at(48*time.Hour, 1)),
		series("server-b", "/", at(-time.Hour, 42)),
	}}

	got := read(t, src, query())
	if !got.Window.To.Equal(now) {
		t.Fatalf("window ends at %v, want it to stay at %v", got.Window.To, now)
	}
	if len(got.Series) != 1 || got.Series[0].Node != "server-b" {
		t.Fatalf("series = %+v, want the skewed node not to evict the other", got.Series)
	}
}

// spec: history.md#reduction — a bucket with no points contributes nothing.
func TestReadInventsNothingForAnEmptyBucket(t *testing.T) {
	var points []storage.Point
	for i := range 1001 {
		points = append(points, at(-time.Hour+time.Duration(i)*time.Millisecond, float64(i%7)))
	}
	src := &source{series: []storage.SeriesPoints{series("server-b", "/", points...)}}

	got := read(t, src, query()).Series[0]
	if !got.Reduced {
		t.Fatalf("series was not reduced, want it reduced above %d points", 1000)
	}
	// Every point falls inside one bucket of the 500 the window is cut into; the rest
	// hold nothing and must contribute nothing.
	if len(got.Points) > 3*2+1 {
		t.Fatalf("points = %d, want only the occupied buckets to contribute", len(got.Points))
	}
	stored := map[time.Time]float64{}
	for _, point := range points {
		stored[point.TS] = point.Value
	}
	for _, point := range got.Points {
		if value, kept := stored[point.TS]; !kept || value != point.Value {
			t.Fatalf("point %+v was invented: no measurement carries it", point)
		}
	}
}

// spec: history.md#reduction — both extremes of every bucket, ties to the earliest, and at
// most 1001 points.
func TestReadReducesEachBucketToItsExtremes(t *testing.T) {
	// Four points per bucket over a four-minute window cut into 500 buckets would leave
	// most of them empty, so the window is walked point by point instead: 2001 points
	// spread evenly means four or five per bucket.
	var points []storage.Point
	step := 24 * time.Hour / 2001
	for i := range 2001 {
		points = append(points, at(-24*time.Hour+time.Duration(i)*step, float64(i%5)))
	}
	src := &source{series: []storage.SeriesPoints{series("server-b", "/", points...)}}

	got := read(t, src, query()).Series[0]
	if len(got.Points) > 1001 {
		t.Fatalf("points = %d, want at most 1001", len(got.Points))
	}
	if got.Stored != 2001 {
		t.Fatalf("stored = %d, want every point of the window counted", got.Stored)
	}
	// Every kept point is a stored one, and the lowest of a bucket is the earliest of its
	// ties: a later point of the same value never appears without its earlier twin.
	seen := map[time.Time]bool{}
	for _, point := range got.Points {
		if seen[point.TS] {
			t.Fatalf("point at %v appears twice", point.TS)
		}
		seen[point.TS] = true
	}
}

// spec: history.md#gaps — a metric no rule declares has no interval, so nothing breaks its
// line; its unit still comes from its id.
func TestReadLeavesAnUndeclaredMetricWithoutAnInterval(t *testing.T) {
	src := &source{series: []storage.SeriesPoints{series("server-b", "/", at(-20*time.Hour, 1), at(-time.Hour, 2))}}
	silent := reader(src)
	silent.Interval = func(string, string) time.Duration { return 0 }

	got, err := silent.Read(context.Background(), query())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	series := got.Series[0]
	if series.Interval != 0 || series.Unit != history.Percent {
		t.Fatalf("series interval=%v unit=%q, want no interval and the unit of its id", series.Interval, series.Unit)
	}
	if history.Gap(series.Interval, series.Points[0].TS, series.Points[1].TS) {
		t.Error("a series with no interval was broken by a gap")
	}
}

// spec: history.md#selection — the listing narrows by label and is bounded the same way.
func TestListFiltersAndIsBounded(t *testing.T) {
	src := &source{series: []storage.SeriesPoints{
		series("server-b", "/", at(-time.Hour, 42)),
		series("server-b", "/data", at(-time.Hour, 50)),
	}}

	q := query()
	q.Labels = map[string]string{"mount": "/data"}
	listed, err := reader(src).List(context.Background(), q)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 || listed[0].Labels["mount"] != "/data" {
		t.Fatalf("listing = %+v, want the /data series alone", listed)
	}

	crowded := &source{}
	for i := range 51 {
		crowded.series = append(crowded.series, series("server-b", string(rune('a'+i%26))+string(rune('a'+i/26))))
	}
	if _, err := reader(crowded).List(context.Background(), query()); err == nil {
		t.Fatal("List of 51 series was answered, want a refusal")
	}
}

// spec: history.md#reduction — a bucket contributes its lowest and its highest point, ties
// resolve to the earliest timestamp, and the newest point survives whatever bucketing does.
func TestReadKeepsBothExtremesAndTheNewestPoint(t *testing.T) {
	// 1001 points a millisecond apart all land in one bucket, so the answer is exactly
	// that bucket's two extremes plus the newest point, and can be written down.
	points := make([]storage.Point, 1001)
	for i := range points {
		points[i] = at(-24*time.Hour+time.Duration(i)*time.Millisecond, 5)
	}
	points[10].Value = 1  // the lowest
	points[30].Value = 1  // the same value later: the earliest of the ties wins
	points[20].Value = 9  // the highest
	points[900].Value = 9 // likewise for the highest
	src := &source{series: []storage.SeriesPoints{series("server-b", "/", points...)}}

	got := read(t, src, query()).Series[0]

	want := []storage.Point{points[10], points[20], points[1000]}
	if len(got.Points) != len(want) {
		t.Fatalf("points = %+v, want the two extremes and the newest point", got.Points)
	}
	for i, point := range want {
		if got.Points[i] != point {
			t.Fatalf("point %d = %+v, want %+v", i, got.Points[i], point)
		}
	}
}

// spec: history.md#reduction — a bucket whose points are all equal contributes once.
func TestReadKeepsOnePointForABucketWithoutSpread(t *testing.T) {
	points := make([]storage.Point, 1001)
	for i := range points {
		points[i] = at(-24*time.Hour+time.Duration(i)*time.Millisecond, 5)
	}
	src := &source{series: []storage.SeriesPoints{series("server-b", "/", points...)}}

	got := read(t, src, query()).Series[0]

	if len(got.Points) != 2 || got.Points[0] != points[0] || got.Points[1] != points[1000] {
		t.Fatalf("points = %+v, want the bucket's single value and the newest point", got.Points)
	}
}

// spec: history.md#window — the clamp holds exactly at one window ahead.
func TestReadWindowMovesToAPointExactlyOneWindowAhead(t *testing.T) {
	src := &source{series: []storage.SeriesPoints{series("server-b", "/", at(24*time.Hour, 42))}}

	if got := read(t, src, query()); !got.Window.To.Equal(now.Add(24 * time.Hour)) {
		t.Fatalf("window ends at %v, want it moved to the point at %v", got.Window.To, now.Add(24*time.Hour))
	}

	beyond := &source{series: []storage.SeriesPoints{series("server-b", "/", at(24*time.Hour+time.Millisecond, 42))}}
	if got := read(t, beyond, query()); !got.Window.To.Equal(now) {
		t.Fatalf("window ends at %v, want it to stay at %v", got.Window.To, now)
	}
}

// spec: history.md#gaps — the hole opens at three intervals, not before.
func TestGapOpensAtThreeIntervals(t *testing.T) {
	interval := 15 * time.Minute
	if history.Gap(interval, now, now.Add(3*interval)) {
		t.Error("a distance of exactly three intervals broke the line")
	}
	if !history.Gap(interval, now, now.Add(3*interval+time.Millisecond)) {
		t.Error("a distance past three intervals did not break the line")
	}
}

// spec: history.md#window — when a future point carries the window forward, what falls off
// the back of it is no longer inside.
func TestReadDropsWhatTheShiftedWindowLeavesBehind(t *testing.T) {
	src := &source{series: []storage.SeriesPoints{series("server-b", "/",
		at(-24*time.Hour+time.Minute, 90),
		at(-time.Hour, 42),
		at(10*time.Minute, 41),
	)}}

	got := read(t, src, query()).Series[0]

	if len(got.Points) != 2 || got.Points[0].Value != 42 {
		t.Fatalf("points = %+v, want the oldest one outside the window it moved to", got.Points)
	}
}
