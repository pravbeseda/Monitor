package history

import (
	"context"
	"time"

	"github.com/pravbeseda/monitor/internal/storage"
)

// Source is what history needs of persistence.
type Source interface {
	Series(ctx context.Context, sel storage.Selection) ([]storage.SeriesRef, error)
	Points(ctx context.Context, sel storage.Selection, from time.Time) ([]storage.SeriesPoints, error)
}

// Interval reports how often a node is expected to report a metric, and zero when the
// metric belongs to no rule.
type Interval func(node, metric string) time.Duration

// Reader answers history queries.
type Reader struct {
	Source   Source
	Interval Interval
	Now      func() time.Time
}

const (
	// maxSeries bounds one answer: a query wide enough to exceed it is refused rather
	// than truncated, so no caller is quietly told less than it asked for.
	maxSeries = 50
	// rawLimit is the number of stored points a series may hold before it is reduced.
	rawLimit = 1000
	buckets  = rawLimit / 2
)

// List answers what exists, with no window and no points.
func (r Reader) List(ctx context.Context, query Query) ([]Series, error) {
	refs, err := r.Source.Series(ctx, selection(query))
	if err != nil {
		return nil, err
	}
	out := make([]Series, 0, len(refs))
	for _, ref := range refs {
		if matches(ref.Labels, query.Labels) {
			out = append(out, r.describe(ref))
		}
	}
	if len(out) > maxSeries {
		return nil, tooMany(len(out))
	}
	return out, nil
}

// Read answers one window of history.
func (r Reader) Read(ctx context.Context, query Query) (Result, error) {
	now := r.Now()
	stored, err := r.Source.Points(ctx, selection(query), now.Add(-query.Window))
	if err != nil {
		return Result{}, err
	}

	selected := make([]storage.SeriesPoints, 0, len(stored))
	for _, series := range stored {
		if matches(series.Labels, query.Labels) {
			selected = append(selected, series)
		}
	}
	if len(selected) > maxSeries {
		return Result{}, tooMany(len(selected))
	}

	window := Window{To: now}
	for _, series := range selected {
		if len(series.Points) == 0 {
			continue
		}
		// The rows arrive oldest first, so the last point of a series is its newest. A
		// clock running ahead moves the end of the window to its value, but no further
		// ahead than the window is long: one node stamped a century out would otherwise
		// carry every other series off the chart for good.
		newest := series.Points[len(series.Points)-1].TS
		if newest.After(window.To) && newest.Sub(now) <= query.Window {
			window.To = newest
		}
	}
	window.From = window.To.Add(-query.Window)

	result := Result{Window: window, Series: make([]Series, 0, len(selected))}
	for _, series := range selected {
		points := insideWindow(series.Points, window)
		if len(points) == 0 {
			continue
		}
		out := r.describe(series.SeriesRef)
		out.Stored = len(points)
		out.Points, out.Reduced = reduce(points, window)
		result.Series = append(result.Series, out)
	}
	return result, nil
}

func (r Reader) describe(ref storage.SeriesRef) Series {
	return Series{
		Node:     ref.Node,
		Metric:   ref.Metric,
		Labels:   ref.Labels,
		Unit:     UnitOf(ref.Metric),
		Interval: r.Interval(ref.Node, ref.Metric),
	}
}

func selection(query Query) storage.Selection {
	return storage.Selection{Metric: query.Metric, Node: query.Node}
}

func tooMany(count int) error {
	return refuse("too_many_series", "this query selects %d series, more than the %d one answer carries", count, maxSeries)
}

// matches is exact equality on every named label. A filter cannot demand that a series
// carry no other label, which is why a link meant for one series names them all.
func matches(labels, filters map[string]string) bool {
	for name, want := range filters {
		if labels[name] != want {
			return false
		}
	}
	return true
}

// insideWindow keeps both bounds. The read started at `now - window` with no upper bound,
// so it may hand back points on either side of the window the answer settled on.
func insideWindow(points []storage.Point, window Window) []storage.Point {
	first := 0
	for first < len(points) && points[first].TS.Before(window.From) {
		first++
	}
	last := len(points)
	for last > first && points[last-1].TS.After(window.To) {
		last--
	}
	return points[first:last]
}

// reduce keeps both extremes of every bucket, because which of them matters is a property
// of the metric and this package judges no value (docs/specs/history.md#reduction). The
// newest point survives whatever the bucketing does, so a chart's right edge and the index
// page never disagree about the current value.
func reduce(points []storage.Point, window Window) ([]storage.Point, bool) {
	if len(points) <= rawLimit {
		return points, false
	}
	// A window shorter than the bucket count is not reachable through a query, but a
	// zero width would divide by zero rather than answer badly.
	width := max(window.To.Sub(window.From)/buckets, 1)

	lowest, highest := map[int]int{}, map[int]int{}
	for i, point := range points {
		bucket := int(point.TS.Sub(window.From) / width)
		if bucket >= buckets {
			bucket = buckets - 1 // the last bucket is closed at the end of the window.
		}
		if at, seen := lowest[bucket]; !seen || point.Value < points[at].Value {
			lowest[bucket] = i
		}
		if at, seen := highest[bucket]; !seen || point.Value > points[at].Value {
			highest[bucket] = i
		}
	}

	keep := make(map[int]bool, 2*len(lowest)+1)
	for _, at := range lowest {
		keep[at] = true
	}
	for _, at := range highest {
		keep[at] = true
	}
	keep[len(points)-1] = true

	out := make([]storage.Point, 0, len(keep))
	for i, point := range points {
		if keep[i] {
			out = append(out, point)
		}
	}
	return out, true
}
