package api_test

import (
	"testing"
	"time"

	"github.com/pravbeseda/Monitor/internal/api"
)

func TestFormatDuration(t *testing.T) {
	tests := map[time.Duration]string{
		5 * time.Minute:  "5m",
		time.Hour:        "1h",
		90 * time.Minute: "1h30m",
		30 * time.Second: "30s",
	}

	for d, want := range tests {
		if got := api.FormatDuration(d); got != want {
			t.Errorf("FormatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}
