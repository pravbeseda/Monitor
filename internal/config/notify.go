package config

import (
	"fmt"
	"time"

	"github.com/pravbeseda/monitor/internal/i18n"
)

// The channels a notification can go out on. The log channel needs no secret, so a hub
// alerts into its journal from the first start, before a bot exists.
const (
	ChannelLog      = "log"
	ChannelTelegram = "telegram"
)

// The Telegram credentials are deployment settings and live in the environment, never in
// the file (ADR 0007 rule 4).
const (
	telegramTokenEnv  = "MONITOR_TELEGRAM_TOKEN"
	telegramChatIDEnv = "MONITOR_TELEGRAM_CHAT_ID"
)

// Digest is when the daily digest goes out. The zone comes from the file rather than from
// the host, so moving the hub to another machine cannot move the hour it arrives.
type Digest struct {
	Hour, Minute int
	Location     *time.Location
}

// Telegram is what the bot needs to send. It is empty unless the channel is telegram.
type Telegram struct {
	Token, ChatID string
}

// Notify is how notifications leave the hub.
type Notify struct {
	Channel  string
	Locale   i18n.Locale
	Telegram Telegram
}

func resolveDigest(f fileDigest) (Digest, error) {
	at := last(defaultDigestAt, f.At)
	parsed, err := time.Parse("15:04", at)
	if err != nil {
		return Digest{}, fmt.Errorf("digest.at: %q is not an hour of the day, which is written as HH:MM", at)
	}

	zone := last(defaultDigestZone, f.Timezone)
	if zone == "Local" {
		return Digest{}, fmt.Errorf("digest.timezone: %q follows the host, so the digest hour would move with the machine; name a zone such as UTC or Europe/Berlin", zone)
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return Digest{}, fmt.Errorf("digest.timezone: %q is not a zone this system carries, which is an IANA name such as Europe/Berlin", zone)
	}
	return Digest{Hour: parsed.Hour(), Minute: parsed.Minute(), Location: location}, nil
}

func resolveNotify(f fileNotify) (Notify, error) {
	channel := last(defaultChannel, f.Channel)
	if channel != ChannelLog && channel != ChannelTelegram {
		return Notify{}, fmt.Errorf("notify.channel: %q is unknown; the channels are %s and %s", channel, ChannelLog, ChannelTelegram)
	}

	tag := last(string(defaultLocale), f.Locale)
	locale, spoken := i18n.Parse(tag)
	if !spoken {
		return Notify{}, fmt.Errorf("notify.locale: %q is not a language the interface speaks (%s, %s)", tag, i18n.English, i18n.Russian)
	}

	out := Notify{Channel: channel, Locale: locale}
	if channel != ChannelTelegram {
		return out, nil
	}
	token, err := envValue(telegramTokenEnv, "with notify.channel: "+ChannelTelegram+" it has no default")
	if err != nil {
		return Notify{}, err
	}
	chatID, err := envValue(telegramChatIDEnv, "with notify.channel: "+ChannelTelegram+" it has no default")
	if err != nil {
		return Notify{}, err
	}
	out.Telegram = Telegram{Token: token, ChatID: chatID}
	return out, nil
}

// String keeps the bot's credentials out of a debug print. A configuration is one careless
// %v away from publishing them, and this repository is public (ADR 0007 rule 4).
func (t Telegram) String() string {
	if t.Token == "" {
		return "telegram: no credentials"
	}
	return "telegram: credentials from the environment"
}
