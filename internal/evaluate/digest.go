package evaluate

import (
	"context"
	"log/slog"
	"time"

	"github.com/pravbeseda/monitor/internal/storage"
)

// Schedule is when the daily digest goes out. The zone comes from the file rather than
// from the host, so moving the hub to another machine cannot move the hour it arrives.
type Schedule struct {
	Hour, Minute int
	Location     *time.Location
}

// mostRecent is the latest occurrence of the configured hour at or before now. An hour a
// DST change removes is normalised forward by time.Date, so a day is never skipped.
func (s Schedule) mostRecent(now time.Time) time.Time {
	location := s.Location
	if location == nil {
		location = time.UTC
	}
	local := now.In(location)
	at := func(day time.Time) time.Time {
		return time.Date(day.Year(), day.Month(), day.Day(), s.Hour, s.Minute, 0, 0, location)
	}
	if occurrence := at(local); !occurrence.After(local) {
		return occurrence
	}
	return at(local.AddDate(0, 0, -1))
}

// digest sends the day's summary when the tick crosses the configured hour. The window is
// closed even when there was nothing to say, so a warning that appears after the hour
// waits for tomorrow rather than going out at once.
func (e *Evaluator) digest(ctx context.Context, subjects []Subject, now time.Time) error {
	since, digested, err := e.store.LastDigestAt(ctx)
	if err != nil {
		return err
	}
	if !digested {
		// A database that has never digested starts at this hub's first run, so history
		// is never replayed.
		since = e.started
	}
	// The occurrence decides whether a digest is due; the tick time records what has been
	// reported, which is why it is the mark below.
	occurrence := e.schedule.mostRecent(now)
	if !occurrence.After(since) {
		return nil
	}

	entries, err := e.entries(ctx, subjects, since, now, occurrence)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		if err := e.send(ctx, func(ctx context.Context) error {
			return e.notifier.Digest(ctx, occurrence, entries)
		}); err != nil {
			// The window stays open, so the next tick sends the same one again.
			slog.Error("deliver the digest", "at", occurrence, "error", err)
			return nil
		}
	}
	// The window ends where it was read, not at the hour it speaks for: stamping the
	// occurrence would leave everything recorded since inside tomorrow's window too.
	return e.store.SetLastDigestAt(ctx, now)
}

// entries is what the digest lists: every transition of the window that was not delivered
// at once, and every subject standing in warning, one line per subject, in the order
// subjects come out in. A frozen subject is neither, because its values are stale.
func (e *Evaluator) entries(ctx context.Context, subjects []Subject, since, now, occurrence time.Time) ([]Message, error) {
	events, err := e.store.EventsBetween(ctx, since, now)
	if err != nil {
		return nil, err
	}
	// The window is read newest-last, so a subject ends up with the last thing that
	// happened to it and is listed once.
	newest := make(map[string]storage.Transition, len(events))
	for _, event := range events {
		if key, err := event.Key(); err == nil {
			newest[key] = event
		}
	}

	var out []Message
	for _, subject := range subjects {
		if subject.Frozen {
			continue
		}
		key, err := subject.Key()
		if err != nil {
			continue
		}
		// Everything critical touches was delivered at once (ADR 0016). A subject whose
		// last move was such a change has nothing left to report here — but it is still
		// listed below if it is standing in warning now.
		if event, changed := newest[key]; changed && !instant(event) {
			from := storedLevel(event.From, subject.Node, subject.Rule)
			to := storedLevel(event.To, subject.Node, subject.Rule)
			out = append(out, message(subject, from, to, event.Readings, event.FromSince, event.At))
			continue
		}
		if subject.Level == Warning {
			out = append(out, message(subject, Warning, Warning, subject.Readings, subject.Since, occurrence))
		}
	}
	return out, nil
}
