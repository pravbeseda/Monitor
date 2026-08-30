package notify_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pravbeseda/monitor/internal/config"
	"github.com/pravbeseda/monitor/internal/evaluate"
	"github.com/pravbeseda/monitor/internal/i18n"
	"github.com/pravbeseda/monitor/internal/notify"
)

var at = time.Date(2026, time.August, 30, 9, 0, 0, 0, time.UTC)

func entering() evaluate.Message {
	return evaluate.Message{
		Node:   "server-b",
		Rule:   "disk",
		Labels: map[string]string{"mount": "/data", "fs": "ext4", "removable": "false"},
		From:   evaluate.Warning,
		To:     evaluate.Critical,
		Readings: map[string]float64{
			"disk.free_bytes": 3e9,
			"disk.free_pct":   2.34,
		},
		Since: at.Add(-2 * time.Hour),
		At:    at,
	}
}

func capture(t *testing.T) (*slog.Logger, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	return slog.New(slog.NewTextHandler(&out, nil)), &out
}

// spec: evaluation.md#messages — the log channel writes one English line per message and
// sends nothing, so the default channel needs no secret.
func TestTheLogChannelWritesOneEnglishLine(t *testing.T) {
	logger, out := capture(t)
	channel := notify.Log{Logger: logger}
	if err := channel.Notify(context.Background(), entering()); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	line := out.String()
	if strings.Count(strings.TrimSpace(line), "\n") != 0 {
		t.Fatalf("one message wrote more than one line:\n%s", line)
	}
	for _, want := range []string{"server-b", "/data", "critical", "warning", "3.0 GB"} {
		if !strings.Contains(line, want) {
			t.Fatalf("the log line is missing %q:\n%s", want, line)
		}
	}
}

// spec: evaluation.md#messages — the log line stays English whatever the locale: logs are
// diagnostic, and the locale governs delivered channels only.
func TestTheLogChannelStaysEnglishUnderARussianLocale(t *testing.T) {
	built, err := notify.New(config.Notify{Channel: config.ChannelLog, Locale: i18n.Russian})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	channel, isLog := built.(notify.Log)
	if !isLog {
		t.Fatalf("channel log built a %T", built)
	}
	logger, out := capture(t)
	channel.Logger = logger
	if err := channel.Notify(context.Background(), entering()); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if line := out.String(); !strings.Contains(line, "critical") || strings.Contains(line, "критично") {
		t.Fatalf("the log line followed the locale:\n%s", line)
	}
}

// spec: evaluation.md#messages — a delivered message carries its text, byte sizes and times
// from the catalogue of the configured locale.
func TestARussianMessageComesFromTheRussianCatalogue(t *testing.T) {
	got := notify.Render(i18n.For(i18n.Russian), entering())
	for _, want := range []string{"критично", "предупреждение", "3,0 ГБ", "30.08.2026"} {
		if !strings.Contains(got, want) {
			t.Fatalf("the Russian message is missing %q: %s", want, got)
		}
	}
}

// spec: evaluation.md#messages — every message carries the node, the subject, both levels,
// the values that produced it and how long the subject had been where it was.
func TestAMessageCarriesEveryField(t *testing.T) {
	got := notify.Render(i18n.For(i18n.English), entering())
	for _, want := range []string{"server-b", "/data", "critical", "warning", "3.0 GB", "2.3%", "2026-08-30 07:00"} {
		if !strings.Contains(got, want) {
			t.Fatalf("the message is missing %q: %s", want, got)
		}
	}
}

// spec: evaluation.md#node-silence — the silence subject has no readings, so its message
// says what it is instead of pretending to values.
func TestTheSilenceMessageNamesTheNodeAlone(t *testing.T) {
	message := entering()
	message.Rule = evaluate.SilenceRule
	message.Labels = nil
	message.Readings = nil

	got := notify.Render(i18n.For(i18n.English), message)
	if !strings.Contains(got, "server-b") || strings.Contains(got, "GB") {
		t.Fatalf("the silence message reads %q", got)
	}
}

// spec: evaluation.md#notifications — a repeat says the subject has not moved rather than
// claiming a change it did not make.
func TestARepeatReadsAsAStandingLevel(t *testing.T) {
	message := entering()
	message.From = evaluate.Critical

	got := notify.Render(i18n.For(i18n.English), message)
	if !strings.Contains(got, "still critical") {
		t.Fatalf("a repeat reads %q", got)
	}
}

// spec: evaluation.md#digest — a digest is one message carrying a list of entries, not one
// message per subject.
func TestTheLogChannelWritesOneDigest(t *testing.T) {
	logger, out := capture(t)
	channel := notify.Log{Logger: logger}
	first, second := entering(), entering()
	second.Labels = map[string]string{"mount": "/srv"}

	if err := channel.Digest(context.Background(), at, []evaluate.Message{first, second}); err != nil {
		t.Fatalf("Digest: %v", err)
	}
	line := out.String()
	if strings.Count(line, "msg=") != 1 {
		t.Fatalf("a digest of two entries wrote more than one record:\n%s", line)
	}
	for _, want := range []string{"Daily digest", "/data", "/srv"} {
		if !strings.Contains(line, want) {
			t.Fatalf("the digest is missing %q:\n%s", want, line)
		}
	}
}

// spec: evaluation.md#messages — the telegram channel sends one message per notification
// to the configured chat.
func TestTheTelegramChannelSendsOneMessageToTheChat(t *testing.T) {
	var got struct {
		path, body string
		calls      int
	}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got.path, got.body, got.calls = r.URL.Path, string(body), got.calls+1
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer api.Close()

	channel := notify.Telegram{Token: "bot-secret", ChatID: "12345", Locale: i18n.English, API: api.URL}
	if err := channel.Notify(context.Background(), entering()); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if got.calls != 1 {
		t.Fatalf("one notification made %d requests", got.calls)
	}
	if !strings.Contains(got.path, "bot-secret") || !strings.Contains(got.path, "sendMessage") {
		t.Fatalf("the request went to %s", got.path)
	}
	for _, want := range []string{"12345", "server-b", "critical"} {
		if !strings.Contains(got.body, want) {
			t.Fatalf("the request body is missing %q: %s", want, got.body)
		}
	}
}

// spec: evaluation.md#messages — under a Russian locale the delivered text, its byte sizes
// and its times come from the Russian catalogue.
func TestTheTelegramChannelDeliversTheConfiguredLocale(t *testing.T) {
	var body string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer api.Close()

	channel := notify.Telegram{Token: "t", ChatID: "1", Locale: i18n.Russian, API: api.URL}
	if err := channel.Notify(context.Background(), entering()); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	for _, want := range []string{"критично", "3,0 ГБ"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the delivered text is missing %q: %s", want, body)
		}
	}
}

// spec: evaluation.md#notifications — a channel that refuses returns an error, which is
// what leaves the event undelivered and retried.
func TestTheTelegramChannelReportsARefusal(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"description":"chat not found"}`))
	}))
	defer api.Close()

	channel := notify.Telegram{Token: "t", ChatID: "1", Locale: i18n.English, API: api.URL}
	err := channel.Notify(context.Background(), entering())
	if err == nil {
		t.Fatal("a refused send was counted as a delivery")
	}
	if strings.Contains(err.Error(), "t") && strings.Contains(err.Error(), "/bot") {
		t.Fatalf("the error carries the bot token: %v", err)
	}
}
