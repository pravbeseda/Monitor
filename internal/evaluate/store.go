package evaluate

import (
	"context"
	"time"

	"github.com/pravbeseda/monitor/internal/storage"
)

// Store is what evaluation needs of persistence, declared where it is consumed: the hub's
// own Storage boundary keeps the three methods ingest and the web page use, so adding to
// this one costs nothing to their test doubles.
type Store interface {
	// Snapshot is the one view a tick evaluates against: nodes and their latest values,
	// the level every subject held, and each subject's newest transition.
	Snapshot(ctx context.Context) (storage.Snapshot, error)

	SaveState(ctx context.Context, state storage.State) error
	ApplyTransition(ctx context.Context, change storage.Transition) error
	RecordNotified(ctx context.Context, subject storage.Subject, at time.Time) error

	EventsBetween(ctx context.Context, from, to time.Time) ([]storage.Transition, error)
	LastDigestAt(ctx context.Context) (time.Time, bool, error)
	SetLastDigestAt(ctx context.Context, at time.Time) error
}

// The hub's storage has to satisfy it, and the compiler is what says so.
var _ Store = (*storage.SQLite)(nil)
