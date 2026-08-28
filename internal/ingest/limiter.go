package ingest

import (
	"sync"
	"time"
)

// limiter counts requests per node in fixed one-minute windows. A node that floods the
// hub is refused before its body is read.
type limiter struct {
	mu        sync.Mutex
	perMinute int
	windows   map[string]window
}

type window struct {
	start time.Time
	count int
}

func newLimiter(perMinute int) *limiter {
	return &limiter{perMinute: perMinute, windows: map[string]window{}}
}

func (l *limiter) allow(node string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	current := l.windows[node]
	if now.Sub(current.start) >= time.Minute {
		current = window{start: now}
	}
	if current.count >= l.perMinute {
		l.windows[node] = current
		return false
	}
	current.count++
	l.windows[node] = current
	return true
}
