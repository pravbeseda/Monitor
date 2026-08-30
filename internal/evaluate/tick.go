package evaluate

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/pravbeseda/monitor/internal/storage"
)

// Evaluator runs the hub's evaluation pass. It is the only writer of the event log
// (ADR 0015): silence, levels, the daily repeat and the digest are one pass on one clock,
// never a step inside an ingest request.
type Evaluator struct {
	store Store
	now   func() time.Time
	// targets is the configuration as evaluation reads it, resolved once at startup: the
	// file is never re-read while the hub runs.
	targets []Target
	// running keeps two passes from overlapping. A tick that would start while the
	// previous one is still going gives way rather than queueing behind it.
	running atomic.Bool
}

// New builds the evaluator the hub ticks.
func New(store Store, targets []Target, now func() time.Time) *Evaluator {
	return &Evaluator{store: store, now: now, targets: targets}
}

// Tick runs one pass against one consistent view of the data, taken at the instant it
// evaluates for: a measurement that arrives while a tick is running belongs to the next
// tick, never to half of this one.
func (e *Evaluator) Tick(ctx context.Context) error {
	if !e.running.CompareAndSwap(false, true) {
		slog.Warn("evaluation tick skipped: the previous pass is still running")
		return nil
	}
	defer e.running.Store(false)

	snapshot, err := e.store.Snapshot(ctx)
	if err != nil {
		return err
	}
	now := e.now()
	for _, subject := range Subjects(e.targets, snapshot, now) {
		// A hub asked to stop evaluates no further subject; what it already recorded
		// stays recorded, and the next start picks the rest up.
		if err := ctx.Err(); err != nil {
			return err
		}
		if subject.Frozen {
			continue
		}
		if err := e.record(ctx, subject, now); err != nil {
			return err
		}
	}
	return nil
}

// record writes what the pass made of one subject. A change is a level and an event
// together; a subject that has not moved is only ever inserted, so a first evaluation at
// ok appears without an event and `since` survives every tick that follows.
func (e *Evaluator) record(ctx context.Context, subject Subject, now time.Time) error {
	if !subject.Changed() {
		return e.store.SaveState(ctx, storage.State{
			Subject: subject.Subject,
			Level:   subject.Level.String(),
			Since:   subject.Since,
		})
	}
	return e.store.ApplyTransition(ctx, storage.Transition{
		Subject:   subject.Subject,
		At:        now,
		From:      subject.Previous.String(),
		To:        subject.Level.String(),
		FromSince: subject.Since,
		Readings:  subject.Readings,
	})
}
