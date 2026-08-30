package notify

import (
	"fmt"

	"github.com/pravbeseda/monitor/internal/config"
	"github.com/pravbeseda/monitor/internal/evaluate"
)

// New builds the channel the configuration names. The locale reaches the channels that
// deliver to a person; the log channel ignores it on purpose (ADR 0008).
func New(settings config.Notify) (evaluate.Notifier, error) {
	switch settings.Channel {
	case config.ChannelLog:
		return Log{}, nil
	case config.ChannelTelegram:
		return Telegram{
			Token:  settings.Telegram.Token,
			ChatID: settings.Telegram.ChatID,
			Locale: settings.Locale,
		}, nil
	default:
		return nil, fmt.Errorf("notify.channel: %q is unknown", settings.Channel)
	}
}
