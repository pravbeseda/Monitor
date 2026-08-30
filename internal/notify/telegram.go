package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pravbeseda/monitor/internal/evaluate"
	"github.com/pravbeseda/monitor/internal/i18n"
)

// telegramAPI is the Bot API root. The token goes in the path, which is why no error and
// no log line here ever prints a URL (ADR 0007 rule 4).
const telegramAPI = "https://api.telegram.org"

// telegramTimeout bounds one send. The evaluator abandons a channel that outlasts its own
// deadline anyway; this is what keeps a socket from being held until then.
const telegramTimeout = 15 * time.Second

// Telegram sends one message per notification to the configured chat. Stage 2 only sends:
// long polling and inbound commands are worth building when there is history to show.
type Telegram struct {
	Token, ChatID string
	Locale        i18n.Locale
	// Client defaults to one bounded by telegramTimeout.
	Client *http.Client
	// API is the Bot API root, and defaults to Telegram's own.
	API string
}

var _ evaluate.Notifier = Telegram{}

// Notify sends one message to the configured chat.
func (t Telegram) Notify(ctx context.Context, m evaluate.Message) error {
	return t.send(ctx, Render(i18n.For(t.Locale), m))
}

// Digest sends the day's summary as one message, not one per subject.
func (t Telegram) Digest(ctx context.Context, _ time.Time, entries []evaluate.Message) error {
	return t.send(ctx, RenderDigest(i18n.For(t.Locale), entries))
}

func (t Telegram) send(ctx context.Context, text string) error {
	body, err := json.Marshal(struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}{ChatID: t.ChatID, Text: text})
	if err != nil {
		return fmt.Errorf("encode a telegram message: %w", err)
	}

	url := t.api() + "/bot" + t.Token + "/sendMessage"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		// The URL carries the token, so it is never quoted back.
		return fmt.Errorf("build the telegram request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := t.client().Do(request)
	if err != nil {
		return fmt.Errorf("send to telegram: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram refused the message: %s", response.Status)
	}
	return nil
}

func (t Telegram) api() string {
	if t.API != "" {
		return t.API
	}
	return telegramAPI
}

func (t Telegram) client() *http.Client {
	if t.Client != nil {
		return t.Client
	}
	return &http.Client{Timeout: telegramTimeout}
}

// String keeps the bot's credentials out of a debug print of the channel.
func (t Telegram) String() string { return "telegram channel" }
