package evaluate

import (
	"context"
	"testing"
	"time"
)

// stuck is a channel that never answers on its own.
type stuck struct{ released chan struct{} }

func (s stuck) Notify(context.Context, Message) error {
	<-s.released
	return nil
}

func (s stuck) Digest(context.Context, time.Time, []Message) error { return nil }

// spec: evaluation.md#the-tick — a notifier that does not return cannot hold evaluation
// open: the send is abandoned and counted as a failure.
func TestASendThatNeverAnswersIsAbandoned(t *testing.T) {
	was := sendTimeout
	sendTimeout = 10 * time.Millisecond
	channel := stuck{released: make(chan struct{})}
	t.Cleanup(func() {
		sendTimeout = was
		close(channel.released)
	})

	evaluator := New(nil, channel, nil, time.Now)
	if err := evaluator.send(context.Background(), Message{}); err == nil {
		t.Fatal("a channel that never answered was counted as a delivery")
	}
}
