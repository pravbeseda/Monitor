package ingest

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := map[time.Duration]string{
		5 * time.Minute:  "5m",
		time.Hour:        "1h",
		90 * time.Minute: "1h30m",
		30 * time.Second: "30s",
	}

	for d, want := range tests {
		if got := formatDuration(d); got != want {
			t.Errorf("formatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}
