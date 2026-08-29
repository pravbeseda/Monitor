package config_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pravbeseda/monitor/internal/config"
	"github.com/pravbeseda/monitor/internal/i18n"
)

const (
	telegramTokenEnv  = "MONITOR_TELEGRAM_TOKEN"
	telegramChatIDEnv = "MONITOR_TELEGRAM_CHAT_ID"
)

// spec: evaluation.md#digest — the hour and the zone are the deployment's, and the default
// names no installation.
func TestDigestDefaultsToNineUTC(t *testing.T) {
	got := load(t, minimal).Digest()
	if got.Hour != 9 || got.Minute != 0 {
		t.Fatalf("the digest goes out at %02d:%02d, want 09:00", got.Hour, got.Minute)
	}
	if got.Location.String() != "UTC" {
		t.Fatalf("the digest zone is %s, want UTC", got.Location)
	}
}

// The hub reads the zone from the file, never from the host: moving the hub must not move
// the hour a digest arrives. The zone has to be a real one, because the digest is built by
// normalising a wall-clock time through it.
func TestDigestTakesItsHourAndZoneFromTheFile(t *testing.T) {
	got := load(t, "digest: { at: \"07:30\", timezone: Europe/Berlin }\n"+minimal).Digest()
	if got.Hour != 7 || got.Minute != 30 {
		t.Fatalf("the digest goes out at %02d:%02d, want 07:30", got.Hour, got.Minute)
	}
	if _, offset := time.Date(2026, time.January, 15, 7, 30, 0, 0, got.Location).Zone(); offset != 3600 {
		t.Fatalf("the zone is %s at an offset of %ds, want Europe/Berlin in winter", got.Location, offset)
	}
}

// Each field of the digest layers on its own, so naming the hour keeps the default zone.
func TestDigestFieldsLayerApart(t *testing.T) {
	hourOnly := load(t, "digest: { at: \"07:30\" }\n"+minimal).Digest()
	if hourOnly.Hour != 7 || hourOnly.Location.String() != "UTC" {
		t.Fatalf("naming the hour changed the zone: %02d:%02d %s", hourOnly.Hour, hourOnly.Minute, hourOnly.Location)
	}
	zoneOnly := load(t, "digest: { timezone: Europe/Berlin }\n"+minimal).Digest()
	if zoneOnly.Hour != 9 || zoneOnly.Location.String() != "Europe/Berlin" {
		t.Fatalf("naming the zone changed the hour: %02d:%02d %s", zoneOnly.Hour, zoneOnly.Minute, zoneOnly.Location)
	}
}

// spec: evaluation.md#messages — the default channel needs no secret, so a hub starts and
// alerts into its log before a bot exists.
func TestNotifyDefaultsToTheLogChannel(t *testing.T) {
	t.Setenv(telegramTokenEnv, "synthetic-bot-token")
	t.Setenv(telegramChatIDEnv, "1000000")

	got := load(t, minimal).Notify()
	if got.Channel != config.ChannelLog {
		t.Fatalf("the default channel is %q, want log", got.Channel)
	}
	if got.Locale != i18n.English {
		t.Fatalf("the default locale is %q, want en", got.Locale)
	}
	if got.Telegram.Token != "" || got.Telegram.ChatID != "" {
		t.Fatal("the log channel carries Telegram credentials")
	}
}

// spec: evaluation.md#messages — the locale governs what is delivered; the secrets come
// from the environment, never from the file (ADR 0007 rule 4).
func TestTelegramChannelTakesItsSecretsFromTheEnvironment(t *testing.T) {
	t.Setenv(telegramTokenEnv, "synthetic-bot-token")
	t.Setenv(telegramChatIDEnv, "1000000")

	got := load(t, "notify: { channel: telegram, locale: ru }\n"+minimal).Notify()
	if got.Channel != config.ChannelTelegram {
		t.Fatalf("the channel is %q, want telegram", got.Channel)
	}
	if got.Locale != i18n.Russian {
		t.Fatalf("the locale is %q, want ru", got.Locale)
	}
	if got.Telegram.Token != "synthetic-bot-token" || got.Telegram.ChatID != "1000000" {
		t.Fatal("the Telegram credentials did not reach the notifier")
	}
}

