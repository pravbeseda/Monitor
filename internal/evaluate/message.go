package evaluate

import (
	"context"
	"time"

	"github.com/pravbeseda/monitor/internal/storage"
)

// repeatAfter is how long an unresolved critical stays quiet before it is said again
// (ADR 0006). Nothing below critical repeats at all: that is what the digest is for.
const repeatAfter = 24 * time.Hour

// Message is one notification as every channel receives it. It carries the same fields
// whatever produced it, so a channel formats rather than decides
// (docs/specs/evaluation.md#messages).
type Message struct {
	Node   string
	Rule   string
	Labels map[string]string
	// From is the level the subject left and To the level it reached. A repeat and a
	// digest entry for a subject that has not moved carry the same level in both.
	From, To Level
	// Readings are the values that produced the message, keyed by metric id. The silence
	// subject has none: its input is hub receipt time.
	Readings map[string]float64
	// Since is when the subject entered the level it is leaving, which is how long a
	// reader is told it had been there.
	Since time.Time
	// At is when the change was recorded, or the instant of a repeat.
	At time.Time
}

// Notifier is where a notification leaves the hub. What is worth sending is decided before
// a channel is called (ADR 0006, ADR 0016); a channel formats and delivers.
type Notifier interface {
	// Notify delivers one message.
	Notify(ctx context.Context, m Message) error
	// Digest delivers the day's summary. It is called only when there is something to say.
	Digest(ctx context.Context, at time.Time, entries []Message) error
}

// instant reports whether an event is one of the three ADR 0016 delivers at once: entering
// critical, or leaving it for either lower level.
func instant(change storage.Transition) bool {
	return change.To == Critical.String() || change.From == Critical.String()
}

// due decides what a subject is owed. Delivery is driven by the subject's newest event
// against last_notified_at rather than by what changed on this tick, so a send that failed
// is tried again instead of being lost.
func due(s Subject, newest storage.Transition, recorded bool, now time.Time) (Message, bool) {
	if recorded && instant(newest) && newest.At.After(s.LastNotifiedAt) {
		from, _ := ParseLevel(newest.From)
		to, _ := ParseLevel(newest.To)
		return Message{
			Node: s.Node, Rule: s.Rule, Labels: s.Labels,
			From: from, To: to, Readings: newest.Readings,
			Since: newest.FromSince, At: newest.At,
		}, true
	}
	if s.Level == Critical && (s.LastNotifiedAt.IsZero() || now.Sub(s.LastNotifiedAt) >= repeatAfter) {
		return Message{
			Node: s.Node, Rule: s.Rule, Labels: s.Labels,
			From: s.Level, To: s.Level, Readings: s.Readings,
			Since: s.Since, At: now,
		}, true
	}
	return Message{}, false
}
