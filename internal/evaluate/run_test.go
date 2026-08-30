package evaluate_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pravbeseda/monitor/internal/evaluate"
	"github.com/pravbeseda/monitor/internal/storage"
)

// counting is the hub's own store, counting how often a pass asked it for a snapshot:
// that is what a loop can be observed by.
type counting struct {
	evaluate.Store
	passes atomic.Int32
}

func (c *counting) Snapshot(ctx context.Context, owed []string) (storage.Snapshot, error) {
	c.passes.Add(1)
	return c.Store.Snapshot(ctx, owed)
}

// spec: evaluation.md#the-tick — evaluation runs on its own schedule until the hub stops.
func TestRunEvaluatesUntilTheHubStops(t *testing.T) {
	store := &counting{Store: open(t)}
	e := evaluate.New(evaluate.Options{
		Store: store, Notifier: &recorder{},
		Digest: schedule, Started: time.Now(), Now: time.Now,
	})

	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		e.Run(ctx, time.Millisecond)
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for store.passes.Load() < 2 {
		select {
		case <-deadline:
			t.Fatalf("the loop ran %d passes in two seconds", store.passes.Load())
		default:
			time.Sleep(time.Millisecond)
		}
	}
	stop()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the loop outlived the hub it belongs to")
	}
}