// A channel says nothing about a language: telegram still speaks English until the file
// says otherwise.
func TestTelegramKeepsTheDefaultLocale(t *testing.T) {
	t.Setenv(telegramTokenEnv, "synthetic-bot-token")
	t.Setenv(telegramChatIDEnv, "1000000")

	if got := load(t, "notify: { channel: telegram }\n"+minimal).Notify(); got.Locale != i18n.English {
		t.Fatalf("the locale is %q, want en", got.Locale)
	}
}

// ADR 0007 rule 4: printing the configuration must not publish a secret, whatever verb the
// caller reaches for.
func TestPrintingTheConfigurationHidesEverySecret(t *testing.T) {
	t.Setenv(telegramTokenEnv, "synthetic-bot-token")
	t.Setenv(telegramChatIDEnv, "1000000")

	cfg := load(t, "notify: { channel: telegram }\n"+minimal)
	printed := fmt.Sprintf("%v %+v %v %+v", cfg, cfg, cfg.Notify(), node(t, cfg, "laptop-a"))
	for _, secret := range []string{"synthetic-bot-token", token} {
		if strings.Contains(printed, secret) {
			t.Fatalf("a secret reached a printed configuration: %s", printed)
		}
	}
}

// spec: hub-config.md#configuration-version — the digest and the notifier are the hub's
// own business, so an agent is never made to re-fetch by them.
func TestNotifySettingsDoNotChangeTheConfigurationVersion(t *testing.T) {
	t.Setenv(telegramTokenEnv, "synthetic-bot-token")
	t.Setenv(telegramChatIDEnv, "1000000")

	plain := node(t, load(t, minimal), "laptop-a").Version
	loud := node(t, load(t, "digest: { at: \"23:00\", timezone: Europe/Berlin }\nnotify: { channel: telegram, locale: ru }\n"+minimal), "laptop-a").Version
	if plain != loud {
		t.Fatalf("the version changed with a hub-only value: %s vs %s", plain, loud)
	}
}

// spec: evaluation.md#startup-validation — every deployment setting is refused rather than
// guessed, and an error about a secret names its variable, never its value.
func TestDigestAndNotifyReject(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		env    map[string]string
		want   string
		unwant string
	}{
		{
			name: "an hour that is not HH:MM",
			body: "digest: { at: \"9am\" }\n",
			want: "digest.at",
		},
		{
			name: "an hour outside the clock",
			body: "digest: { at: \"24:00\" }\n",
			want: "digest.at",
		},
		{
			name: "an hour carrying seconds",
			body: "digest: { at: \"07:30:00\" }\n",
			want: "digest.at",
		},
		{
			name: "an hour on a twelve-hour clock",
			body: "digest: { at: \"9:00 PM\" }\n",
			want: "digest.at",
		},
		{
			name: "the host's own zone, which moves with the machine",
			body: "digest: { timezone: Local }\n",
			want: "digest.timezone",
		},
		{
			name: "a zone no database carries",
			body: "digest: { timezone: Mars/Olympus }\n",
			want: "digest.timezone",
		},
		{
			name: "a channel the hub cannot deliver on",
			body: "notify: { channel: smoke }\n",
			want: "notify.channel",
		},
		{
			name: "a locale the interface does not speak",
			body: "notify: { locale: fr }\n",
			want: "notify.locale",
		},
		{
			name: "a regional tag, which is a browser's business and not a file's",
			body: "notify: { locale: ru-BY }\n",
			want: "notify.locale",
		},
		{
			name: "telegram with no token",
			body: "notify: { channel: telegram }\n",
			env:  map[string]string{telegramChatIDEnv: "1000000"},
			want: telegramTokenEnv,
		},
		{
			name:   "telegram with no chat id",
			body:   "notify: { channel: telegram }\n",
			env:    map[string]string{telegramTokenEnv: "synthetic-bot-token"},
			want:   telegramChatIDEnv,
			unwant: "synthetic-bot-token",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tokenEnv, token)
			// The machine running the tests may be the one holding the real secrets, so
			// every case starts from an environment that carries none.
			t.Setenv(telegramTokenEnv, "")
			t.Setenv(telegramChatIDEnv, "")
			for name, value := range tc.env {
				t.Setenv(name, value)
			}
			_, err := config.Load(write(t, tc.body+minimal))
			if err == nil {
				t.Fatal("the file was accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("the error does not name %q: %v", tc.want, err)
			}
			if tc.unwant != "" && strings.Contains(err.Error(), tc.unwant) {
				t.Fatalf("the error carries a secret: %v", err)
			}
		})
	}
}
